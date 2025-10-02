package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Config represents the main configuration structure
type Config struct {
	Limit  LimitConfig    `toml:"limit"`
	Server []ServerConfig `toml:"server"`
}

// LimitConfig represents rate limiting configuration
type LimitConfig struct {
	Count  int `toml:"count"`  // Maximum requests per window
	Window int `toml:"window"` // Time window in seconds
}

// ServerConfig represents individual server configuration
type ServerConfig struct {
	Name      string      `toml:"name"`
	Port      int         `toml:"port"`
	TargetURL string      `toml:"target_url"`
	SecretKey string      `toml:"secret_key"`
	Expired   int         `toml:"expired"`   // Cookie expiration in seconds
	CtnMax    int         `toml:"ctn_max"`   // Maximum connections (0 = unlimited)
	HTTPS     HTTPSConfig `toml:"https"`
}

// HTTPSConfig represents HTTPS configuration
type HTTPSConfig struct {
	Enabled  bool   `toml:"enabled"`
	CertPath string `toml:"cert_path"`
	KeyPath  string `toml:"key_path"`
}

// LoadConfig loads configuration from the specified file
func LoadConfig(configPath string) (*Config, error) {
	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Try to copy from example file
		examplePath := configPath + ".example"
		if _, err := os.Stat(examplePath); err == nil {
			if err := copyFile(examplePath, configPath); err != nil {
				return nil, fmt.Errorf("failed to copy example config: %v", err)
			}
			return nil, fmt.Errorf("first time running. Please edit %s", configPath)
		}
		return nil, fmt.Errorf("configuration file %s does not exist and no example file found", configPath)
	}

	var cfg Config
	if _, err := toml.DecodeFile(configPath, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse TOML configuration: %v", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %v", err)
	}

	return &cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if len(c.Server) == 0 {
		return fmt.Errorf("no server configuration found")
	}

	for i, server := range c.Server {
		if server.Name == "" {
			return fmt.Errorf("server[%d]: name is required", i)
		}
		if server.Port <= 0 || server.Port > 65535 {
			return fmt.Errorf("server[%d]: invalid port number %d", i, server.Port)
		}
		if server.TargetURL == "" {
			return fmt.Errorf("server[%d]: target_url is required", i)
		}
		if server.SecretKey == "" {
			return fmt.Errorf("server[%d]: secret_key is required", i)
		}
		if server.Expired <= 0 {
			return fmt.Errorf("server[%d]: expired must be positive", i)
		}

		// Validate HTTPS configuration
		if server.HTTPS.Enabled {
			if server.HTTPS.CertPath == "" {
				return fmt.Errorf("server[%d]: HTTPS cert_path is required when HTTPS is enabled", i)
			}
			if server.HTTPS.KeyPath == "" {
				return fmt.Errorf("server[%d]: HTTPS key_path is required when HTTPS is enabled", i)
			}
			// Check if certificate files exist
			if _, err := os.Stat(server.HTTPS.CertPath); os.IsNotExist(err) {
				return fmt.Errorf("server[%d]: certificate file not found: %s", i, server.HTTPS.CertPath)
			}
			if _, err := os.Stat(server.HTTPS.KeyPath); os.IsNotExist(err) {
				return fmt.Errorf("server[%d]: key file not found: %s", i, server.HTTPS.KeyPath)
			}
		}
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0644)
}