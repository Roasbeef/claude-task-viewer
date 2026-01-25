package taskviewer

import (
	"encoding/json"
	"fmt"
	"net/http"

	claudeagent "github.com/roasbeef/claude-agent-sdk-go"
)

// PageData is the base data structure passed to templates.
type PageData struct {
	Title   string
	ListID  string
	TaskID  string
	Error   string
	Message string
}

// IndexData holds data for the index page.
type IndexData struct {
	PageData
	Groups      []ProjectGroup
	ActiveLists []ActiveTaskList
}

// ListSummary summarizes a task list.
type ListSummary struct {
	ID         string
	TaskCount  int
	Pending    int
	InProgress int
	Completed  int
}

// ProjectViewData holds data for the project view.
type ProjectViewData struct {
	PageData
	Project  Project
	Sessions []SessionViewEntry
}

// SessionViewEntry extends SessionEntry with task info.
type SessionViewEntry struct {
	SessionEntry
	TaskCount int
	HasTasks  bool
}

// ListViewData holds data for the task list view.
type ListViewData struct {
	PageData
	Tasks           []claudeagent.TaskListItem
	Filter          string
	TotalCount      int
	PendingCount    int
	InProgressCount int
	CompletedCount  int
}

// TaskDetailData holds data for the task detail view.
type TaskDetailData struct {
	PageData
	Task     *claudeagent.TaskListItem
	Blockers []claudeagent.TaskListItem
	Blocking []claudeagent.TaskListItem
}

// GraphData holds data for the dependency graph API.
type GraphData struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// GraphNode represents a task in the graph.
type GraphNode struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Status      string `json:"status"`
	IsBlocked   bool   `json:"isBlocked"`
	Description string `json:"description,omitempty"`
}

