const express = require("express");
const httpProxy = require("http-proxy");
const cookieParser = require("cookie-parser");

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
let not_found_page;
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
    not_found_page = fs.readFileSync("public/404.html", "utf8");
} catch (err) {
    console.error("Error reading file:", err);
    not_found_page = "<h1>404 Not Found</h1>";
}

// Parse TOML file
let config;
try {
    config = toml.parse(data);
} catch (parseError) {
    console.error("Parse TOML failed:", parseError);
    process.exit(1);
}

Object.entries(config).forEach(([key, proxyConfig]) => {
    // Create Express app
    const app = express();
    const proxy = httpProxy.createProxyServer({});
    const port = proxyConfig.port;
    const targetUrl = proxyConfig.target_url;
    const expired = proxyConfig.expired;
    const secret_key = proxyConfig.secret_key;

    function encryptToken(data) {
        const hmac = crypto.createHmac("sha256", secret_key);
        hmac.update(data);
        return hmac.digest("hex");
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

    // Middleware to verify the token
    app.use(checkVerification);

    // Proxy all requests
    app.all("*", (req, res) => {
        proxy.web(req, res, { target: targetUrl }, (err) => {
            console.error(`Proxy error: Unable to connect to ${targetUrl}`);
            handleNotFound(req, res);
        });
    });

    // Error handling for proxy server
    proxy.on("error", (err, req, res) => {
        console.error("Proxy encountered an error:", err.message);
        handleNotFound(req, res);
    });

    // Function to handle 404 responses
    function handleNotFound(req, res) {
        const ext = path.extname(req.path).toLowerCase();
        if (!ext || ext === ".html") {
            res.status(404).send(not_found_page);
        } else {
            res.status(404).send("Not Found");
        }
    }

    app.listen(port, () => {
        console.log(`Server running on http://127.0.0.1:${port}`);
    });
});