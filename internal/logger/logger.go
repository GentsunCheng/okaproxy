package logger

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/oschwald/geoip2-golang"
	"github.com/sirupsen/logrus"
)

// Logger wraps logrus with additional functionality
type Logger struct {
	*logrus.Logger
	geoipDB *geoip2.Reader
}

// NewLogger creates a new logger instance
func NewLogger() *Logger {
	logger := logrus.New()
	
	// Create logs directory if it doesn't exist
	if err := os.MkdirAll("logs", 0755); err != nil {
		logger.Errorf("Failed to create logs directory: %v", err)
	}

	// Configure logger format
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	// Set log level
	logger.SetLevel(logrus.InfoLevel)

	// Add file output
	if file, err := os.OpenFile("logs/combined.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
		logger.SetOutput(file)
	} else {
		logger.Errorf("Failed to open log file: %v", err)
	}

	l := &Logger{Logger: logger}
	l.initGeoIP()

	return l
}

// initGeoIP initializes the GeoIP database
func (l *Logger) initGeoIP() {
	// Try to find GeoLite2 database file
	possiblePaths := []string{
		"GeoLite2-City.mmdb",
		"data/GeoLite2-City.mmdb",
		"/usr/share/GeoIP/GeoLite2-City.mmdb",
		"/opt/GeoIP/GeoLite2-City.mmdb",
	}

	for _, path := range possiblePaths {
		if db, err := geoip2.Open(path); err == nil {
			l.geoipDB = db
			l.Infof("GeoIP database loaded from: %s", path)
			return
		}
	}

	l.Warn("GeoIP database not found. Geographic location features will be disabled.")
}

// GetClientIP extracts the client IP from the request
func GetClientIP(r *http.Request) string {
	// Check CloudFlare header
	if cfIP := r.Header.Get("CF-Connecting-IP"); cfIP != "" {
		return cfIP
	}

	// Check X-Real-IP header
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}

	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP from the comma-separated list
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// GetGeolocation returns the geolocation information for an IP address
func (l *Logger) GetGeolocation(ip string) string {
	if l.geoipDB == nil {
		return "Unknown location (GeoIP disabled)"
	}

	netIP := net.ParseIP(ip)
	if netIP == nil {
		return "Invalid IP address"
	}

	record, err := l.geoipDB.City(netIP)
	if err != nil {
		return "Unknown location"
	}

	var location strings.Builder
	
	// Add country
	if record.Country.Names["en"] != "" {
		location.WriteString(record.Country.Names["en"])
	}

	// Add city
	if record.City.Names["en"] != "" {
		if location.Len() > 0 {
			location.WriteString(" - ")
		}
		location.WriteString(record.City.Names["en"])
	}

	// Add coordinates if available
	if record.Location.Latitude != 0 || record.Location.Longitude != 0 {
		if location.Len() > 0 {
			location.WriteString(" ")
		}
		location.WriteString(fmt.Sprintf("(%.4f, %.4f)", 
			record.Location.Latitude, record.Location.Longitude))
	}

	if location.Len() == 0 {
		return "Unknown location"
	}

	return location.String()
}

// LogRequestFailure logs a failed request with IP and location information
func (l *Logger) LogRequestFailure(r *http.Request, err error) {
	clientIP := GetClientIP(r)
	location := l.GetGeolocation(clientIP)
	
	l.WithFields(logrus.Fields{
		"ip":       clientIP,
		"location": location,
		"method":   r.Method,
		"url":      r.URL.String(),
		"error":    err.Error(),
	}).Warn("Request failed")
}

// LogRateLimit logs a rate limit violation
func (l *Logger) LogRateLimit(r *http.Request) {
	clientIP := GetClientIP(r)
	location := l.GetGeolocation(clientIP)
	
	l.WithFields(logrus.Fields{
		"ip":       clientIP,
		"location": location,
		"method":   r.Method,
		"url":      r.URL.String(),
	}).Info("[RATE LIMIT] Request blocked")
}

// LogServerStart logs server startup information
func (l *Logger) LogServerStart(protocol string, port int) {
	l.WithFields(logrus.Fields{
		"protocol": protocol,
		"port":     port,
	}).Infof("%s server running on %s://127.0.0.1:%d", 
		strings.ToUpper(protocol), strings.ToLower(protocol), port)
}

// Close closes the GeoIP database
func (l *Logger) Close() {
	if l.geoipDB != nil {
		l.geoipDB.Close()
	}
}