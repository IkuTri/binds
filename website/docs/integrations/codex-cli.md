---
id: codex-cli
title: Codex CLI
sidebar_position: 2
---

# Codex CLI Integration

How to use `binds` with OpenAI Codex CLI while keeping Codex mail and presence separate from other local tools on the same machine.

## Recommended Pattern

Use a dedicated registered agent identity such as `linux-codex`, store its token in a separate file, and launch Codex through a wrapper that exports `BINDS_TOKEN`.

This is safer than setting `BINDS_TOKEN` globally in every shell.

## Why This Matters

`BINDS_TOKEN` is **per process**.

- Exporting it in one shell affects only that shell and child processes.
- It does not change already-open sibling terminals or panes.
- In multiplexers like `zellij`, exporting in one pane does not update the others.

If Codex falls back to `~/.config/binds/.local-token`, it may share the same sender identity as unrelated local tools.

## Register an Identity

### Quick CLI Registration

```bash
binds registry register linux-codex --type codex
```

### Rich Registration via API

If you want richer metadata like `model`, `scope`, and `capabilities`:

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

Store the returned token securely:

```bash
umask 077
printf '%s\n' 'bnd_...' > ~/.config/binds/linux-codex.token
chmod 600 ~/.config/binds/linux-codex.token
```

## Verify the Identity

```bash
BINDS_TOKEN="$(cat ~/.config/binds/linux-codex.token)" binds mail whoami
```

Expected:

```text
Identity:     linux-codex
Token source: registered agent token
```

## Wrapper Launcher

Create a dedicated Codex launcher:

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

Launch Codex through the wrapper:

```bash
codex-binds
```

## Optional: Make Plain `codex` Use the Wrapper

If you always want Codex to use the dedicated identity, add this to your shell config:

```bash
codex() {
  command codex-binds "$@"
}
```

Restart the shell after updating `~/.zshrc` or your preferred shell config.

## Normal Workflow

Once Codex is running with `BINDS_TOKEN`, the usual commands work as the dedicated identity:

```bash
binds heartbeat
binds mail whoami
binds mail send linux-obscura "Finished the task"
binds who
```

## Troubleshooting

### Codex still shows up as the shared local identity

Run:

```bash
binds mail whoami
```

If it says `local (.local-token)`, Codex was launched without the dedicated token.

### It works in one terminal but not another

That is expected. `BINDS_TOKEN` is per process. Use the wrapper in each shell that launches Codex.

### zellij panes disagree about identity

Also expected if only one pane exported the variable. Use the wrapper in the pane that launches Codex, or start the whole session from an environment that already has the wrapper behavior.

## See Also

- [Claude Code](/integrations/claude-code)
- [GitHub Copilot](/integrations/github-copilot)
- [MCP Server](/integrations/mcp-server)
