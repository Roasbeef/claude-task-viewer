# Claude Task Viewer

HTMX-based web application in Go to visualize Claude Code's Task system.

## Quick Start

```bash
# Build and start dev server
make dev

# After code changes
make restart

# Stop server
make stop
```

Server runs at http://localhost:8080

## Architecture

- **Server**: Pure Go HTTP server with embedded templates/static files (single binary)
- **UI**: HTMX for server-rendered partials, D3.js for dependency graph
- **Data**: Reads from `~/.claude/projects/` and `~/.claude/tasks/`
- **Logging**: btclog/v2 with `backend.SubSystem("TVWR")` pattern

## Key Files

| File | Purpose |
|------|---------|
| `cmd/taskviewerd/main.go` | Entry point |
| `server.go` | Server lifecycle (Start/Stop) |
| `http.go` | HTTP server, routes, template parsing |
| `handlers.go` | Route handlers |
| `project.go` | Project/session indexer |
| `config.go` | Configuration structs |
| `templates/*.html` | HTMX templates |
| `static/` | CSS, JS (htmx, d3, graph.js) |

## Data Sources

1. **Projects**: `~/.claude/projects/{sanitized-path}/sessions-index.json`
   - Contains session metadata (summary, branch, timestamps)
   - Only populated after sessions complete

2. **Tasks**: `~/.claude/tasks/{sessionID}/*.json`
   - Contains task files during active sessions
   - Ephemeral - removed when sessions end

## Routes

| Route | Description |
|-------|-------------|
| `/` | Dashboard with projects and active sessions |
| `/tasks` | All tasks across all active sessions |
| `/projects/{projectID}` | Project sessions |
| `/lists/{listID}` | Task list (Kanban board) |
| `/lists/{listID}/tasks/{taskID}` | Task detail |
| `/lists/{listID}/graph` | Dependency graph |

## Testing Flow After Adding Features

After implementing any UI feature, always verify it works:

1. **Rebuild and restart**:
   ```bash
   make restart
   ```

2. **Test the endpoint with curl** (quick sanity check):
   ```bash
   # Check page loads
   curl -s http://localhost:8080/tasks | head -50

   # Verify specific elements render
   curl -s http://localhost:8080/tasks | grep -c 'task-card'
   ```

3. **Visual verification** - Open http://localhost:8080 in browser and:
   - Click through all new UI elements
   - Verify links work
   - Check styling looks correct
   - Test on different data states (empty, one item, many items)

4. **Template gotchas to watch for**:
   - `{{$.Foo}}` in nested `{{range}}` refers to ROOT data, not parent
   - Use `{{range $item := .Items}}` then `{{$item.Field}}` for nested access
   - Template errors may render partial HTML - always check output

## Make Commands

```bash
make dev       # Build and start server
make restart   # Rebuild and restart after changes
make stop      # Stop server
make logs      # View running process
make fmt       # Format code
make test      # Run tests
```

## Dependencies

- `github.com/roasbeef/claude-agent-sdk-go` - Local replace directive
- `github.com/btcsuite/btclog/v2` - Logging

## Code Style

- 80 character line limit
- Tab = 8 spaces
- Complete sentence comments
- Function comments start with function name
