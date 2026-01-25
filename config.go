package taskviewer

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the main configuration for the task viewer daemon.
type Config struct {
	// ListenAddr is the address the HTTP server will listen on.
	ListenAddr string `long:"listen" description:"Address to listen on" default:":8080"`

	// TasksDir is the directory containing task lists. Defaults to
	// ~/.claude/tasks if empty.
	TasksDir string `long:"tasks-dir" description:"Task storage directory"`

	// LogLevel sets the logging verbosity.
	LogLevel string `long:"loglevel" description:"Log level (trace, debug, info, warn, error, critical)" default:"info"`

	// DebugHTTP enables HTTP request/response logging.
	DebugHTTP bool `long:"debug-http" description:"Enable HTTP debug logging"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		ListenAddr: ":8080",
		LogLevel:   "info",
	}
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	if c.ListenAddr == "" {
		return fmt.Errorf("listen address cannot be empty")
	}

	return nil
}

// ResolveTasksDir returns the tasks directory, defaulting to ~/.claude/tasks.
func (c *Config) ResolveTasksDir() (string, error) {
	if c.TasksDir != "" {
		return c.TasksDir, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(home, ".claude", "tasks"), nil
}

// HTTPConfig holds configuration for the HTTP server component.
type HTTPConfig struct {
	// ListenAddr is the address to listen on.
	ListenAddr string

	// ClaudeDir is the base ~/.claude directory.
	ClaudeDir string

	// TasksDir is the resolved path to the tasks directory.
	TasksDir string

	// DebugHTTP enables request/response logging.
	DebugHTTP bool
}

// ResolveClaudeDir returns the base claude directory, defaulting to ~/.claude.
func (c *Config) ResolveClaudeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(home, ".claude"), nil
}
