const express = require("express");
const http = require("http");
const https = require("https");
const httpProxy = require("http-proxy");
const cookieParser = require("cookie-parser");
const geoip = require("geoip-lite");
const crypto = require("crypto");
const fs = require("fs");
const toml = require("toml");
const winston = require("winston");
const DailyRotateFile = require("winston-daily-rotate-file");
const compression = require("compression");
const Redis = require("ioredis");

const configFilePath = "config.toml";
const exampleConfigFilePath = "config.toml.example";

const redis = new Redis({
    connectTimeout: 10000,
    maxRetriesPerRequest: 3
});

const logger = winston.createLogger({
    level: "info",
    format: winston.format.combine(
        winston.format.timestamp(),
        winston.format.printf(({ timestamp, level, message }) => {
            return `[${timestamp}] [${level.toUpperCase()}] ${message}`;
        })
    ),
    transports: [
        new winston.transports.Console(),
        new winston.transports.File({ filename: "logs/error.log", level: "error", maxsize: 10 * 1024 * 1024, maxFiles: 5 }),
        new winston.transports.File({ filename: "logs/warn.log", level: "warn", maxsize: 10 * 1024 * 1024, maxFiles: 5 }),
        new winston.transports.File({ filename: "logs/combined.log", maxsize: 10 * 1024 * 1024, maxFiles: 5 })
    ],
});

logger.add(new DailyRotateFile({
    filename: "logs/application-%DATE%.log",
    datePattern: "YYYY-MM-DD",
    zippedArchive: true,
    maxSize: "20m",
    maxFiles: "14d"
}));

function initializeConfig() {
    if (!fs.existsSync(configFilePath)) {
        if (fs.existsSync(exampleConfigFilePath)) {
            fs.copyFileSync(exampleConfigFilePath, configFilePath);
            logger.info(`First time running. Please edit ${configFilePath}.`);
        } else {
            logger.error("Both 'config.toml' are missing. Please provide a valid configuration file.");
        }
        process.exit(1);
    }
}

function loadFile(filePath, defaultValue) {
    try {
        return fs.readFileSync(filePath, "utf8");
    } catch (err) {
        logger.error(`Error reading file ${filePath}:`, err);
        return defaultValue;
    }
}

function getClientIp(req) {
    return req.headers["cf-connecting-ip"] ||
           req.headers["x-real-ip"] ||
           req.headers["x-forwarded-for"]?.split(",")[0] ||
           req.socket.remoteAddress;
}

function parseConfig() {
    try {
        return toml.parse(loadFile(configFilePath, ""));
    } catch (parseError) {
        logger.error("Parse TOML failed:", parseError);
        process.exit(1);
    }
}

function getGeolocation(ip) {
    const geo = geoip.lookup(ip);
    return geo ? `${geo.country} - ${geo.city} (${geo.ll[0]}, ${geo.ll[1]})` : "Unknown location";
}

function logRequestFailure(req, err) {
    const clientIp = getClientIp(req);
    logger.warn(`IP: ${clientIp} | Location: ${getGeolocation(clientIp)} | Warning: ${err.message}`);
}

function encryptToken(data, secret_key) {
    return crypto.createHmac("sha256", secret_key).update(data).digest("hex");
}

function verifyToken(data, token, secret_key) {
    const expected = encryptToken(data, secret_key);
    const expectedBuffer = Buffer.from(expected, 'utf8');
    const tokenBuffer = Buffer.from(token, 'utf8');
    if (expectedBuffer.length !== tokenBuffer.length) {
        return false;
    }
    return crypto.timingSafeEqual(expectedBuffer, tokenBuffer);
}


