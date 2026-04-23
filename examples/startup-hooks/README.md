# Startup Hooks for AI Agents

This directory contains startup hook scripts that help AI agents automatically detect and adapt to changes in their environment.

## bd-version-check.sh

**Purpose:** Automatically detect bd (beads) upgrades and show what changed

**Features:**
- ✅ Detects when bd version changes between sessions
- ✅ Shows `bd info --whats-new` output automatically
- ✅ Auto-updates outdated git hooks
- ✅ Persists version tracking in `.binds/metadata.json`
- ✅ Works today - no bd code changes required!

**Usage:**

```bash
# Source the script at session start (recommended)
source examples/startup-hooks/bd-version-check.sh

# Or execute it directly
bash examples/startup-hooks/bd-version-check.sh
```

### Integration Examples

#### Claude Code

If Claude Code supports startup hooks:
```bash
# Add to .claude/hooks/session-start
source examples/startup-hooks/bd-version-check.sh
```

Alternatively, manually run at the start of each coding session.

#### GitHub Copilot

Add to your shell initialization file:
```bash
# ~/.bashrc or ~/.zshrc
# Run bd version check when entering a beads project
if [ -d ".binds" ]; then
  source /path/to/beads/examples/startup-hooks/bd-version-check.sh
fi
```

#### Cursor

Add to workspace settings or your shell init file following the same pattern as GitHub Copilot.

#### Generic Integration

Any AI coding environment that allows custom startup scripts can source this file.

### Requirements

- **bd (beads)**: Must be installed and in PATH
- **jq**: Required for JSON manipulation (`brew install jq` on macOS, `apt-get install jq` on Ubuntu)
- **.binds directory**: Must exist in current project

### How It Works

1. **Version Detection**: Reads current bd version and compares to `.binds/metadata.json`
2. **Change Notification**: If version changed, displays upgrade banner with what's new
3. **Hook Updates**: Checks for outdated git hooks and auto-updates them
4. **Persistence**: Updates `metadata.json` with current version for next session

### Example Output

```
🔄 bd upgraded: 0.23.0 → 0.24.2

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🆕 What's New in bd (Current: v0.24.2)
=============================================================

## v0.24.2 (2025-11-23)
  • New feature X
  • Bug fix Y
  • Performance improvement Z

[... rest of what's new output ...]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Review changes above and adapt your workflow accordingly

🔧 Git hooks outdated. Updating to match bd v0.24.2...
✓ Git hooks updated successfully
```

### Edge Cases Handled

- **Not in a beads project**: Silently exits (safe to include in global shell init)
- **bd not installed**: Silently exits
- **jq not installed**: Shows warning but doesn't break
- **metadata.json missing**: Auto-creates it
- **First run**: Sets version without showing upgrade message
- **bd command fails**: Silently exits

### Troubleshooting

**Q: Script doesn't detect version change**
A: Check that `.binds/metadata.json` exists and contains `last_bd_version` field

**Q: "jq not found" warning**
A: Install jq: `brew install jq` (macOS) or `apt-get install jq` (Ubuntu)

**Q: Git hooks not auto-updating**
A: Ensure you have write permissions to `.git/hooks/` directory

### Related

- **GitHub Discussion #239**: "Upgrading beads: how to let the Agent know"
- **Parent Epic**: bd-nxgk - Agent upgrade awareness system
- **AGENTS.md**: See "After Upgrading bd" section for manual workflow
