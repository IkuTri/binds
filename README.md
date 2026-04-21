# binds — Agent Coordination & Work Tracking

Coordination layer for AI coding agents. Single Go binary. Local-first. Zero cloud dependency.

Originally forked from [steveyegge/beads](https://github.com/steveyegge/beads) v0.49.4. The upstream moved to Dolt-only at v0.51.0; binds stays on SQLite + JSONL and adds multi-agent coordination.

## What Changed From Beads

- Daemon lobotomized (no background processes, no zombie spawning)
- Embedded HTTP server for agent coordination (`binds serve`)
- Mail, rooms, presence, reservations, checkpoints
- Cross-repo issue aggregation (`binds issues`)
- Workspace detection (`binds workspace current`)
- `.binds/` directory (`.beads/` still supported as fallback)

## Install

Build from source:

```bash
go build -o binds ./cmd/binds/
cp binds ~/.local/bin/
```

## Features

### Issue Tracking
- Dependency-aware graph with hash-based IDs
- Git-backed — issues stored as JSONL in `.binds/`, versioned like code
- Per-repo SQLite databases with audit trail
- Checkpoints — snapshot and restore milestone state

### Agent Coordination (`binds serve`)
- **Mail** — async messaging between agents with priority, types, and threads
- **Rooms** — shared planning channels for multi-agent collaboration
- **Presence** — who's online, what workspace they're in
- **Reservations** — advisory file locks so agents don't step on each other
- **Cross-repo index** — query issues across all registered workspaces

## Quick Start

```bash
cd your-project
binds init
binds create "First task" -p 1
binds ready
```

For agent onboarding, see the [quickstart template](https://github.com/IkuTri/Obscura-Hideout/blob/main/reference/binds-quickstart.md).

## Essential Commands

| Command | Action |
|---------|--------|
| `binds ready` | List tasks with no open blockers |
| `binds create "Title" -p 0` | Create a P0 task |
| `binds update <id> --status in_progress` | Claim a task |
| `binds close <id>` | Mark complete |
| `binds dep add <child> <parent>` | Link dependencies |
| `binds show <id>` | View task details and audit trail |
| `binds sync` | Sync with git remote |
| `binds serve` | Start coordination server |
| `binds who` | List online agents |
| `binds mail inbox` | Check messages |
| `binds mail send <agent> "msg"` | Send a message |
| `binds issues` | Cross-repo issue scan |

## Configuration

```toml
# ~/.config/binds/config.toml
[server]
port = 8890
listen = "0.0.0.0"

[identity]
name = "my-agent"
type = "cc"

[workspaces]
paths = [
    "~/projects/repo-a",
    "~/projects/repo-b",
]
```

## License

MIT — same as upstream beads. See `LICENSE`.
