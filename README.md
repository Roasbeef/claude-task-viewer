# Claude Task Viewer

When you're running multiple Claude Code sessions across different projects,
keeping track of what's happening becomes surprisingly difficult. Which
sessions have active tasks? What's the status of that refactoring job you
kicked off an hour ago? Is that agent still working or did it finish?

Claude Task Viewer provides a unified dashboard for all your Claude Code
activity. It reads directly from Claude's local state (`~/.claude/`) and
presents everything in a single web interface—running instances, active
sessions, task lists, and dependency graphs.

## Quick Start

```bash
go install github.com/roasbeef/claude-task-viewer/cmd/taskviewerd@latest
taskviewerd --listen=:8080
```

Open http://localhost:8080.

## What You Get

**Running Instances** — See every Claude process currently running on your
machine. The dashboard detects Claude via process inspection and shows you
which directory each instance is working in, how long it's been running, and
whether it has active tasks.

**Project Overview** — Sessions are grouped by project (repository). Click
into any project to see its session history, including summaries, branches,
and timestamps. This data comes from Claude's `sessions-index.json` files.

**Task Board** — Active sessions with tasks get a Kanban-style view showing
pending, in-progress, and completed items. Dependencies between tasks are
visualized so you can see what's blocked on what.

**Dependency Graph** — For complex task trees, a force-directed D3.js graph
shows the full dependency structure. Useful when an agent has decomposed a
large problem into many subtasks.

## Building from Source

```bash
git clone https://github.com/roasbeef/claude-task-viewer
cd claude-task-viewer
go build ./cmd/taskviewerd
./taskviewerd --listen=:8080
```

Or use the Makefile:

```bash
make dev      # Build and start server
make restart  # Rebuild after changes
make stop     # Stop the server
```

## How It Works

Claude Code stores its state in `~/.claude/`:

- **Projects** live in `~/.claude/projects/{sanitized-path}/sessions-index.json`
  containing session metadata (summary, branch, message count, timestamps).

- **Tasks** appear in `~/.claude/tasks/{sessionID}/` as JSON files during
  active sessions. These are ephemeral—they're removed when sessions end.

The viewer reads this state directly. There's no separate database, no sync
process, no configuration pointing at directories. It just reads what Claude
writes.

Running instances are detected via `pgrep` and `lsof`. The dashboard polls
for updates every few seconds using HTMX, so you see changes without
refreshing.

## Architecture

The server is pure Go with embedded templates and static assets—a single
binary with no external dependencies at runtime. The UI uses HTMX for
server-rendered partials, avoiding the complexity of a JavaScript framework
while still feeling responsive.

```
cmd/taskviewerd/     Entry point
server.go            Lifecycle management
http.go              Routes, template functions
handlers.go          Route handlers
project.go           Project/session indexer
instance.go          Process detection
templates/           HTMX templates
static/              CSS, htmx.min.js, d3.min.js
```

## Configuration

```bash
taskviewerd --listen=:8080 --claude-dir=/path/to/.claude
```

| Flag | Default | Description |
|------|---------|-------------|
| `--listen` | `:8080` | HTTP listen address |
| `--claude-dir` | `~/.claude` | Claude state directory |

## Requirements

- Go 1.21+
- macOS or Linux (process detection uses `pgrep`/`lsof`)

## License

MIT
