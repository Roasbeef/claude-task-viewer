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
| `/` | Project list |
| `/projects/{projectID}` | Project sessions |
| `/lists/{listID}` | Task list |
| `/lists/{listID}/tasks/{taskID}` | Task detail |
| `/lists/{listID}/graph` | Dependency graph |

## Development

```bash
# Rebuild and restart
make restart

# View running process
make logs

# Format code
make fmt

# Run tests
make test
```

## Dependencies

- `github.com/roasbeef/claude-agent-sdk-go` - Local replace directive
- `github.com/btcsuite/btclog/v2` - Logging

## Code Style

- 80 character line limit
- Tab = 8 spaces
- Complete sentence comments
- Function comments start with function name
