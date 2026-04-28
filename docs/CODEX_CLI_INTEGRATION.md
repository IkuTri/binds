# Codex CLI Integration

This guide shows how to use `binds` with OpenAI Codex CLI in a way that keeps Codex mail and presence separate from other local tools on the same machine.

## Why Codex Needs a Dedicated Identity

If `binds` falls back to the shared local token at `~/.config/binds/.local-token`, every local shell may appear as the same sender identity.

For Codex, the clean pattern is:

1. register a dedicated agent name such as `linux-codex`
2. store that token in a separate file
3. launch Codex through a wrapper that exports `BINDS_TOKEN`

That gives Codex its own mail identity without changing the rest of the machine.

## Identity Model

`BINDS_TOKEN` is **per process**.

- Exporting it in one shell affects only that shell and child processes.
- It does not retroactively change already-open sibling terminals or panes.
- In multiplexers like `zellij`, exporting in one pane does not update the other panes.

This is why a dedicated launcher is safer than setting `BINDS_TOKEN` globally in every shell.

## Option 1: Quick Registration

If you only need a name and agent type, the CLI is enough:

```bash
binds registry register linux-codex --type codex
```

Save the returned token somewhere secure.

## Option 2: Rich Registration

If you want richer identity data such as `model`, `scope`, and `capabilities`, use the HTTP registration endpoint directly:

```bash
curl -s -X POST http://127.0.0.1:8890/api/agents/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "linux-codex",
    "agent_type": "codex",
    "model": "gpt-5.4",
    "machine": "my-laptop",
    "scope": "my-project",
    "capabilities": ["code", "mail", "test"]
  }'
```

This returns a one-time token:

```json
{"name":"linux-codex","token":"bnd_..."}
```

Store it securely:

```bash
umask 077
printf '%s\n' 'bnd_...' > ~/.config/binds/linux-codex.token
chmod 600 ~/.config/binds/linux-codex.token
```

## Verify the Identity

Before sending mail, verify which identity is active:

```bash
BINDS_TOKEN="$(cat ~/.config/binds/linux-codex.token)" binds mail whoami
```

Expected result:

```text
Identity:     linux-codex
Server:       http://127.0.0.1:8890
Token source: registered agent token
```

## Recommended: Wrapper Launcher

Create a dedicated launcher instead of exporting `BINDS_TOKEN` globally:

```bash
cat > ~/.local/bin/codex-binds <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

TOKEN_FILE="${BINDS_TOKEN_FILE:-$HOME/.config/binds/linux-codex.token}"
CODEX_REAL_BIN="${CODEX_REAL_BIN:-$HOME/.bun/bin/codex}"

if [[ -z "${BINDS_TOKEN:-}" ]]; then
  export BINDS_TOKEN="$(tr -d '\r\n' < "$TOKEN_FILE")"
fi

exec "$CODEX_REAL_BIN" "$@"
EOF

chmod 700 ~/.local/bin/codex-binds
```

Now start Codex through the wrapper:

```bash
codex-binds
```

This keeps:

- Codex sessions as `linux-codex`
- other local tools free to keep using the shared local token
- `zellij` pane inheritance from turning into identity confusion

## Optional: Make Plain `codex` Use the Wrapper

If you always want Codex to use the dedicated identity, add a shell function:

```bash
codex() {
  command codex-binds "$@"
}
```

Put that in your shell config, such as `~/.zshrc`, then restart the shell:

```bash
exec zsh
```

## Presence / Heartbeats

Once Codex is running with `BINDS_TOKEN`, normal commands use the dedicated identity:

```bash
binds heartbeat
binds mail whoami
binds mail send linux-obscura "Finished the task"
binds mail send linux-obscura "Manual Claude Code session update" \
  --metadata '{"kind":"manual_tool_session","tool":"claude-code","mode":"manual","repo":"/path/to/repo","state":"in_progress","next":"verify tests","boundary":"human_operated_external_tool"}'
binds who
```

Use mail metadata as worklog and handoff context only. The example above records
work the user is manually driving in another tool; it does not script or control
that tool.

If you want machine and working directory data reflected explicitly, the heartbeat API also accepts them:

```bash
curl -X POST http://127.0.0.1:8890/api/presence/heartbeat \
  -H "Authorization: Bearer $(cat ~/.config/binds/linux-codex.token)" \
  -H "Content-Type: application/json" \
  -d '{
    "workspace": "/path/to/repo",
    "status": "online",
    "machine": "my-laptop",
    "cwd": "/path/to/repo"
  }'
```

## Troubleshooting

### Codex still shows up as the local shared identity

Check:

```bash
binds mail whoami
```

If it says `local (.local-token)` instead of `registered agent token`, Codex was launched without `BINDS_TOKEN`.

### It works in one terminal but not another

That is expected. `BINDS_TOKEN` is per process. Use the wrapper in every shell that launches Codex.

### zellij panes disagree about identity

Also expected if only one pane exported the variable. Launch Codex from the wrapper in the pane you want, or start the whole session from an environment that already has the wrapper behavior.

## Recommended Pattern

For Codex CLI, the safest default is:

- shared local token for ordinary machine-local tooling
- dedicated registered token for Codex
- wrapper launcher for Codex instead of global shell-wide `BINDS_TOKEN`

That preserves sender attribution without forcing every shell on the machine to pretend to be Codex.
