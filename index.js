const express = require("express");
const http = require("http");
const https = require("https");
const httpProxy = require("http-proxy");
const cookieParser = require("cookie-parser");
const geoip = require("geoip-lite");
const crypto = require("crypto");
const fs = require("fs");
const toml = require("toml");
const compression = require("compression");
const Redis = require("ioredis");

const configFilePath = "config.toml";
const exampleConfigFilePath = "config.toml.example";

const redis = new Redis({
    connectTimeout: 10000,
    maxRetriesPerRequest: 3
});

function initializeConfig() {
    if (!fs.existsSync(configFilePath)) {
        if (fs.existsSync(exampleConfigFilePath)) {
            fs.copyFileSync(exampleConfigFilePath, configFilePath);
            console.log(`First time running. Please edit ${configFilePath}.`);
        } else {
            console.error("Both 'config.toml' are missing. Please provide a valid configuration file.");
        }
        process.exit(1);
    }
}

function loadFile(filePath, defaultValue) {
    try {
        return fs.readFileSync(filePath, "utf8");
    } catch (err) {
        console.error(`Error reading file ${filePath}:`, err);
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
        console.error("Parse TOML failed:", parseError);
        process.exit(1);
    }
}

function getGeolocation(ip) {
    const geo = geoip.lookup(ip);
    return geo ? `${geo.country} - ${geo.city} (${geo.latitude}, ${geo.longitude})` : "Unknown location";
}

function logRequestFailure(req, err) {
    const clientIp = getClientIp(req);
    console.error(`[ERROR] ${new Date().toISOString()} | IP: ${clientIp} | Location: ${getGeolocation(clientIp)} | Error: ${err.message}`);
}

function encryptToken(data, secret_key) {
    return crypto.createHmac("sha256", secret_key).update(data).digest("hex");
}

function verifyToken(data, token, secret_key) {
    return encryptToken(data, secret_key) === token;
}

async function rateLimitMiddleware(req, res, next) {
    const clientIp = getClientIp(req);
    const limit = 100;
    const window = 600;
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
    if (requests > limit) {
        res.status(429).json({ message: "Too many requests, please try again later." });
        return;
    }
    next();
}

function createProxyServer(proxyConfig) {
    const app = express();
    const proxy = httpProxy.createProxyServer({
        agent: new http.Agent({ keepAlive: true, maxSockets: proxyConfig.ctn.max || 50, timeout: 60000 }),
        changeOrigin: true,
        preserveHeaderKeyCase: true,
        proxyTimeout: 120000,
    });

    function checkVerification(req, res, next) {
        const { oka_validation_token, oka_validation_expiration } = req.cookies;
        if (!oka_validation_token || !oka_validation_expiration || Date.now() > oka_validation_expiration) {
            const newExpirationTime = Date.now() + proxyConfig.expired * 1000;
            const newToken = encryptToken(newExpirationTime.toString(), proxyConfig.secret_key);
            res.cookie("oka_validation_token", newToken, { maxAge: proxyConfig.expired * 1000 });
            res.cookie("oka_validation_expiration", newExpirationTime, { maxAge: proxyConfig.expired * 1000, httpOnly: true, secure: true });
            res.status(200).send(loadFile("public/verification.html", "<h1>Verification</h1><script>setTimeout(() => window.location.reload(), 5000);</script>"));
            return;
        }
        if (!verifyToken(oka_validation_expiration.toString(), oka_validation_token, proxyConfig.secret_key)) {
            res.clearCookie("oka_validation_token");
            res.clearCookie("oka_validation_expiration");
            res.status(200).send(loadFile("public/verification.html", "<h1>Verification</h1><script>setTimeout(() => window.location.reload(), 5000);</script>"));
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
            res.end(loadFile("public/502.html", "<h1>502 Bad Gateway</h1>"));
        });
    });

    proxy.on("error", (err, req, res) => {
        logRequestFailure(req, err);
        if (!res.headersSent) {
            res.writeHead(502, { "Content-Type": "text/html" });
            res.end(loadFile("public/502.html", "<h1>502 Bad Gateway</h1>"));
        }
    });

    return app;
}

function startServers(config) {
    if (!config.server || !Array.isArray(config.server)) {
        console.error("No valid server configuration found.");
        process.exit(1);
    }
    config.server.forEach((proxyConfig) => {
        if (!proxyConfig.name) {
            console.error("Each server configuration must have a name.");
            process.exit(1);
        }
        const app = createProxyServer(proxyConfig);
        const port = proxyConfig.port || 3000;
        if (proxyConfig.https.enabled) {
            if (!fs.existsSync(proxyConfig.https.cert_path) || !fs.existsSync(proxyConfig.https.key_path)) {
                console.error("HTTPS enabled, but certificate or key file is missing.");
                process.exit(1);
            }
            const sslOptions = { key: loadFile(proxyConfig.https.key_path, ""), cert: loadFile(proxyConfig.https.cert_path, "") };
            https.createServer(sslOptions, app).listen(port, () => console.log(`HTTPS server running on https://127.0.0.1:${port}`));
        } else {
            http.createServer(app).listen(port, () => console.log(`HTTP server running on http://127.0.0.1:${port}`));
        }
    });
}

initializeConfig();
const config = parseConfig();
startServers(config);
