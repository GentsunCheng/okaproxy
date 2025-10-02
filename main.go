package main

import (
	"flag"
	"log"
	"os"

	"okaproxy/internal/config"
	"okaproxy/internal/server"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "config.toml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize and start servers
	serverManager := server.NewManager(cfg)
	if err := serverManager.Start(); err != nil {
		log.Fatalf("Failed to start servers: %v", err)
		os.Exit(1)
	}

	// Wait for shutdown signal
	serverManager.WaitForShutdown()
}