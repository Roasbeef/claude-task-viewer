package taskviewer

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/btcsuite/btclog/v2"
	claudeagent "github.com/roasbeef/claude-agent-sdk-go"
)

// Server is the main task viewer daemon that orchestrates all components.
type Server struct {
	cfg *Config

	httpServer *HTTPServer
	taskStore  claudeagent.TaskStore

	started uint32
	stopped uint32
	quit    chan struct{}
	wg      sync.WaitGroup

	log btclog.Logger
}

// NewServer creates a new task viewer server with the given configuration.
func NewServer(cfg *Config, log btclog.Logger) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Resolve claude base directory and tasks directory.
	claudeDir, err := cfg.ResolveClaudeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve claude dir: %w", err)
	}

	tasksDir, err := cfg.ResolveTasksDir()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve tasks dir: %w", err)
	}

	// Create file task store.
	taskStore, err := claudeagent.NewFileTaskStore(tasksDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create task store: %w", err)
	}

	// Create HTTP server.
	httpCfg := &HTTPConfig{
		ListenAddr: cfg.ListenAddr,
		ClaudeDir:  claudeDir,
		TasksDir:   tasksDir,
		DebugHTTP:  cfg.DebugHTTP,
	}
	httpServer, err := NewHTTPServer(httpCfg, taskStore, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP server: %w", err)
	}

	return &Server{
		cfg:        cfg,
		httpServer: httpServer,
		taskStore:  taskStore,
		quit:       make(chan struct{}),
		log:        log,
	}, nil
}

// Start launches all server components. This method is idempotent.
func (s *Server) Start() error {
	if !atomic.CompareAndSwapUint32(&s.started, 0, 1) {
		return nil
	}

	s.log.Info("Starting task viewer server")

	// Start HTTP server.
	if err := s.httpServer.Start(); err != nil {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	s.log.Infof("Task viewer listening on %s", s.cfg.ListenAddr)

	return nil
}

// Stop gracefully shuts down all server components. This method is idempotent.
func (s *Server) Stop() error {
	if !atomic.CompareAndSwapUint32(&s.stopped, 0, 1) {
		return nil
	}

	s.log.Info("Stopping task viewer server")

	close(s.quit)

	// Stop HTTP server.
	if err := s.httpServer.Stop(context.Background()); err != nil {
		s.log.Errorf("Error stopping HTTP server: %v", err)
	}

	s.wg.Wait()

	s.log.Info("Task viewer server stopped")

	return nil
}

// TaskStore returns the underlying task store for testing.
func (s *Server) TaskStore() claudeagent.TaskStore {
	return s.taskStore
}
