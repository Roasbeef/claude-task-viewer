package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/btcsuite/btclog/v2"
	flags "github.com/jessevdk/go-flags"
	taskviewer "github.com/roasbeef/claude-task-viewer"
)

func main() {
	// Parse configuration.
	cfg := taskviewer.DefaultConfig()
	parser := flags.NewParser(cfg, flags.Default)
	if _, err := parser.Parse(); err != nil {
		if flagsErr, ok := err.(*flags.Error); ok {
			if flagsErr.Type == flags.ErrHelp {
				os.Exit(0)
			}
		}
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// Set up logging.
	backend := btclog.NewDefaultHandler(os.Stdout)
	logger := btclog.NewSLogger(backend.SubSystem("TVWR"))

	// Parse log level.
	level, ok := btclog.LevelFromString(cfg.LogLevel)
	if !ok {
		fmt.Fprintf(os.Stderr, "Invalid log level: %s\n", cfg.LogLevel)
		os.Exit(1)
	}
	logger.SetLevel(level)

	log := logger

	// Create and start server.
	server, err := taskviewer.NewServer(cfg, log)
	if err != nil {
		log.Criticalf("Failed to create server: %v", err)
		os.Exit(1)
	}

	if err := server.Start(); err != nil {
		log.Criticalf("Failed to start server: %v", err)
		os.Exit(1)
	}

	// Wait for shutdown signal.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Infof("Received signal %v, shutting down...", sig)

	if err := server.Stop(); err != nil {
		log.Errorf("Error during shutdown: %v", err)
	}
}
