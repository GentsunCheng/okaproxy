package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	
	"okaproxy/internal/config"
	"okaproxy/internal/logger"
	"okaproxy/internal/middleware"
	"okaproxy/internal/proxy"
)

// Manager manages multiple proxy servers
type Manager struct {
	config       *config.Config
	logger       *logger.Logger
	redisManager *middleware.RedisManager
	servers      []*http.Server
	proxyManager *proxy.ProxyManager
	wg           sync.WaitGroup
	shutdown     chan os.Signal
}

// NewManager creates a new server manager
func NewManager(cfg *config.Config) *Manager {
	// Initialize logger
	log := logger.NewLogger()
	
	// Initialize Redis manager
	redisManager := middleware.NewRedisManager(log)
	
	// Test Redis connection
	if err := redisManager.Ping(); err != nil {
		log.Warnf("Redis connection failed: %v. Rate limiting will be disabled.", err)
	} else {
		log.Info("Redis connection established successfully")
	}

	// Load static pages
	errorPage := loadStaticPage("public/502.html", getDefaultErrorPage())

	// Initialize proxy manager
	proxyManager := proxy.NewProxyManager(log, errorPage)

	return &Manager{
		config:       cfg,
		logger:       log,
		redisManager: redisManager,
		proxyManager: proxyManager,
		shutdown:     make(chan os.Signal, 1),
	}
}

// Start starts all configured proxy servers
func (m *Manager) Start() error {
	if len(m.config.Server) == 0 {
		return fmt.Errorf("no server configurations found")
	}

	// Setup signal handling
	signal.Notify(m.shutdown, syscall.SIGINT, syscall.SIGTERM)

	// Start each server
	for i, serverConfig := range m.config.Server {
		if err := m.startServer(i, &serverConfig); err != nil {
			m.logger.Errorf("Failed to start server %s: %v", serverConfig.Name, err)
			return err
		}
	}

	m.logger.Infof("Started %d proxy servers successfully", len(m.servers))
	return nil
}

// startServer starts a single proxy server
func (m *Manager) startServer(index int, serverConfig *config.ServerConfig) error {
	// Set Gin mode to release for production
	gin.SetMode(gin.ReleaseMode)

	// Create Gin router
	router := gin.New()

	// Add middlewares
	m.addMiddlewares(router, serverConfig)

	// Add routes
	m.addRoutes(router, serverConfig)

	// Create HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", serverConfig.Port),
		Handler: router,
		
		// Timeouts
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
		
		// Security settings
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	// Configure TLS if enabled
	if serverConfig.HTTPS.Enabled {
		// Load TLS certificate
		cert, err := tls.LoadX509KeyPair(serverConfig.HTTPS.CertPath, serverConfig.HTTPS.KeyPath)
		if err != nil {
			return fmt.Errorf("failed to load TLS certificate: %v", err)
		}

		server.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
		}
	}

	// Start server in goroutine
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		
		var err error
		if serverConfig.HTTPS.Enabled {
			m.logger.LogServerStart("HTTPS", serverConfig.Port)
			err = server.ListenAndServeTLS("", "")
		} else {
			m.logger.LogServerStart("HTTP", serverConfig.Port)
			err = server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			m.logger.Errorf("Server %s stopped with error: %v", serverConfig.Name, err)
		}
	}()

	// Store server reference for shutdown
	m.servers = append(m.servers, server)

	return nil
}

// addMiddlewares adds all necessary middlewares to the router
func (m *Manager) addMiddlewares(router *gin.Engine, serverConfig *config.ServerConfig) {
	// Recovery middleware
	router.Use(gin.Recovery())

	// Custom logger middleware
	router.Use(middleware.LoggerMiddleware(m.logger))

	// Request ID middleware
	router.Use(middleware.RequestIDMiddleware())

	// Security headers middleware
	router.Use(middleware.SecurityHeadersMiddleware())

	// CORS middleware
	router.Use(middleware.CORSMiddleware())

	// Gzip compression
	router.Use(gzip.Gzip(gzip.DefaultCompression))

	// Authentication middleware
	verificationPage := loadStaticPage("public/verification.html", getDefaultVerificationPage())
	authMiddleware := middleware.NewAuthMiddleware(m.logger, verificationPage)
	router.Use(authMiddleware.CheckVerification(serverConfig))

	// Rate limiting middleware
	router.Use(m.redisManager.RateLimitMiddleware(m.config))
}

