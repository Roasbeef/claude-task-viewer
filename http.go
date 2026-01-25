package taskviewer

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/btcsuite/btclog/v2"
	claudeagent "github.com/roasbeef/claude-agent-sdk-go"
)

// Ensure time is used for template functions.
var _ = time.Now

//go:embed templates/*
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

// HTTPServer handles all HTTP requests for the task viewer.
type HTTPServer struct {
	cfg *HTTPConfig

	server         *http.Server
	listener       net.Listener
	taskStore      claudeagent.TaskStore
	projectIndexer *ProjectIndexer
	templates      *template.Template

	// sseClients tracks active SSE connections per list ID.
	sseClients   map[string][]chan []byte
	sseClientsMu sync.RWMutex

	started uint32
	stopped uint32
	quit    chan struct{}
	wg      sync.WaitGroup

	log btclog.Logger
}

// NewHTTPServer creates a new HTTP server component.
func NewHTTPServer(cfg *HTTPConfig, taskStore claudeagent.TaskStore,
	log btclog.Logger) (*HTTPServer, error) {

	// Parse embedded templates.
	tmpl, err := template.New("").Funcs(templateFuncs()).ParseFS(
		templatesFS, "templates/*.html", "templates/partials/*.html",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	// Create project indexer using same base dir as task store.
	projectIndexer := NewProjectIndexer(cfg.ClaudeDir)

	h := &HTTPServer{
		cfg:            cfg,
		taskStore:      taskStore,
		projectIndexer: projectIndexer,
		templates:      tmpl,
		sseClients:     make(map[string][]chan []byte),
		quit:           make(chan struct{}),
		log:            log,
	}

	return h, nil
}

// templateFuncs returns the custom template functions.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"statusClass": func(status claudeagent.TaskListStatus) string {
			switch status {
			case claudeagent.TaskListStatusPending:
				return "status-pending"

			case claudeagent.TaskListStatusInProgress:
				return "status-progress"

			case claudeagent.TaskListStatusCompleted:
				return "status-completed"

			default:
				return ""
			}
		},
		"statusIcon": func(status claudeagent.TaskListStatus) string {
			switch status {
			case claudeagent.TaskListStatusPending:
				return "○"

			case claudeagent.TaskListStatusInProgress:
				return "◐"

			case claudeagent.TaskListStatusCompleted:
				return "●"

			default:
				return "?"
			}
		},
		"isBlocked": func(task claudeagent.TaskListItem) bool {
			return len(task.BlockedBy) > 0
		},
		"formatTime": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.Local().Format("Jan 02, 2006 3:04 PM")
		},
		"truncate": func(s string, n int) string {
			if len(s) <= n {
				return s
			}
			return s[:n] + "..."
		},
		"truncateID": func(s string, n int) string {
			if len(s) <= n {
				return s
			}
			return s[:n] + "..."
		},
	}
}

// Start begins serving HTTP requests.
func (h *HTTPServer) Start() error {
	if !atomic.CompareAndSwapUint32(&h.started, 0, 1) {
		return nil
	}

	// Create listener.
	listener, err := net.Listen("tcp", h.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", h.cfg.ListenAddr, err)
	}
	h.listener = listener

	// Set up routes.
	mux := http.NewServeMux()
	h.registerRoutes(mux)

	h.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start serving.
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		if err := h.server.Serve(h.listener); err != http.ErrServerClosed {
			h.log.Errorf("HTTP server error: %v", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the HTTP server.
func (h *HTTPServer) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapUint32(&h.stopped, 0, 1) {
		return nil
	}

	close(h.quit)

	// Close all SSE clients.
	h.sseClientsMu.Lock()
	for _, clients := range h.sseClients {
		for _, ch := range clients {
			close(ch)
		}
	}
	h.sseClients = make(map[string][]chan []byte)
	h.sseClientsMu.Unlock()

	// Shutdown HTTP server.
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := h.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shutdown HTTP server: %w", err)
	}

	h.wg.Wait()

	return nil
}

// registerRoutes sets up all HTTP routes.
func (h *HTTPServer) registerRoutes(mux *http.ServeMux) {
	// Static files.
	staticSub, _ := fs.Sub(staticFS, "static")
	mux.Handle("GET /static/", http.StripPrefix(
		"/static/", http.FileServer(http.FS(staticSub)),
	))

	// Pages.
	mux.HandleFunc("GET /", h.handleIndex)
	mux.HandleFunc("GET /projects/{projectID}", h.handleProjectView)
	mux.HandleFunc("GET /lists/{listID}", h.handleListView)
	mux.HandleFunc("GET /lists/{listID}/tasks/{taskID}", h.handleTaskDetail)
	mux.HandleFunc("GET /lists/{listID}/graph", h.handleGraphView)

	// API endpoints.
	mux.HandleFunc("GET /api/lists/{listID}/graph", h.handleGraphData)
	mux.HandleFunc("GET /api/lists/{listID}/events", h.handleSSE)

	// HTMX partials.
	mux.HandleFunc(
		"GET /partials/task/{listID}/{taskID}", h.handleTaskPartial,
	)
	mux.HandleFunc("GET /partials/tasks/{listID}", h.handleTasksPartial)
}

// addSSEClient registers a new SSE client for a task list.
func (h *HTTPServer) addSSEClient(listID string, ch chan []byte) {
	h.sseClientsMu.Lock()
	defer h.sseClientsMu.Unlock()
	h.sseClients[listID] = append(h.sseClients[listID], ch)
}

// removeSSEClient unregisters an SSE client.
func (h *HTTPServer) removeSSEClient(listID string, ch chan []byte) {
	h.sseClientsMu.Lock()
	defer h.sseClientsMu.Unlock()

	clients := h.sseClients[listID]
	for i, c := range clients {
		if c == ch {
			h.sseClients[listID] = append(clients[:i], clients[i+1:]...)
			break
		}
	}
}

// broadcastSSE sends an event to all clients watching a task list.
func (h *HTTPServer) broadcastSSE(listID string, eventType, data string) {
	msg := []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, data))

	h.sseClientsMu.RLock()
	defer h.sseClientsMu.RUnlock()

	for _, ch := range h.sseClients[listID] {
		select {
		case ch <- msg:
		default:
			// Skip if buffer full.
		}
	}
}
