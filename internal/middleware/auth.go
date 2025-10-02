package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	
	"okaproxy/internal/config"
	"okaproxy/internal/logger"
)

const (
	ValidationTokenCookie     = "oka_validation_token"
	ValidationExpirationCookie = "oka_validation_expiration"
)

// AuthMiddleware provides authentication and verification functionality
type AuthMiddleware struct {
	logger           *logger.Logger
	verificationPage string
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(logger *logger.Logger, verificationPage string) *AuthMiddleware {
	return &AuthMiddleware{
		logger:           logger,
		verificationPage: verificationPage,
	}
}

// encryptToken creates an HMAC-SHA256 token
func (am *AuthMiddleware) encryptToken(data, secretKey string) string {
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// verifyToken verifies an HMAC-SHA256 token using constant-time comparison
func (am *AuthMiddleware) verifyToken(data, token, secretKey string) bool {
	expected := am.encryptToken(data, secretKey)
	expectedBytes := []byte(expected)
	tokenBytes := []byte(token)
	
	// Ensure both slices have the same length to prevent timing attacks
	if len(expectedBytes) != len(tokenBytes) {
		return false
	}
	
	return subtle.ConstantTimeCompare(expectedBytes, tokenBytes) == 1
}

// CheckVerification creates a middleware that checks for valid verification cookies
func (am *AuthMiddleware) CheckVerification(serverConfig *config.ServerConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get validation cookies
		validationToken, err := c.Cookie(ValidationTokenCookie)
		if err != nil || validationToken == "" {
			am.showVerificationPage(c, serverConfig)
			return
		}
		
		validationExpirationStr, err := c.Cookie(ValidationExpirationCookie)
		if err != nil || validationExpirationStr == "" {
			am.showVerificationPage(c, serverConfig)
			return
		}
		
		// Parse expiration time
		validationExpiration, err := strconv.ParseInt(validationExpirationStr, 10, 64)
		if err != nil {
			am.clearCookiesAndShowVerification(c, serverConfig)
			return
		}
		
		// Check if token has expired
		if time.Now().UnixMilli() > validationExpiration {
			am.clearCookiesAndShowVerification(c, serverConfig)
			return
		}
		
		// Verify token
		if !am.verifyToken(validationExpirationStr, validationToken, serverConfig.SecretKey) {
			am.clearCookiesAndShowVerification(c, serverConfig)
			return
		}
		
		// Token is valid, continue to next middleware
		c.Next()
	}
}

// showVerificationPage displays the verification page and sets new cookies
func (am *AuthMiddleware) showVerificationPage(c *gin.Context, serverConfig *config.ServerConfig) {
	// Generate new expiration time
	newExpirationTime := time.Now().UnixMilli() + int64(serverConfig.Expired*1000)
	newExpirationStr := strconv.FormatInt(newExpirationTime, 10)
	
	// Generate new token
	newToken := am.encryptToken(newExpirationStr, serverConfig.SecretKey)
	
	// Set cookies
	c.SetCookie(
		ValidationTokenCookie,
		newToken,
		serverConfig.Expired,
		"/",
		"",
		false, // secure (set to true in HTTPS)
		true,  // httpOnly
	)
	
	c.SetCookie(
		ValidationExpirationCookie,
		newExpirationStr,
		serverConfig.Expired,
		"/",
		"",
		false, // secure (set to true in HTTPS)
		true,  // httpOnly
	)
	
	// Show verification page
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, am.verificationPage)
	c.Abort()
}

// clearCookiesAndShowVerification clears invalid cookies and shows verification page
func (am *AuthMiddleware) clearCookiesAndShowVerification(c *gin.Context, serverConfig *config.ServerConfig) {
	// Clear invalid cookies
	c.SetCookie(ValidationTokenCookie, "", -1, "/", "", false, true)
	c.SetCookie(ValidationExpirationCookie, "", -1, "/", "", false, true)
	
	// Show verification page with new cookies
	am.showVerificationPage(c, serverConfig)
}

// SecurityHeadersMiddleware adds security headers
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		
		// Don't add HSTS header for HTTP connections
		if c.Request.TLS != nil {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		
		c.Next()
	}
}

// CORSMiddleware adds CORS headers
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		
		c.Next()
	}
}

// CompressionMiddleware enables gzip compression (Gin has built-in gzip middleware)
func CompressionMiddleware() gin.HandlerFunc {
	return gin.Recovery() // Placeholder, use gin's gzip middleware in actual implementation
}

// RequestIDMiddleware adds a unique request ID to each request
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := generateRequestID()
		c.Header("X-Request-ID", requestID)
		c.Set("RequestID", requestID)
		c.Next()
	}
}

// generateRequestID generates a simple request ID
func generateRequestID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

// LoggerMiddleware creates a custom logger middleware
func LoggerMiddleware(lg *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		startTime := time.Now()
		
		// Process request
		c.Next()
		
		// Calculate latency
		latency := time.Since(startTime)
		
		// Get request info
		clientIP := logger.GetClientIP(c.Request)
		method := c.Request.Method
		path := c.Request.URL.Path
		statusCode := c.Writer.Status()
		
		// Log the request
		lg.WithFields(map[string]interface{}{
			"ip":       clientIP,
			"method":   method,
			"path":     path,
			"status":   statusCode,
			"latency":  latency,
			"location": lg.GetGeolocation(clientIP),
		}).Info("Request processed")
	}
}