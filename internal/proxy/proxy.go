package proxy

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	
	"okaproxy/internal/config"
	"okaproxy/internal/logger"
)

// ProxyManager manages HTTP proxy operations
type ProxyManager struct {
	logger    *logger.Logger
	errorPage string
}

// NewProxyManager creates a new proxy manager
func NewProxyManager(logger *logger.Logger, errorPage string) *ProxyManager {
	return &ProxyManager{
		logger:    logger,
		errorPage: errorPage,
	}
}

// CreateReverseProxy creates a reverse proxy for the given target URL and configuration
func (pm *ProxyManager) CreateReverseProxy(serverConfig *config.ServerConfig) (*httputil.ReverseProxy, error) {
	// Parse target URL
	target, err := url.Parse(serverConfig.TargetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse target URL: %v", err)
	}

	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Configure transport
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// Set connection limits if specified
	if serverConfig.CtnMax > 0 {
		transport.MaxIdleConnsPerHost = serverConfig.CtnMax
		transport.MaxConnsPerHost = serverConfig.CtnMax
	}

	proxy.Transport = transport

	// Custom director to modify requests
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		
		// Preserve original Host header or use target host
		if req.Header.Get("Host") == "" {
			req.Host = target.Host
		}
		
		// Add X-Forwarded-For header
		clientIP := pm.getClientIP(req)
		if prior, ok := req.Header["X-Forwarded-For"]; ok {
			clientIP = strings.Join(prior, ", ") + ", " + clientIP
		}
		req.Header.Set("X-Forwarded-For", clientIP)
		
		// Add X-Real-IP header
		req.Header.Set("X-Real-IP", pm.getClientIP(req))
		
		// Add X-Forwarded-Proto header
		if req.TLS != nil {
			req.Header.Set("X-Forwarded-Proto", "https")
		} else {
			req.Header.Set("X-Forwarded-Proto", "http")
		}

		// Add X-Forwarded-Host header
		req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))

		// Log the proxied request
		pm.logger.WithFields(map[string]interface{}{
			"method":     req.Method,
			"url":        req.URL.String(),
			"target":     target.String(),
			"client_ip":  clientIP,
			"user_agent": req.Header.Get("User-Agent"),
		}).Debug("Proxying request")
	}

	// Custom error handler
	proxy.ErrorHandler = pm.createErrorHandler(serverConfig)

	// Custom response modifier
	originalModifyResponse := proxy.ModifyResponse
	proxy.ModifyResponse = func(resp *http.Response) error {
		// Call original modifier if exists
		if originalModifyResponse != nil {
			if err := originalModifyResponse(resp); err != nil {
				return err
			}
		}

		// Add security headers to response
		resp.Header.Set("X-Proxy-By", "OkaProxy")
		resp.Header.Set("X-Content-Type-Options", "nosniff")
		
		// Remove potentially sensitive headers
		resp.Header.Del("Server")
		resp.Header.Del("X-Powered-By")

		return nil
	}

	return proxy, nil
}

// createErrorHandler creates a custom error handler for the proxy
func (pm *ProxyManager) createErrorHandler(serverConfig *config.ServerConfig) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		pm.logger.LogRequestFailure(r, err)

		// Set error headers
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Proxy-Error", "true")
		
		// Write error page
		w.WriteHeader(http.StatusBadGateway)
		
		if pm.errorPage != "" {
			io.WriteString(w, pm.errorPage)
		} else {
			io.WriteString(w, `
			<!DOCTYPE html>
			<html>
			<head>
				<title>502 Bad Gateway</title>
				<style>
					body { font-family: Arial, sans-serif; text-align: center; margin-top: 100px; }
					.error { color: #e74c3c; font-size: 24px; }
					.message { color: #7f8c8d; margin-top: 20px; }
				</style>
			</head>
			<body>
				<div class="error">502 Bad Gateway</div>
				<div class="message">The server is temporarily unavailable. Please try again later.</div>
			</body>
			</html>
			`)
		}
	}
}

// ProxyHandler creates a Gin handler that proxies requests
func (pm *ProxyManager) ProxyHandler(serverConfig *config.ServerConfig) gin.HandlerFunc {
	proxy, err := pm.CreateReverseProxy(serverConfig)
	if err != nil {
		pm.logger.Errorf("Failed to create reverse proxy: %v", err)
		return func(c *gin.Context) {
			c.String(http.StatusInternalServerError, "Proxy configuration error")
		}
	}

	return func(c *gin.Context) {
		// Use the reverse proxy to handle the request
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

// getClientIP extracts the real client IP from the request
func (pm *ProxyManager) getClientIP(r *http.Request) string {
	return logger.GetClientIP(r)
}

// HealthCheckHandler provides a health check endpoint
func (pm *ProxyManager) HealthCheckHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
			"version":   "1.0.0",
		})
	}
}

// StatusHandler provides server status information
func (pm *ProxyManager) StatusHandler(serverConfig *config.ServerConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Test target connectivity
		targetStatus := "unknown"
		if targetURL, err := url.Parse(serverConfig.TargetURL); err == nil {
			if resp, err := http.Get(targetURL.String()); err == nil {
				resp.Body.Close()
				targetStatus = fmt.Sprintf("reachable (status: %d)", resp.StatusCode)
			} else {
				targetStatus = "unreachable: " + err.Error()
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"server_name":   serverConfig.Name,
			"target_url":    serverConfig.TargetURL,
			"target_status": targetStatus,
			"uptime":        time.Since(time.Now()).String(), // This should be actual uptime
			"timestamp":     time.Now().Unix(),
		})
	}
}

// WebSocketProxy handles WebSocket proxy connections
type WebSocketProxy struct {
	target *url.URL
	logger *logger.Logger
}

// NewWebSocketProxy creates a new WebSocket proxy
func NewWebSocketProxy(targetURL string, logger *logger.Logger) (*WebSocketProxy, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}
	
	// Convert HTTP(S) to WS(S)
	if target.Scheme == "http" {
		target.Scheme = "ws"
	} else if target.Scheme == "https" {
		target.Scheme = "wss"
	}
	
	return &WebSocketProxy{
		target: target,
		logger: logger,
	}, nil
}

// ServeHTTP handles WebSocket proxy requests
func (wsp *WebSocketProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// This is a simplified WebSocket proxy implementation
	// For production use, consider using gorilla/websocket or similar
	wsp.logger.Info("WebSocket proxy request received (simplified implementation)")
	
	// Return a simple response for now
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("WebSocket proxy not fully implemented"))
}