// GraphEdge represents a dependency relationship.
type GraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// handleIndex renders the dashboard with all projects.
func (h *HTTPServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	// Get project groups (projects grouped by base repo).
	groups, err := h.projectIndexer.ListProjectGroups()
	if err != nil {
		h.renderError(
			w, "Failed to list projects: "+err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	// Get active task lists (sessions with actual task files).
	activeLists, _ := h.projectIndexer.ListActiveTaskLists()

	data := IndexData{
		PageData:    PageData{Title: "Task Viewer"},
		Groups:      groups,
		ActiveLists: activeLists,
	}

	h.render(w, "index.html", data)
}

// handleProjectView renders a project with its sessions.
func (h *HTTPServer) handleProjectView(w http.ResponseWriter, r *http.Request) {
	dirName := r.PathValue("projectID")

	project, err := h.projectIndexer.GetProject(dirName)
	if err != nil {
		h.renderError(w, "Project not found: "+err.Error(), http.StatusNotFound)
		return
	}

	// Build session view entries with task counts.
	sessions := make([]SessionViewEntry, len(project.Sessions))
	for i, s := range project.Sessions {
		taskCount := h.projectIndexer.GetTaskCount(s.SessionID)
		sessions[i] = SessionViewEntry{
			SessionEntry: s,
			TaskCount:    taskCount,
			HasTasks:     taskCount > 0,
		}
	}

	data := ProjectViewData{
		PageData: PageData{
			Title:  project.Name,
			ListID: dirName,
		},
		Project:  project,
		Sessions: sessions,
	}

	h.render(w, "project.html", data)
}

// handleListView renders a task list.
func (h *HTTPServer) handleListView(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	listID := r.PathValue("listID")
	filter := r.URL.Query().Get("filter")

	tasks, err := h.taskStore.List(ctx, listID)
	if err != nil {
		h.renderError(w, "Failed to load tasks: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Count tasks by status.
	var pendingCount, inProgressCount, completedCount int
	for _, t := range tasks {
		switch t.Status {
		case "pending":
			pendingCount++
		case "in_progress":
			inProgressCount++
		case "completed":
			completedCount++
		}
	}

	// Apply filter.
	filtered := tasks
	if filter != "" {
		filtered = nil
		for _, t := range tasks {
			if string(t.Status) == filter {
				filtered = append(filtered, t)
			}
		}
	}

	data := ListViewData{
		PageData:        PageData{Title: "Tasks - " + listID, ListID: listID},
		Tasks:           filtered,
		Filter:          filter,
		TotalCount:      len(tasks),
		PendingCount:    pendingCount,
		InProgressCount: inProgressCount,
		CompletedCount:  completedCount,
	}

	h.render(w, "tasks.html", data)
}

// handleTaskDetail renders a single task.
func (h *HTTPServer) handleTaskDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	listID := r.PathValue("listID")
	taskID := r.PathValue("taskID")

	task, err := h.taskStore.Get(ctx, listID, taskID)
	if err != nil {
		h.renderError(w, "Task not found: "+err.Error(), http.StatusNotFound)
		return
	}

	// Load blocker and blocking tasks.
	var blockers, blocking []claudeagent.TaskListItem
	allTasks, _ := h.taskStore.List(ctx, listID)

	taskMap := make(map[string]claudeagent.TaskListItem)
	for _, t := range allTasks {
		taskMap[t.ID] = t
	}

	for _, id := range task.BlockedBy {
		if t, ok := taskMap[id]; ok {
			blockers = append(blockers, t)
		}
	}
	for _, id := range task.Blocks {
		if t, ok := taskMap[id]; ok {
			blocking = append(blocking, t)
		}
	}

	data := TaskDetailData{
		PageData: PageData{
			Title:  task.Subject,
			ListID: listID,
			TaskID: taskID,
		},
		Task:     task,
		Blockers: blockers,
		Blocking: blocking,
	}

	h.render(w, "task_detail.html", data)
}

// handleGraphView renders the dependency graph page.
func (h *HTTPServer) handleGraphView(w http.ResponseWriter, r *http.Request) {
	listID := r.PathValue("listID")

	data := PageData{
		Title:  "Dependency Graph - " + listID,
		ListID: listID,
	}

	h.render(w, "graph.html", data)
}

// handleGraphData returns JSON data for the dependency graph.
func (h *HTTPServer) handleGraphData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	listID := r.PathValue("listID")

	tasks, err := h.taskStore.List(ctx, listID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	graph := GraphData{
		Nodes: make([]GraphNode, 0, len(tasks)),
		Edges: make([]GraphEdge, 0),
	}

	for _, t := range tasks {
		node := GraphNode{
			ID:          t.ID,
			Label:       t.Subject,
			Status:      string(t.Status),
			IsBlocked:   len(t.BlockedBy) > 0,
			Description: t.Description,
		}
		graph.Nodes = append(graph.Nodes, node)

		// Add edges for blockedBy relationships.
		for _, blockerID := range t.BlockedBy {
			graph.Edges = append(graph.Edges, GraphEdge{
				Source: blockerID,
				Target: t.ID,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(graph)
}

// handleSSE streams task events via Server-Sent Events.
func (h *HTTPServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	listID := r.PathValue("listID")

	// Set SSE headers.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Subscribe to task events.
	events, err := h.taskStore.Subscribe(ctx, listID)
	if err != nil {
		http.Error(w, "Failed to subscribe: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Send initial ping.
	fmt.Fprintf(w, "event: ping\ndata: connected\n\n")
	flusher.Flush()

	// Stream events.
	for {
		select {
		case <-ctx.Done():
			return

		case <-h.quit:
			return

		case event, ok := <-events:
			if !ok {
				return
			}

			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "event: task-%s\ndata: %s\n\n", event.Type, data)
			flusher.Flush()
		}
	}
}

// handleTaskPartial renders a single task row for HTMX updates.
func (h *HTTPServer) handleTaskPartial(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	listID := r.PathValue("listID")
	taskID := r.PathValue("taskID")

	task, err := h.taskStore.Get(ctx, listID, taskID)
	if err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	h.render(w, "task_row.html", task)
}

// handleTasksPartial renders the task list for HTMX updates.
func (h *HTTPServer) handleTasksPartial(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	listID := r.PathValue("listID")
	filter := r.URL.Query().Get("filter")

	tasks, err := h.taskStore.List(ctx, listID)
	if err != nil {
		http.Error(w, "Failed to load tasks", http.StatusInternalServerError)
		return
	}

	// Apply filter.
	filtered := tasks
	if filter != "" {
		filtered = nil
		for _, t := range tasks {
			if string(t.Status) == filter {
				filtered = append(filtered, t)
			}
		}
	}

	data := struct {
		Tasks  []claudeagent.TaskListItem
		ListID string
	}{
		Tasks:  filtered,
		ListID: listID,
	}

	h.render(w, "tasks_list.html", data)
}

// render executes a template and writes the result.
func (h *HTTPServer) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		h.log.Errorf("Template error (%s): %v", name, err)
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

// renderError renders an error page.
func (h *HTTPServer) renderError(w http.ResponseWriter, msg string, code int) {
	w.WriteHeader(code)
	data := PageData{
		Title: "Error",
		Error: msg,
	}
	h.render(w, "error.html", data)
}
