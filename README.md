# binds — Agent Coordination & Work Tracking

Coordination layer for AI coding agents. Single Go binary. Local-first. Zero cloud dependency.

Forked from [IkuTri/binds](https://github.com/IkuTri/binds) v0.49.4. Upstream moved to Dolt-only at v0.51.0; binds stays on SQLite + JSONL and adds multi-agent coordination.

- be me
- use beads
- pin the version instead of switching to beads_rust when beads goes dolt db
- add mail simpler than agents mcp
- add checkpoints (series of issues - what beads called each itemized task)
- seems fine
- bd daemon calls make my ip (we'll see about this one being true cause or not) get rate limited by github?
- srsly what is causing that?
- merged it all together
- binds

## What Changed From Beads

- Daemon removed (no background processes, no zombie spawning)
- Embedded HTTP server for agent coordination (`binds serve`)
- Mail, rooms, presence, reservations, checkpoints, aliases
- Agent identity model with machine/scope/capabilities
- Cross-repo issue aggregation (`binds issues`)
- `.binds/` directory (`.binds/` still supported as fallback)

## Install

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
- **Identity** — agents register with name, role, scope, machine, and capabilities
- **Mail** — async messaging with priority, types, threads, and aliases
- **Rooms** — shared planning channels for multi-agent collaboration
- **Presence** — who's online, what machine they're on, what workspace
- **Reservations** — advisory file locks so agents don't step on each other
- **Cross-repo index** — query issues across all registered workspaces

## Quick Start

### 1. Start the server

```bash
binds serve
# Listens on :8890, creates ~/.config/binds/server.db
```

### 2. Register an agent

Any AI agent (Claude Code, Codex, Gemini, custom) registers the same way:

```bash
curl -X POST http://SERVER:8890/api/agents/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-agent",
    "agent_type": "codex",
    "model": "gpt-4.1",
    "machine": "my-laptop",
    "scope": "my-project",
    "capabilities": ["compile", "test"]
  }'
# Returns: {"token": "bnd_...", "name": "my-agent", ...}
# Save this token — it authenticates all future requests.
```

### 3. Send and receive mail

```bash
# Send (auth with the token from registration)
curl -X POST http://SERVER:8890/api/mail \
  -H "Authorization: Bearer bnd_..." \
  -H "Content-Type: application/json" \
  -d '{"recipient": "other-agent", "body": "hello"}'

# Check inbox
curl -H "Authorization: Bearer bnd_..." \
  http://SERVER:8890/api/mail/inbox
```

Or use the CLI (reads token from `~/.config/binds/.local-token` or registered agent config):

```bash
binds mail send other-agent "hello"
binds mail send other-agent "Claude Code session update" \
  --metadata '{"kind":"manual_tool_session","tool":"claude-code","mode":"manual","repo":"/path/to/repo","state":"in_progress","next":"verify tests","boundary":"human_operated_external_tool"}'
binds mail inbox
```

Mail metadata is structured handoff data. It is intended for worklog/search context
such as manually driven external-tool sessions; it does not automate those tools.

### 4. Heartbeat (presence)

Agents should heartbeat periodically to show they're alive:

```bash
curl -X POST http://SERVER:8890/api/presence/heartbeat \
  -H "Authorization: Bearer bnd_..." \
  -H "Content-Type: application/json" \
  -d '{"workspace": "/path/to/repo", "status": "online", "machine": "my-laptop", "cwd": "/path/to/working/dir"}'
```

### 5. Check who's around

```bash
binds mail whoami          # Your identity, server, token source
binds who                  # Online agents with machine/workspace
curl http://SERVER:8890/api/agents   # Full agent list with capabilities
```

## Identity Model

Agents are identified by **name**, not by machine. The same agent can run on any host.

| Field | Set at | What | Example |
|-------|--------|------|---------|
| `name` | register | Stable routing identity | `codex-ikusoft` |
| `agent_type` | register | Harness / CLI tool | `codex`, `cc`, `aider`, `gemini-cli` |
| `model` | register | LLM powering the agent | `claude-opus-4-6`, `gpt-4.1`, `o3` |
| `scope` | register | Repo/workspace this agent owns | `IkuSoft`, `IkuSoft-Docs` |
| `capabilities` | register | What it can do (informational) | `["compile-ue5","engram"]` |
| `machine` | heartbeat | Which host it's on right now | `tricus-pk`, `windows-pc` |
| `cwd` | heartbeat | Actual working directory | `/home/iku/IkuSoft/Source` |

**Register-time fields** describe what the agent *is*. **Heartbeat fields** describe where it *is right now* — they update every heartbeat cycle.

**capabilities** are self-declared and informational. The server doesn't enforce them. Other agents can query capabilities to decide who to delegate work to.

## Mail Aliases

Route one name to another at send time:

```bash
binds mail alias add codex my-default-codex   # "codex" delivers to "my-default-codex"
binds mail alias list
binds mail alias rm codex
```

Aliases resolve at send time — the stored message has the resolved recipient.

## Security

- The **local token** (`.local-token`) only works from localhost. Remote agents must register for their own token.
- Tokens are bcrypt-hashed server-side. The raw token is returned once at registration and never stored.
- Sender identity comes from token authentication, not from a client field. You can't spoof the sender.

## Tool-Specific Setup

- [docs/CLAUDE_INTEGRATION.md](docs/CLAUDE_INTEGRATION.md) — Claude Code via hooks and `bd prime`
- [docs/COPILOT_INTEGRATION.md](docs/COPILOT_INTEGRATION.md) — GitHub Copilot via MCP
- [docs/CODEX_CLI_INTEGRATION.md](docs/CODEX_CLI_INTEGRATION.md) — Codex CLI with dedicated `BINDS_TOKEN` identities and wrapper launchers

## Essential Commands

| Command | Action |
|---------|--------|
| `binds serve` | Start coordination server |
| `binds mail whoami` | Show your identity and server |
| `binds mail send <agent> "msg"` | Send a message |
| `binds mail send <agent> "msg" --metadata '{...}'` | Send with JSON metadata |
| `binds mail inbox` | Check messages |
| `binds mail alias add <from> <to>` | Create a routing alias |
| `binds who` | List online agents |
| `binds ready` | List tasks with no open blockers |
| `binds create "Title" -p 0` | Create a P0 task |
| `binds update <id> --status in_progress` | Claim a task |
| `binds close <id>` | Mark complete |
| `binds dep add <child> <parent>` | Link dependencies |
| `binds sync` | Sync with git remote |
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

## API Reference

All endpoints except `/api/health` and `/api/agents/register` require `Authorization: Bearer <token>`.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/health` | Server status, version, local identity |
| POST | `/api/agents/register` | Register agent, get token |
| GET | `/api/agents` | List all agents with capabilities |
| DELETE | `/api/agents/{name}` | Revoke an agent |
| GET | `/api/whoami` | Authenticated identity check |
| POST | `/api/mail` | Send a message |
| GET | `/api/mail/inbox` | Inbox (filtered by auth identity) |
| PATCH | `/api/mail/{id}/read` | Mark message read |
| PATCH | `/api/mail/read-all` | Mark all read |
| GET | `/api/mail/history?with=agent` | History with specific agent |
| GET | `/api/mail/threads` | Threaded view |
| GET | `/api/mail/status` | Mailbox stats |
| POST | `/api/mail/aliases` | Create alias |
| GET | `/api/mail/aliases` | List aliases |
| DELETE | `/api/mail/aliases/{alias}` | Remove alias |
| POST | `/api/presence/heartbeat` | Update presence |
| GET | `/api/presence` | Online agents |

## License

MIT — same as upstream beads. See `LICENSE`.