// addRoutes adds all routes to the router
func (m *Manager) addRoutes(router *gin.Engine, serverConfig *config.ServerConfig) {
	// Health check endpoint
	router.GET("/health", m.proxyManager.HealthCheckHandler())

	// Status endpoint
	router.GET("/status", m.proxyManager.StatusHandler(serverConfig))

	// Catch-all proxy handler
	router.NoRoute(m.proxyManager.ProxyHandler(serverConfig))
}

// WaitForShutdown waits for shutdown signal and gracefully shuts down all servers
func (m *Manager) WaitForShutdown() {
	<-m.shutdown
	m.logger.Info("Shutdown signal received, starting graceful shutdown...")

	// Create context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown all servers
	for i, server := range m.servers {
		go func(index int, srv *http.Server) {
			if err := srv.Shutdown(ctx); err != nil {
				m.logger.Errorf("Server %d shutdown error: %v", index, err)
			} else {
				m.logger.Infof("Server %d shutdown completed", index)
			}
		}(i, server)
	}

	// Wait for all servers to shutdown or timeout
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		m.logger.Info("All servers shutdown gracefully")
	case <-ctx.Done():
		m.logger.Warn("Graceful shutdown timeout, forcing exit")
	}

	// Close resources
	m.cleanup()
	m.logger.Info("Shutdown completed")
}

// cleanup closes all resources
func (m *Manager) cleanup() {
	// Close Redis connection
	if m.redisManager != nil {
		m.redisManager.Close()
	}

	// Close logger resources
	if m.logger != nil {
		m.logger.Close()
	}
}

// loadStaticPage loads a static HTML page from file, fallback to default if not found
func loadStaticPage(filePath, defaultContent string) string {
	if content, err := os.ReadFile(filePath); err == nil {
		return string(content)
	}
	return defaultContent
}

// getDefaultVerificationPage returns the default verification page HTML
func getDefaultVerificationPage() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Verification</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            margin: 0;
            padding: 0;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .container {
            background: white;
            padding: 2rem;
            border-radius: 10px;
            box-shadow: 0 10px 25px rgba(0,0,0,0.1);
            text-align: center;
            max-width: 400px;
            width: 90%;
        }
        .spinner {
            border: 4px solid #f3f3f3;
            border-radius: 50%;
            border-top: 4px solid #667eea;
            width: 40px;
            height: 40px;
            animation: spin 1s linear infinite;
            margin: 0 auto 1rem;
        }
        @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
        }
        h1 {
            color: #333;
            margin-bottom: 1rem;
            font-size: 1.5rem;
        }
        p {
            color: #666;
            margin-bottom: 1rem;
        }
        .progress {
            width: 100%;
            height: 4px;
            background: #f0f0f0;
            border-radius: 2px;
            overflow: hidden;
            margin-top: 1rem;
        }
        .progress-bar {
            height: 100%;
            background: #667eea;
            border-radius: 2px;
            animation: progress 5s ease-in-out;
        }
        @keyframes progress {
            0% { width: 0%; }
            100% { width: 100%; }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="spinner"></div>
        <h1>Security Verification</h1>
        <p>Verifying your connection security...</p>
        <p>This process helps protect against automated attacks.</p>
        <div class="progress">
            <div class="progress-bar"></div>
        </div>
    </div>
    <script>
        setTimeout(function() {
            window.location.reload();
        }, 5000);
    </script>
</body>
</html>`
}

// getDefaultErrorPage returns the default error page HTML
func getDefaultErrorPage() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>502 Bad Gateway</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #ff6b6b 0%, #ee5a24 100%);
            margin: 0;
            padding: 0;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .container {
            background: white;
            padding: 2rem;
            border-radius: 10px;
            box-shadow: 0 10px 25px rgba(0,0,0,0.1);
            text-align: center;
            max-width: 400px;
            width: 90%;
        }
        .error-icon {
            font-size: 4rem;
            color: #ff6b6b;
            margin-bottom: 1rem;
        }
        h1 {
            color: #333;
            margin-bottom: 1rem;
            font-size: 1.8rem;
        }
        p {
            color: #666;
            margin-bottom: 1rem;
            line-height: 1.5;
        }
        .retry-button {
            background: #ff6b6b;
            color: white;
            border: none;
            padding: 0.75rem 1.5rem;
            border-radius: 5px;
            cursor: pointer;
            font-size: 1rem;
            margin-top: 1rem;
            transition: background 0.3s;
        }
        .retry-button:hover {
            background: #ff5252;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="error-icon">⚠️</div>
        <h1>502 Bad Gateway</h1>
        <p>The server is temporarily unavailable or under maintenance.</p>
        <p>Please try again in a few moments.</p>
        <button class="retry-button" onclick="window.location.reload()">
            Try Again
        </button>
    </div>
</body>
</html>`
}