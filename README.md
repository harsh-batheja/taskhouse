# Taskhouse - Task Management for Agentic Workflows

A lightweight task management server with a CLI client and built-in webhook support. Designed for multi-machine agent orchestration where tasks need to be created, tracked, and synced in real-time across VMs.

## Architecture

- **Server**: Single Go binary, SQLite backend, HTTP REST API
- **CLI**: `task` binary that talks to the server API
- **Webhooks**: Server dispatches HTTP POST on task create/update/delete

## API Endpoints

```
POST   /api/v1/tasks          # Create task
GET    /api/v1/tasks          # List tasks (filter by project, status, tags)
GET    /api/v1/tasks/:id      # Get task
PUT    /api/v1/tasks/:id      # Update task
DELETE /api/v1/tasks/:id      # Delete task
POST   /api/v1/tasks/:id/done # Mark done

POST   /api/v1/webhooks       # Register webhook
GET    /api/v1/webhooks       # List webhooks
DELETE /api/v1/webhooks/:id   # Remove webhook

GET    /api/v1/health         # Health check
```

## CLI Interface

```
task add "Deploy monitoring" project:obs +agent
task list [project:X] [+tag] [status:pending|done|all]
task done <id>
task modify <id> project:new-project
task info <id>
task sync                    # Pull latest from server (for offline-capable workflows)
task webhook add <url> --events create,update,delete
```

## Task Model

```json
{
  "id": 1,
  "uuid": "550e8400-e29b-41d4-a716-446655440000",
  "description": "Deploy monitoring stack",
  "project": "observability",
  "tags": ["agent", "infra"],
  "status": "pending",
  "priority": "M",
  "annotations": [{"entry": "2026-04-18T04:00:00Z", "description": "Started work"}],
  "entry": "2026-04-18T04:00:00Z",
  "modified": "2026-04-18T04:00:00Z",
  "due": null,
  "done": null,
  "urgency": 5.0
}
```

## Webhook Payload

```json
{
  "event": "create|update|delete|done",
  "task": { ... full task object ... },
  "timestamp": "2026-04-18T04:00:00Z"
}
```

## Configuration

Server config via env vars or config file:
```
TASKHOUSE_DB=./taskhouse.db
TASKHOUSE_PORT=8080
TASKHOUSE_AUTH_TOKEN=my-secret-token   # Bearer token auth
```

CLI config at `~/.config/taskhouse/config.toml`:
```toml
server = "http://192.168.10.127:8080"
token = "my-secret-token"
```

## Requirements

- Go 1.21+
- No external DB dependencies (SQLite via mattn/go-sqlite3 or modernc.org/sqlite)
- Single binary for server, single binary for CLI
- Webhooks dispatch asynchronously (don't block task operations)
- Retry webhook delivery up to 3 times with exponential backoff
- JSON output mode for CLI (`--json` flag) for agent consumption

## Project Structure

```
taskhouse/
├── cmd/
│   ├── server/main.go       # Server entrypoint
│   └── task/main.go         # CLI entrypoint
├── internal/
│   ├── server/              # HTTP server, routes
│   ├── store/               # SQLite store
│   ├── webhook/             # Webhook dispatcher
│   ├── model/               # Task model
│   └── cli/                 # CLI logic
├── go.mod
├── go.sum
├── Makefile
├── Dockerfile
└── README.md
```
