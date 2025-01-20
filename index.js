const express = require('express');
const httpProxy = require('http-proxy');
const cookieParser = require('cookie-parser');

const fs = require('fs');
const toml = require('toml');

// Read TOML file
let data;
let veri_page;
try {
    data = fs.readFileSync('config.toml', 'utf8');
} catch (err) {
    console.error('Error reading file:', err);
}

try {
    veri_page = fs.readFileSync('verification.html', 'utf8');
} catch (err) {
    console.error('Error reading file:', err);
} 
  
// Parse TOML file
let config;
try {
    config = toml.parse(data);
} catch (parseError) {
    console.error('Parse TOML failed:', parseError);
    return;
}

// Create Express app
const app = express();
const proxy = httpProxy.createProxyServer({});
const port = config.port || 4000;
const targetUrl = config.target_url;

app.use(cookieParser());

// Verification middleware
function checkVerification(req, res, next) {
const validationToken = req.cookies.validation_token;
const expirationTime = req.cookies.validation_expiration;

// Check if the validation token and expiration time are present and valid
if (!validationToken || !expirationTime || Date.now() > expirationTime) {
    // Verification failed, redirect to the verification page
    const newToken = generateRandomToken();
    const newExpirationTime = Date.now() + 30 * 60 * 1000; // 30 minutes
    res.cookie('validation_token', newToken, { maxAge: 30 * 60 * 1000 });
    res.cookie('validation_expiration', newExpirationTime, { maxAge: 30 * 60 * 1000 });

    // Generate the HTML for the verification page
    res.status(200).send(veri_page);
    return;
}

next();
}

// Generate a random token
function generateRandomToken() {
return Math.random().toString(36).slice(2);
}

// Middleware to verify the token
app.use(checkVerification);

// Proxy all requests
app.all('*', (req, res) => {
proxy.web(req, res, { target: targetUrl });
});

app.listen(port, () => {
console.log(`Server running on http://127.0.0.1:${port}`);
});