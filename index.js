const express = require("express");
const httpProxy = require("http-proxy");
const cookieParser = require("cookie-parser");

const path = require('path');
const crypto = require('crypto');
const fs = require("fs");
const toml = require("toml");

// Read TOML file
let data;
let veri_page;
try {
    data = fs.readFileSync("config.toml", "utf8");
} catch (err) {
    console.error("Error reading file:", err);
}

try {
    veri_page = fs.readFileSync("public/verification.html", "utf8");
} catch (err) {
    console.error("Error reading file:", err);
}

try {
    not_found_page = fs.readFileSync("public/404.html", "utf8");
} catch (err) {
    console.error("Error reading file:", err);
}

// Parse TOML file
let config;
try {
    config = toml.parse(data);
} catch (parseError) {
    console.error("Parse TOML failed:", parseError);
    return;
}

// Create Express app
const app = express();
const proxy = httpProxy.createProxyServer({});
const port = config.port || 4000;
const targetUrl = config.target_url;
const expired = config.expired;
const secret_key = config.secret_key;

function encryptToken(data) {
    const hmac = crypto.createHmac('sha256', secret_key);
    hmac.update(data);
    return hmac.digest('hex');
}

function verifyToken(data, token) {
    const expectedToken = encryptToken(data);
    return expectedToken === token;
}

app.use(cookieParser());

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
            secure: true
        });

        // Generate the HTML for the verification page
        res.status(200).send(veri_page);
        return;
    }

    if (!verifyToken(expirationTime.toString(), validationToken)) {
        res.clearCookie('validation_token');
        res.clearCookie('validation_expiration');
        res.status(200).send(veri_page);
        return;
    }

    next();
}

// Middleware to verify the token
app.use(checkVerification);

// Proxy all requests
app.all('*', (req, res) => {
    proxy.web(req, res, { target: targetUrl }, (err) => {
        console.error(`Proxy error: Unable to connect to ${targetUrl}`);
        handleNotFound(req, res);
    });
});

// Error handling for proxy server
proxy.on('error', (err, req, res) => {
    console.error('Proxy encountered an error:', err.message);
    handleNotFound(req, res);
});

// Function to handle 404 responses
function handleNotFound(req, res) {
    const ext = path.extname(req.path).toLowerCase();
    if (!ext || ext === '.html') {
        res.status(404).send(not_found_page);
    } else {
        res.status(404).send('Not Found');
    }
}

app.listen(port, () => {
    console.log(`Server running on http://127.0.0.1:${port}`);
});
