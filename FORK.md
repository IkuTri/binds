# binds — Fork of Beads

This is a fork of [steveyegge/beads](https://github.com/steveyegge/beads) pinned at commit
`3032c622` (upstream release `v0.49.4`, 2026-02-05), repurposed as **binds** — an agent
coordination and work tracking tool.

## Why fork

Upstream migrated to Dolt-only backend at `v0.51.0`, removing SQLite entirely. The IkuTri/Obscura
workflow runs 16 SQLite beads databases across separate repos. Beyond the backend preference,
we want to evolve the tool toward multi-agent coordination primitives (rooms, presence, mail)
that upstream has no plans for.

This fork becomes our upstream. We pull in upstream fixes only if they apply cleanly and are
worth the review cost.

## Project direction

**binds** keeps the beads core (SQLite + JSONL, dependency graph, hooks, daemon) and adds:

- **Phase 0 (done):** Rebrand to `binds`, reset version to `0.1.0`
- **Phase 1 (in development):** Agent coordination primitives — rooms, presence, inter-agent mail
- **Phase 2 (planned):** Checkpoint streaming, agent-to-agent task handoff

## Branch layout

- **main** — primary development branch for binds
- **feat/binds-phase-0** — Phase 0 rebrand work
- **feat/binds-phase-1** — Phase 1 coordination primitives

## Upstream sync policy

```
git fetch upstream
git log upstream/main --not main --oneline    # see what's new upstream
# cherry-pick only if a specific fix matters; do not rebase
```

Upstream remote: `https://github.com/steveyegge/beads`

## Known divergences from upstream

- Binary renamed from `bd` to `binds`
- Version reset to `0.1.0` (fork baseline)
- Root command `Use` field updated to `binds`
- Upstream Dolt migration path not followed (SQLite retained)

## Install

```bash
cd ~/Soft-Serve/beads
go build -o binds ./cmd/binds/
cp binds ~/.local/bin/binds
```

## License

Original beads is MIT-licensed. This fork retains the MIT license. See `LICENSE`.
