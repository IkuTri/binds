# IkuTri Fork of Beads

This is a frozen fork of [steveyegge/beads](https://github.com/steveyegge/beads) pinned at commit `3032c622` (upstream release `v0.49.4`, 2026-02-05).

## Why fork

Upstream migrated to Dolt-only backend starting at `v0.51.0` — SQLite was removed entirely. The IkuTri/Obscura workflow runs 16 SQLite beads databases across separate repos with hook-driven session-start/compaction integration. Migrating all of them to Dolt is a larger lift than we need, and the SQLite + JSONL design at 0.49.4 is the architecture we want long-term.

Rather than track an upstream that's diverged from our needs, this fork becomes our upstream. We pull in upstream fixes only if they apply cleanly and are worth the review cost.

## Branch layout

- **main** — tracks upstream `steveyegge/beads:main` for reference. Do not develop on this branch.
- **ikutri-pin** — our working branch. Starts at the v0.49.4 commit. All IkuTri divergences land here.

## Upstream sync policy

```
git fetch upstream
git log upstream/main --not ikutri-pin --oneline    # see what's new upstream
# cherry-pick only if a specific fix matters; do not rebase
```

## Planned divergences

None critical yet. Candidates for future work (none of these are active TODOs):

- **Daemon-clobber race fix** — `bd export -o <file>` and direct JSONL writes can be clobbered by auto-spawned sync daemons. Seen in production.
- **`bd doctor --fix` non-interactive mode** — currently blocks in `$EDITOR`-requiring prompts; add `--yes` flag.
- **`bd export` staleness signal** — occasionally reports "exported 0" when JSONL is genuinely stale.
- **Engram integration hooks** (speculative) — on-close event hook that writes a learning/win capture to `~/.claude/engram/memory/index.db`. Scoped in a separate design spike (see Obscura-Hideout bead for tracking).

## Install

```bash
cd ~/Soft-Serve/beads
git checkout ikutri-pin
go install ./cmd/bd
# Installed binary lands in $GOBIN (typically ~/go/bin). Ensure ~/.local/bin/bd
# takes precedence, or replace the binary there directly.
```

## License

Original beads is MIT-licensed. This fork retains the MIT license from upstream. See `LICENSE`.