async function rateLimitMiddleware(req, res, next) {
    const clientIp = getClientIp(req);
    const count = 60 || config.limit.count;
    const window = 100 || config.limit.window;
    const key = `oka_rate_limit:${clientIp}`;
    const luaScript = `
        local current
        current = redis.call("INCR", KEYS[1])
        if current == 1 then
            redis.call("EXPIRE", KEYS[1], ARGV[1])
        end
        return current
    `;
    
    const requests = await redis.eval(luaScript, 1, key, window);
    if (requests === 1) {
        await redis.expire(key, window);
    }
    if (requests > count) {
        logger.info(`[RATE LIMIT] | IP: ${clientIp} | Location: ${getGeolocation(clientIp)} | Request: ${req.method} ${req.url}`);
        res.status(429).json({ message: "Too many requests, please try again later." });
        return;
    }
    next();
}


const verificationPage = loadFile("public/verification.html", "<h1>Verification</h1><script>setTimeout(() => window.location.reload(), 5000);</script>");
const errorPage = loadFile("public/502.html", "<h1>502 Bad Gateway</h1>");

function createProxyServer(proxyConfig) {
    const app = express();
    const proxy = httpProxy.createProxyServer({
        agent: new http.Agent({ keepAlive: true, maxSockets: proxyConfig.ctn_max || 50, timeout: 60000 }),
        changeOrigin: true,
        preserveHeaderKeyCase: true,
        proxyTimeout: 120000,
    });

    function checkVerification(req, res, next) {
        const { oka_validation_token, oka_validation_expiration } = req.cookies;
        if (!oka_validation_token || !oka_validation_expiration || Date.now() > Number(oka_validation_expiration)) {
            const newExpirationTime = Date.now() + proxyConfig.expired * 1000;
            const newToken = encryptToken(newExpirationTime.toString(), proxyConfig.secret_key);
            res.cookie("oka_validation_token", newToken, { maxAge: proxyConfig.expired * 1000 });
            res.cookie("oka_validation_expiration", newExpirationTime, { maxAge: proxyConfig.expired * 1000, httpOnly: true, secure: true });
            res.status(200).send(verificationPage);
            return;
        }
        if (!verifyToken(oka_validation_expiration.toString(), oka_validation_token, proxyConfig.secret_key)) {
            res.clearCookie("oka_validation_token");
            res.clearCookie("oka_validation_expiration");
            res.status(200).send(verificationPage);
            return;
        }
        next();
    }

    app.use(cookieParser());
    app.use(compression());
    app.use(checkVerification);
    app.use(rateLimitMiddleware);

    app.all("*", (req, res) => {
        proxy.web(req, res, { target: proxyConfig.target_url }, (err) => {
            logRequestFailure(req, err);
            res.writeHead(502, { "Content-Type": "text/html" });
            res.end(errorPage);
        });
    });

    proxy.on("error", (err, req, res) => {
        logRequestFailure(req, err);
        if (!res.headersSent) {
            res.writeHead(502, { "Content-Type": "text/html" });
            res.end(errorPage);
        }
    });

    return app;
}

function startServers(config) {
    if (!config.server || !Array.isArray(config.server)) {
        logger.error("No valid server configuration found.");
        process.exit(1);
    }
    config.server.forEach((proxyConfig) => {
        if (!proxyConfig.name) {
            logger.error("Each server configuration must have a name.");
            process.exit(1);
        }
        const app = createProxyServer(proxyConfig);
        const port = proxyConfig.port || 3000;
        if (proxyConfig.https && proxyConfig.https.enabled) {
            if (!fs.existsSync(proxyConfig.https.cert_path) || !fs.existsSync(proxyConfig.https.key_path)) {
                logger.error("HTTPS enabled, but certificate or key file is missing.");
                process.exit(1);
            }
            const sslOptions = { key: loadFile(proxyConfig.https.key_path, ""), cert: loadFile(proxyConfig.https.cert_path, "") };
            https.createServer(sslOptions, app).listen(port, () => logger.info(`HTTPS server running on https://127.0.0.1:${port}`));
        } else {
            http.createServer(app).listen(port, () => logger.info(`HTTP server running on http://127.0.0.1:${port}`));
        }
    });
}

initializeConfig();
const config = parseConfig();
startServers(config);
