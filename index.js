const http = require("http");
const https = require("https");
const httpProxy = require("http-proxy");
const express = require("express");
const rateLimit = require("express-rate-limit");
const crypto = require("crypto");
const geoip = require("geoip-lite");
const cookieParser = require("cookie-parser");
const fs = require("fs");
const toml = require("toml");
const compression = require("compression");

const configFilePath = "config.toml";
const exampleConfigFilePath = "config.toml.example";

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
    const clientIp = req.headers['cf-connecting-ip'] || req.headers['x-real-ip'] || req.headers['x-forwarded-for'] || req.connection.remoteAddress;
    console.error(`[ERROR] ${new Date().toISOString()} | IP: ${clientIp} | Location: ${getGeolocation(clientIp)} | Error: ${err.message}`);
}

function encryptToken(data, secret_key) {
    return crypto.createHmac("sha256", secret_key).update(data).digest("hex");
}

function verifyToken(data, token, secret_key) {
    return encryptToken(data, secret_key) === token;
}

function createProxyServer(proxyConfig) {
    const app = express();
    const proxy = httpProxy.createProxyServer({
        agent: new http.Agent({ keepAlive: true, maxSockets: proxyConfig.ctn.max || 50, timeout: 60000 }),
        changeOrigin: true,
        preserveHeaderKeyCase: true,
    });

    const limiter = rateLimit({
        windowMs: (proxyConfig.limit.time || 600) * 1000,
        max: proxyConfig.limit.count || 100,
        message: "Too many requests, please try again later.",
        keyGenerator: (req) => req.headers['cf-connecting-ip'] || req.headers['x-real-ip'] || req.headers['x-forwarded-for']?.split(',')[0] || req.connection.remoteAddress,
    });

    const compressor = compression({
        level: 6, 
        threshold: 1024, 
        filter: (req, res) => {
            if (req.headers['x-no-compression']) {
                return false;
            }
            return compression.filter(req, res);
        }
    })

    function checkVerification(req, res, next) {
        const { validation_token, validation_expiration } = req.cookies;
        if (!validation_token || !validation_expiration || Date.now() > validation_expiration) {
            const newExpirationTime = Date.now() + proxyConfig.expired * 1000;
            const newToken = encryptToken(newExpirationTime.toString(), proxyConfig.secret_key);
            res.cookie("validation_token", newToken, { maxAge: proxyConfig.expired * 1000 });
            res.cookie("validation_expiration", newExpirationTime, { maxAge: proxyConfig.expired * 1000, httpOnly: true, secure: true });
            res.status(200).send(loadFile("public/verification.html", "<h1>Verification</h1><script>setTimeout(() => window.location.reload(), 5000);</script>"));
            return;
        }
        if (!verifyToken(validation_expiration.toString(), validation_token, proxyConfig.secret_key)) {
            res.clearCookie("validation_token");
            res.clearCookie("validation_expiration");
            res.status(200).send(loadFile("public/verification.html", "<h1>Verification</h1><script>setTimeout(() => window.location.reload(), 5000);</script>"));
            return;
        }
        next();
    }

    app.use(cookieParser());
    app.use(checkVerification);
    app.use(limiter);
    app.use(compressor);

    app.all("*", (req, res) => {
        proxy.web(req, res, { target: proxyConfig.target_url }, (err) => {
            logRequestFailure(req, err);
            res.writeHead(502, { "Content-Type": "text/html" });
            res.end(loadFile("public/502.html", "<h1>502 Bad Gateway</h1>"));
        });
    });

    proxy.on("error", (err, req, res) => {
        logRequestFailure(req, err);
        res.writeHead(500, { "Content-Type": "text/html" });
        res.end(loadFile("public/502.html", "<h1>502 Bad Gateway</h1>"));
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
