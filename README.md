# OkaProxy 🚀

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](https://hub.docker.com/)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen)]()

A high-performance, secure HTTP proxy server written in **Go** with DDoS protection, rate limiting, and advanced security features.

## ✨ Features

- 🔒 **Security First**: Built-in DDoS protection and bot detection
- ⚡ **High Performance**: Powered by Gin framework with optimized connection pooling
- 🛡️ **Rate Limiting**: Redis-based rate limiting with configurable thresholds
- 🍪 **Cookie-based Verification**: Secure token-based authentication system
- 🌍 **GeoIP Support**: Geographic location tracking and logging
- 📊 **Comprehensive Logging**: Structured logging with request tracking
- 🔧 **Easy Configuration**: TOML-based configuration with validation
- 🐳 **Docker Ready**: Complete Docker and Docker Compose support
- 📈 **Health Monitoring**: Built-in health checks and status endpoints
- 🔐 **HTTPS Support**: Full SSL/TLS encryption support

## 🚀 Quick Start

### Using Go (Recommended)

```bash
# Clone the repository
git clone -b go https://github.com/GentsunCheng/okaproxy.git
cd okaproxy

# Install dependencies
go mod download

# Initialize configuration
make init-config
# Edit config.toml with your settings

# Build and run
make build
./okaproxy
```

### Using Docker

```bash
# Clone and start with Docker Compose
git clone https://github.com/GentsunCheng/okaproxy.git
cd okaproxy

# Copy and edit configuration
cp config.toml.example config.toml
# Edit config.toml with your settings

# Start services
docker-compose up -d
```

### Using Pre-built Binaries

Download the latest release from [GitHub Releases](https://github.com/GentsunCheng/okaproxy/releases)

```bash
# Linux/macOS
wget https://github.com/GentsunCheng/okaproxy/releases/latest/download/okaproxy-linux-amd64.tar.gz
tar xzf okaproxy-linux-amd64.tar.gz
./okaproxy-linux-amd64 --config config.toml

# Windows
# Download and extract okaproxy-windows-amd64.exe
okaproxy-windows-amd64.exe --config config.toml
```

## ⚙️ Configuration

OkaProxy uses TOML configuration files. Copy `config.toml.example` to `config.toml` and customize:

```toml
# Rate limiting
[limit]
count = 100    # Max requests per window
window = 60    # Time window in seconds

# Proxy server configuration
[[server]]
name = "my-proxy"
port = 3000
target_url = "http://localhost:8080"
secret_key = "your-secret-key-here"
expired = 300  # Cookie expiration (seconds)
ctn_max = 50   # Max connections

# HTTPS configuration (optional)
[server.https]
enabled = true
cert_path = "/path/to/cert.pem"
key_path = "/path/to/key.pem"
```

### Key Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `count` | Max requests per time window (0=disabled) | 100 |
| `window` | Rate limit window in seconds | 60 |
| `port` | Server listening port | 3000 |
| `target_url` | Upstream server URL | - |
| `secret_key` | Cookie encryption key (change this!) | - |
| `expired` | Cookie expiration time in seconds | 300 |
| `ctn_max` | Max upstream connections (0=unlimited) | 50 |

## 🔧 Development

### Prerequisites

- Go 1.23 or later
- Redis (for rate limiting)
- Make (optional, for convenience)

### Setup Development Environment

```bash
# Install development tools
make setup

# Run in development mode with auto-reload
make dev

# Run tests
make test

# Check code quality
make ci
```

### Available Make Commands

```bash
make help           # Show all available commands
make build          # Build the application
make run            # Run the application
make test           # Run tests with coverage
make docker         # Build Docker image
make release        # Create release builds for all platforms
make clean          # Clean build artifacts
```

## 📊 Monitoring & Health Checks

OkaProxy provides built-in endpoints for monitoring:

- `GET /health` - Health check endpoint
- `GET /status` - Detailed status information

### Example Health Check Response

```json
{
  "status": "healthy",
  "timestamp": 1640995200,
  "version": "1.0.0"
}
```

## 🐳 Docker Deployment

### Basic Deployment

```bash
# Using Docker Compose (recommended)
docker-compose up -d

# Using Docker directly
docker build -t okaproxy .
docker run -d -p 3000:3000 -v $(pwd)/config.toml:/app/config.toml okaproxy
```

### Production Deployment with Nginx

```bash
# Start with production profile (includes Nginx)
docker-compose --profile production up -d
```

## 🔒 Security Features

### DDoS Protection
- Redis-based rate limiting
- Configurable request thresholds
- Automatic IP blocking

### Bot Detection
- Cookie-based verification challenges
- JavaScript verification page
- Behavioral analysis

### Security Headers
- X-Content-Type-Options: nosniff
- X-Frame-Options: DENY
- X-XSS-Protection: 1; mode=block
- Strict-Transport-Security (HTTPS only)

## 📝 Logging

OkaProxy provides comprehensive structured logging:

```bash
# View real-time logs
make logs

# Docker logs
docker-compose logs -f okaproxy
```

Log files are stored in the `logs/` directory:
- `combined.log` - All log messages
- Automatic log rotation and compression
- GeoIP location tracking for requests

## 🌍 GeoIP Support

OkaProxy can track geographic locations of requests:

1. Download GeoLite2 database from [MaxMind](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data)
2. Place `GeoLite2-City.mmdb` in the application directory
3. Restart OkaProxy to enable geographic logging

## 🛠️ Advanced Configuration

### Multiple Proxy Servers

```toml
# Primary web proxy
[[server]]
name = "web-proxy"
port = 3000
target_url = "http://web-server:80"

# API proxy with shorter cookie expiration
[[server]]
name = "api-proxy"
port = 3001
target_url = "http://api-server:8080"
expired = 120  # 2 minutes for APIs

# Secure HTTPS proxy
[[server]]
name = "secure-proxy"
port = 3443
target_url = "https://secure-backend"
[server.https]
enabled = true
cert_path = "/etc/ssl/certs/domain.pem"
key_path = "/etc/ssl/private/domain.key"
```

### Environment Variables

```bash
GIN_MODE=release          # Set Gin to release mode
TZ=UTC                   # Set timezone
REDIS_URL=redis://...    # Redis connection string
```

## 🚨 Troubleshooting

### Common Issues

1. **Redis Connection Failed**
   ```bash
   # Check Redis status
   redis-cli ping
   
   # Start Redis with Docker
   docker run -d -p 6379:6379 redis:alpine
   ```

2. **Certificate Errors**
   ```bash
   # Check certificate files
   openssl x509 -in cert.pem -text -noout
   
   # Generate self-signed certificate for testing
   openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes
   ```

3. **Permission Denied**
   ```bash
   # Ensure proper file permissions
   chmod +x okaproxy
   chmod 600 config.toml
   ```

### Debug Mode

```bash
# Enable debug logging
GIN_MODE=debug ./okaproxy --config config.toml

# View detailed logs
tail -f logs/combined.log | jq .
```

## 🤝 Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Development Workflow

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes and add tests
4. Run quality checks: `make ci`
5. Commit your changes: `git commit -m 'Add amazing feature'`
6. Push to the branch: `git push origin feature/amazing-feature`
7. Open a Pull Request

## 📄 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Gin Web Framework](https://gin-gonic.com/) - High-performance HTTP web framework
- [Redis](https://redis.io/) - In-memory data structure store
- [Logrus](https://github.com/sirupsen/logrus) - Structured logger for Go
- [MaxMind GeoIP2](https://github.com/oschwald/geoip2-golang) - GeoIP2 database reader

## 🔗 Links

- [Documentation](https://github.com/GentsunCheng/okaproxy/wiki)
- [Docker Hub](https://hub.docker.com/r/okaproxy/okaproxy)
- [Issue Tracker](https://github.com/GentsunCheng/okaproxy/issues)
- [Releases](https://github.com/GentsunCheng/okaproxy/releases)

---

**Made with ❤️ by the OkaProxy Team**
