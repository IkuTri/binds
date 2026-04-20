# binds — Agent Coordination & Work Tracking

Modular, agent-agnostic coordination layer for AI coding agents. Local-first. Single binary.

Forked from [steveyegge/beads](https://github.com/steveyegge/beads) v0.49.4 (`3032c622`).
The upstream moved to Dolt-only at v0.51.0; this fork stays on SQLite + JSONL and evolves
toward multi-agent coordination primitives.

## Install

```bash
go install github.com/ikutri/binds/cmd/binds@latest
```

Or build from source:

```bash
cd ~/Soft-Serve/beads
go build -o binds ./cmd/binds/
```

## Features

- **Issue tracking** — dependency-aware graph, hash-based IDs, audit trail
- **Git-backed** — issues stored as JSONL in `.beads/`, versioned like code
- **Daemon mode** — background SQLite cache, auto-sync, auto-import
- **Hooks** — extensible event hooks for agent integration
- **Checkpoints** — snapshot and restore issue state
- **Mail** — inter-agent messaging (Phase 1, in development)
- **Rooms & Presence** — agent coordination primitives (Phase 1, in development)

## Quick Start

```bash
cd your-project
binds init
binds create "First task" -p 1
binds ready
```

## Essential Commands

| Command | Action |
| --- | --- |
| `binds ready` | List tasks with no open blockers |
| `binds create "Title" -p 0` | Create a P0 task |
| `binds update <id> --claim` | Atomically claim a task |
| `binds dep add <child> <parent>` | Link tasks |
| `binds show <id>` | View task details and audit trail |
| `binds version` | Show version |

## Phase 1: Coordination (In Development)

Phase 1 adds primitives for coordinating multiple agents on shared work:
rooms (shared context channels), presence (who is working on what), and
inter-agent mail. These build on the existing SQLite backend without
requiring a network service.

## License

MIT — same as upstream beads. See `LICENSE`.
