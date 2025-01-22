const express = require("express");
const http = require("http");
const https = require("https");
const httpProxy = require("http-proxy");
const cookieParser = require("cookie-parser");
const rateLimit = require('express-rate-limit');

const geoip = require("geoip-lite");
const crypto = require("crypto");
const fs = require("fs");
const path = require("path");
const toml = require("toml");


const configFilePath = "config.toml";
const exampleConfigFilePath = "config.toml.example";

// Check if config.toml exists
if (!fs.existsSync(configFilePath)) {
    if (fs.existsSync(exampleConfigFilePath)) {
        // Copy example file to config.toml
        fs.copyFileSync(exampleConfigFilePath, configFilePath);
        console.log(
            `First time running. Please edit ${configFilePath}.`
        );
    } else {
        console.error(
            "Both 'config.toml' are missing. Please provide a valid configuration file."
        );
    }
    // Exit the program after copying the file or error
    process.exit(1);
}

let data;
let veri_page;
let bad_gateway_page;
// Read TOML file
try {
    data = fs.readFileSync(configFilePath, "utf8");
} catch (err) {
    console.error("Error reading file:", err);
    process.exit(1);
}

try {
    veri_page = fs.readFileSync("public/verification.html", "utf8");
} catch (err) {
    console.error("Error reading file:", err);
    veri_page = `
    <!DOCTYPE html>
    <html lang="en">
    <h1>Verification</h1>
    <script>
        setTimeout(function () {
            window.location.href = document.location.toString();
        }, 5000);
    </script>
    </html>
    `;
}

try {
    bad_gateway_page = fs.readFileSync("public/502.html", "utf8");
} catch (err) {
    console.error("Error reading file:", err);
    bad_gateway_page = "<h1>502 Bad Gateway</h1>";
}

// Parse TOML file
let config;
try {
    config = toml.parse(data);
} catch (parseError) {
    console.error("Parse TOML failed:", parseError);
    process.exit(1);
}
// Validate and extract server configurations
const configObject = {};
if (config.server && Array.isArray(config.server)) {
    config.server.forEach((serverConfig) => {
        const name = serverConfig.name;
        if (!name) {
            console.error("Each server configuration must have a name.");
            process.exit(1);
        }
        configObject[name] = serverConfig;
    });
} else {
    console.error("No valid server configuration found.");
    process.exit(1);
}

// Function to get geolocation information of the IP
function getGeolocation(ip) {
    const geo = geoip.lookup(ip);
    if (geo) {
        return `${geo.country} - ${geo.city} (${geo.latitude}, ${geo.longitude})`;
    } else {
        return "Unknown location";
    }
}

// Middleware to log request failure details
function logRequestFailure(req, err) {
    const clientIp = req.headers['cf-connecting-ip'] || req.headers['x-real-ip'] || req.headers['x-forwarded-for'] || req.connection.remoteAddress;
    const location = getGeolocation(clientIp);
    console.error(`[ERROR] ${new Date().toISOString()} | IP: ${clientIp} | Location: ${location} | Error: ${err.message}`);
}

Object.entries(configObject).forEach(([key, proxyConfig]) => {
    const port = proxyConfig.port || 3000;
    const targetUrl = proxyConfig.target_url;
    const expired = proxyConfig.expired;
    const secret_key = proxyConfig.secret_key;
    const maxctn = proxyConfig.ctn.max || 50;
    const limiter_time = proxyConfig.limit.time || 600;
    const limiter_count = proxyConfig.limit.count || 100;
    //https config
    const https_enabled = proxyConfig.https.enabled || false;
    const cert_path = proxyConfig.https.cert_path || "";
    const key_path = proxyConfig.https.key_path || "";
    // Create Express app
    const app = express();
    const agent = new http.Agent({
        keepAlive: true,
        maxSockets: maxctn,
        timeout: 60000,
    });
    const proxy = httpProxy.createProxyServer({
        agent: agent,
        changeOrigin: true,
        preserveHeaderKeyCase: true,
    });
    const limiter = rateLimit({
        windowMs: limiter_time * 1000,
        max: limiter_count,
        message: "Too many requests, please try again later.",
        keyGenerator: (req) => {
            // Get real client IP from headers
            let clientIp;
            if (req.headers['cf-connecting-ip']) {
                clientIp = req.headers['cf-connecting-ip'];
            } else if (req.headers['x-real-ip']) {
                clientIp = req.headers['x-real-ip'];
            }  else if (req.headers['x-forwarded-for']) {
                clientIp = req.headers['x-forwarded-for'].split(',')[0];  // First IP in the list
            } else {
                clientIp = req.connection.remoteAddress;
            }
            return clientIp;
        }
    });

    function encryptToken(data) {
        const hmac = crypto.createHmac("sha256", secret_key);
        hmac.update(data);
        return hmac.digest("hex");
    }

    function verifyToken(data, token) {
        const expectedToken = encryptToken(data);
        return expectedToken === token;
    }

    // Verification middleware
    function checkVerification(req, res, next) {
        const validationToken = req.cookies.validation_token;
        const expirationTime = req.cookies.validation_expiration;

        // Check if the validation token and expiration time are present and valid
        if (!validationToken || !expirationTime || Date.now() > expirationTime) {
            // Verification failed, redirect to the verification page
            const newExpirationTime = Date.now() + expired * 1000;
            const newToken = encryptToken(newExpirationTime.toString());
            res.cookie("validation_token", newToken, { maxAge: expired * 1000 });
            res.cookie("validation_expiration", newExpirationTime, {
                maxAge: expired * 1000,
                httpOnly: true,
                secure: true,
            });

            // Generate the HTML for the verification page
            res.status(200).send(veri_page);
            return;
        }

        if (!verifyToken(expirationTime.toString(), validationToken)) {
            res.clearCookie("validation_token");
            res.clearCookie("validation_expiration");
            res.status(200).send(veri_page);
            return;
        }

        next();
    }

    app.use(cookieParser()); // Use cookie-parser middleware to parse cookies
    app.use(checkVerification); // Apply verification middleware
    app.use(limiter); // Apply rate limiting middleware

    // Proxy all requests
    app.all("*", async (req, res) => {
        proxy.web(req, res, { target: targetUrl }, (err) => {
            logRequestFailure(req, err);
            res.writeHead(502, { "Content-Type": "text/html" });
            res.end(bad_gateway_page);
        });
    });

    // Error handling for proxy server
    proxy.on("error", async (err, req, res) => {
        logRequestFailure(req, err);
        res.writeHead(500, { "Content-Type": "text/html" });
        res.end(bad_gateway_page);
    });

    if (https_enabled) {
        if (!fs.existsSync(cert_path) || !fs.existsSync(key_path)) {
            console.error("HTTPS enabled, but certificate or key file is missing.");
            process.exit(1);
        }

        var sslOptions;
        try {
            sslOptions = {
                key: fs.readFileSync(key_path, "utf8"),
                cert: fs.readFileSync(cert_path, "utf8"),
            };
        } catch (err) {
            console.error("Failed to load SSL certificate:", err);
            process.exit(1);
        }

        https.createServer(sslOptions, app).listen(port, () => {
            console.log(`HTTPS server running on https://127.0.0.1:${port}`);
        });
    } else {
        http.createServer(app).listen(port, () => {
            console.log(`HTTP server running on http://127.0.0.1:${port}`);
        });
    }
});