package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/beads"
	internalbeads "github.com/steveyegge/beads/internal/beads"
	"github.com/steveyegge/beads/internal/config"
	"github.com/steveyegge/beads/internal/rpc"
	"github.com/steveyegge/beads/internal/syncbranch"
)

// isDaemonAutoSyncing checks if daemon is running with auto-commit and auto-push enabled.
// Returns false if daemon is not running or check fails (fail-safe to show full protocol).
// This is a variable to allow stubbing in tests.
var isDaemonAutoSyncing = func() bool {
	beadsDir := beads.FindBeadsDir()
	if beadsDir == "" {
		return false
	}

	socketPath := filepath.Join(beadsDir, "bd.sock")
	client, err := rpc.TryConnect(socketPath)
	if err != nil || client == nil {
		return false
	}
	defer func() { _ = client.Close() }()

	status, err := client.Status()
	if err != nil {
		return false
	}

	// Only check auto-commit and auto-push (auto-pull is separate)
	return status.AutoCommit && status.AutoPush
}

var (
	primeFullMode    bool
	primeMCPMode     bool
	primeStealthMode bool
	primeExportMode  bool
)

var primeCmd = &cobra.Command{
	Use:     "prime",
	GroupID: "setup",
	Short:   "Output AI-optimized workflow context",
	Long: `Output essential binds workflow context in AI-optimized markdown format.

Automatically detects if MCP server is active and adapts output:
- MCP mode: Brief workflow reminders (~50 tokens)
- CLI mode: Full command reference (~1-2k tokens)

Designed for Claude Code hooks (SessionStart, PreCompact) to prevent
agents from forgetting binds workflow after context compaction.

Config options:
- no-git-ops: When true, outputs stealth mode (no git commands in session close protocol).
  Set via: binds config set no-git-ops true
  Useful when you want to control when commits happen manually.

Workflow customization:
- Place a .beads/PRIME.md file to override the default output entirely.
- Use --export to dump the default content for customization.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Find .beads/ directory (supports both database and JSONL-only mode)
		beadsDir := beads.FindBeadsDir()
		if beadsDir == "" {
			// Not in a beads project - silent exit with success
			// CRITICAL: No stderr output, exit 0
			// This enables cross-platform hook integration
			os.Exit(0)
		}

		// Detect MCP mode (unless overridden by flags)
		mcpMode := isMCPActive()
		if primeFullMode {
			mcpMode = false
		}
		if primeMCPMode {
			mcpMode = true
		}

		// Check for stealth mode: flag OR config (GH#593)
		// This allows users to disable git ops in session close protocol via config
		stealthMode := primeStealthMode || config.GetBool("no-git-ops")

		// Check for custom PRIME.md override (unless --export flag)
		// This allows users to fully customize workflow instructions
		// Check local .beads/ first (even if redirected), then redirected location
		if !primeExportMode {
			localPrimePath := filepath.Join(".beads", "PRIME.md")
			redirectedPrimePath := filepath.Join(beadsDir, "PRIME.md")

			// Try local first (user's clone-specific customization)
			// #nosec G304 -- path is relative to cwd
			if content, err := os.ReadFile(localPrimePath); err == nil {
				fmt.Print(string(content))
				return
			}
			// Fall back to redirected location (shared customization)
			// #nosec G304 -- path is constructed from beadsDir which we control
			if content, err := os.ReadFile(redirectedPrimePath); err == nil {
				fmt.Print(string(content))
				return
			}
		}

		// Output workflow context (adaptive based on MCP and stealth mode)
		if err := outputPrimeContext(os.Stdout, mcpMode, stealthMode); err != nil {
			// Suppress all errors - silent exit with success
			// Never write to stderr (breaks Windows compatibility)
			os.Exit(0)
		}
	},
}

func init() {
	primeCmd.Flags().BoolVar(&primeFullMode, "full", false, "Force full CLI output (ignore MCP detection)")
	primeCmd.Flags().BoolVar(&primeMCPMode, "mcp", false, "Force MCP mode (minimal output)")
	primeCmd.Flags().BoolVar(&primeStealthMode, "stealth", false, "Stealth mode (no git operations, flush only)")
	primeCmd.Flags().BoolVar(&primeExportMode, "export", false, "Output default content (ignores PRIME.md override)")
	rootCmd.AddCommand(primeCmd)
}

// isMCPActive detects if MCP server is currently active
func isMCPActive() bool {
	// Get home directory with fallback
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to HOME environment variable
		home = os.Getenv("HOME")
		if home == "" {
			// Can't determine home directory, assume no MCP
			return false
		}
	}

	settingsPath := filepath.Join(home, ".claude/settings.json")
	// #nosec G304 -- settings path derived from user home directory
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return false
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return false
	}

	// Check mcpServers section for beads
	mcpServers, ok := settings["mcpServers"].(map[string]interface{})
	if !ok {
		return false
	}

	// Look for beads server (any key containing "beads")
	for key := range mcpServers {
		if strings.Contains(strings.ToLower(key), "beads") {
			return true
		}
	}

	return false
}

// isEphemeralBranch detects if current branch has no upstream (ephemeral/local-only)
var isEphemeralBranch = func() bool {
	// git rev-parse --abbrev-ref --symbolic-full-name @{u}
	// Returns error code 128 if no upstream configured
	rc, err := internalbeads.GetRepoContext()
	if err != nil {
		return true // Default to ephemeral if we can't determine context
	}
	cmd := rc.GitCmdCWD(context.Background(), "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	return cmd.Run() != nil
}

// primeHasGitRemote detects if any git remote is configured (stubbable for tests)
var primeHasGitRemote = func() bool {
	return syncbranch.HasGitRemote(context.Background())
}

// getRedirectNotice returns a notice string if beads is redirected
func getRedirectNotice(verbose bool) string {
	redirectInfo := beads.GetRedirectInfo()
	if !redirectInfo.IsRedirected {
		return ""
	}

	if verbose {
		return fmt.Sprintf(`> ⚠️ **Redirected**: Local .beads → %s
> You share issues with other clones using this redirect.

`, redirectInfo.TargetDir)
	}
	return fmt.Sprintf("**Note**: Beads redirected to %s (shared with other clones)\n\n", redirectInfo.TargetDir)
}

// outputPrimeContext outputs workflow context in markdown format
func outputPrimeContext(w io.Writer, mcpMode bool, stealthMode bool) error {
	if mcpMode {
		return outputMCPContext(w, stealthMode)
	}
	return outputCLIContext(w, stealthMode)
}

// outputMCPContext outputs minimal context for MCP users
func outputMCPContext(w io.Writer, stealthMode bool) error {
	ephemeral := isEphemeralBranch()
	noPush := config.GetBool("no-push")
	localOnly := !primeHasGitRemote()

	var closeProtocol string
	if stealthMode || localOnly {
		closeProtocol = "Before saying \"done\": binds sync --flush-only"
	} else if ephemeral {
		closeProtocol = "Before saying \"done\": git status → git add → binds sync --from-main → git commit (no push - ephemeral branch)"
	} else if noPush {
		closeProtocol = "Before saying \"done\": git status → git add → binds sync → git commit (push disabled - run git push manually)"
	} else {
		closeProtocol = "Before saying \"done\": git status → git add → binds sync → git commit → binds sync → git push"
	}

	redirectNotice := getRedirectNotice(false)

	context := `# Binds Issue Tracker Active

` + redirectNotice + `# 🚨 SESSION CLOSE PROTOCOL 🚨

` + closeProtocol + `

## Core Rules
- **Default**: Use binds for ALL task tracking (` + "`binds create`" + `, ` + "`binds ready`" + `, ` + "`binds close`" + `)
- **Prohibited**: Do NOT use TodoWrite, TaskCreate, or markdown files for task tracking
- **Workflow**: Create issue BEFORE writing code, mark in_progress when starting
- Persistence you don't need beats lost context

Start: Check ` + "`binds ready`" + ` for available work.
`
	_, _ = fmt.Fprint(w, context)
	return nil
}

// outputCLIContext outputs full CLI reference for non-MCP users
func outputCLIContext(w io.Writer, stealthMode bool) error {
	ephemeral := isEphemeralBranch()
	noPush := config.GetBool("no-push")
	localOnly := !primeHasGitRemote()

	var closeProtocol string
	var closeNote string
	var syncSection string
	var completingWorkflow string
	var gitWorkflowRule string

	if stealthMode || localOnly {
		closeProtocol = `[ ] binds sync --flush-only    (export to JSONL only)`
		syncSection = `### Sync & Collaboration
- ` + "`binds sync --flush-only`" + ` - Export to JSONL`
		completingWorkflow = `**Completing work:**
` + "```bash" + `
binds close <id1> <id2> ...    # Close all completed issues at once
binds sync --flush-only        # Export to JSONL
` + "```"
		if localOnly && !stealthMode {
			closeNote = "**Note:** No git remote configured. Issues are saved locally only."
			gitWorkflowRule = "Git workflow: local-only (no git remote)"
		} else {
			gitWorkflowRule = "Git workflow: stealth mode (no git ops)"
		}
	} else if ephemeral {
		closeProtocol = `[ ] 1. git status              (check what changed)
[ ] 2. git add <files>         (stage code changes)
[ ] 3. binds sync --from-main  (pull updates from main)
[ ] 4. git commit -m "..."     (commit code changes)`
		closeNote = "**Note:** This is an ephemeral branch (no upstream). Code is merged to main locally, not pushed."
		syncSection = `### Sync & Collaboration
- ` + "`binds sync --from-main`" + ` - Pull updates from main (for ephemeral branches)
- ` + "`binds sync --status`" + ` - Check sync status without syncing`
		completingWorkflow = `**Completing work:**
` + "```bash" + `
binds close <id1> <id2> ...    # Close all completed issues at once
binds sync --from-main         # Pull latest from main
git add . && git commit -m "..."  # Commit your changes
` + "```"
		gitWorkflowRule = "Git workflow: run `binds sync --from-main` at session end"
	} else if noPush {
		closeProtocol = `[ ] 1. git status              (check what changed)
[ ] 2. git add <files>         (stage code changes)
[ ] 3. binds sync              (commit changes)
[ ] 4. git commit -m "..."     (commit code)
[ ] 5. binds sync              (commit any new changes)`
		closeNote = "**Note:** Push disabled via config. Run `git push` manually when ready."
		syncSection = `### Sync & Collaboration
- ` + "`binds sync`" + ` - Sync with git remote (run at session end)
- ` + "`binds sync --status`" + ` - Check sync status without syncing`
		completingWorkflow = `**Completing work:**
` + "```bash" + `
binds close <id1> <id2> ...    # Close all completed issues at once
binds sync                     # Sync (push disabled)
# git push                    # Run manually when ready
` + "```"
		gitWorkflowRule = "Git workflow: run `binds sync` at session end (push disabled)"
	} else {
		closeProtocol = `[ ] 1. git status              (check what changed)
[ ] 2. git add <files>         (stage code changes)
[ ] 3. binds sync              (commit changes)
[ ] 4. git commit -m "..."     (commit code)
[ ] 5. binds sync              (commit any new changes)
[ ] 6. git push                (push to remote)`
		closeNote = "**NEVER skip this.** Work is not done until pushed."
		syncSection = `### Sync & Collaboration
- ` + "`binds sync`" + ` - Sync with git remote (run at session end)
- ` + "`binds sync --status`" + ` - Check sync status without syncing`
		completingWorkflow = `**Completing work:**
` + "```bash" + `
binds close <id1> <id2> ...    # Close all completed issues at once
binds sync                     # Push to remote
` + "```"
		gitWorkflowRule = "Git workflow: hooks auto-sync, run `binds sync` at session end"
	}

	redirectNotice := getRedirectNotice(true)

	context := `# Binds Workflow Context

> **Context Recovery**: Run ` + "`binds prime`" + ` after compaction, clear, or new session
> Hooks auto-call this in Claude Code when .beads/ detected

` + redirectNotice + `# 🚨 SESSION CLOSE PROTOCOL 🚨

**CRITICAL**: Before saying "done" or "complete", you MUST run this checklist:

` + "```" + `
` + closeProtocol + `
` + "```" + `

` + closeNote + `

## Core Rules
- **Default**: Use binds for ALL task tracking (` + "`binds create`" + `, ` + "`binds ready`" + `, ` + "`binds close`" + `)
- **Prohibited**: Do NOT use TodoWrite, TaskCreate, or markdown files for task tracking
- **Workflow**: Create issue BEFORE writing code, mark in_progress when starting
- Persistence you don't need beats lost context
- ` + gitWorkflowRule + `
- Session management: check ` + "`binds ready`" + ` for available work

## Essential Commands

### Finding Work
- ` + "`binds ready`" + ` - Show issues ready to work (no blockers)
- ` + "`binds list --status=open`" + ` - All open issues
- ` + "`binds list --status=in_progress`" + ` - Your active work
- ` + "`binds show <id>`" + ` - Detailed issue view with dependencies

### Creating & Updating
- ` + "`binds create --title=\"...\" --type=task|bug|feature --priority=2`" + ` - New issue
  - Priority: 0-4 or P0-P4 (0=critical, 2=medium, 4=backlog). NOT "high"/"medium"/"low"
- ` + "`binds update <id> --status=in_progress`" + ` - Claim work
- ` + "`binds update <id> --assignee=username`" + ` - Assign to someone
- ` + "`binds update <id> --title/--description/--notes/--design`" + ` - Update fields inline
- ` + "`binds close <id>`" + ` - Mark complete
- ` + "`binds close <id1> <id2> ...`" + ` - Close multiple issues at once (more efficient)
- ` + "`binds close <id> --reason=\"explanation\"`" + ` - Close with reason
- **Tip**: When creating multiple issues/tasks/epics, use parallel subagents for efficiency
- **WARNING**: Do NOT use ` + "`binds edit`" + ` - it opens $EDITOR (vim/nano) which blocks agents

### Dependencies & Blocking
- ` + "`binds dep add <issue> <depends-on>`" + ` - Add dependency (issue depends on depends-on)
- ` + "`binds blocked`" + ` - Show all blocked issues
- ` + "`binds show <id>`" + ` - See what's blocking/blocked by this issue

` + syncSection + `

### Project Health
- ` + "`binds stats`" + ` - Project statistics (open/closed/blocked counts)
- ` + "`binds doctor`" + ` - Check for issues (sync problems, missing hooks)

## Common Workflows

**Starting work:**
` + "```bash" + `
binds ready           # Find available work
binds show <id>       # Review issue details
binds update <id> --status=in_progress  # Claim it
` + "```" + `

` + completingWorkflow + `

**Creating dependent work:**
` + "```bash" + `
# Run binds create commands in parallel (use subagents for many items)
binds create --title="Implement feature X" --type=feature
binds create --title="Write tests for X" --type=task
binds dep add binds-yyy binds-xxx  # Tests depend on Feature (Feature blocks tests)
` + "```" + `
`
	_, _ = fmt.Fprint(w, context)
	return nil
}
