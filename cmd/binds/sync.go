package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/beads/internal/beads"
)

var syncCmd = &cobra.Command{
	Use:     "sync",
	GroupID: "sync",
	Short:   "Export database to JSONL and push to remote (git-only sync)",
	Long: `Dead-simple git-only sync for beads.

Commands:
  binds sync              Export to JSONL → git add → git commit → git push
  binds sync --pull       git pull → import from JSONL
  binds sync --status     Show dirty issue count and git status of .beads/
  binds sync --flush-only Just export to JSONL, skip git ops
  binds sync --import-only Just import from JSONL, skip git ops

Flags:
  -m, --message    Custom commit message
  --dry-run        Preview without making changes
  --no-push        Skip git push
  --no-pull        Skip git pull (for default push path)`,
	Run: func(cmd *cobra.Command, _ []string) {
		CheckReadonly("sync")
		ctx := rootCtx

		message, _ := cmd.Flags().GetString("message")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		noPush, _ := cmd.Flags().GetBool("no-push")
		noPull, _ := cmd.Flags().GetBool("no-pull")
		flushOnly, _ := cmd.Flags().GetBool("flush-only")
		importOnly, _ := cmd.Flags().GetBool("import-only")
		importFlag, _ := cmd.Flags().GetBool("import")
		pull, _ := cmd.Flags().GetBool("pull")
		status, _ := cmd.Flags().GetBool("status")

		// --import is shorthand for --import-only
		if importFlag {
			importOnly = true
		}

		// Initialize local store.
		if err := ensureStoreActive(); err != nil {
			FatalError("failed to initialize store: %v", err)
		}

		// Find JSONL path
		jsonlPath := findJSONLPath()
		if jsonlPath == "" {
			FatalError("not in a bd workspace (no .beads directory found)")
		}

		// --status: show dirty count and git status of .beads/
		if status {
			if err := showSimpleSyncStatus(ctx, jsonlPath); err != nil {
				FatalError("%v", err)
			}
			return
		}

		// --pull: git pull → import from JSONL
		if pull {
			if dryRun {
				fmt.Println("→ [DRY RUN] Would git pull")
				fmt.Println("→ [DRY RUN] Would import from JSONL")
				return
			}
			fmt.Println("→ Pulling from remote...")
			if err := gitPull(ctx, ""); err != nil {
				FatalError("git pull failed: %v", err)
			}
			fmt.Println("→ Importing from JSONL...")
			if err := importFromJSONLInline(ctx, jsonlPath, false, false, false); err != nil {
				FatalError("import failed: %v", err)
			}
			fmt.Println("✓ Pull + import complete")
			return
		}

		// --import-only: just import, no git ops
		if importOnly {
			if dryRun {
				fmt.Println("→ [DRY RUN] Would import from JSONL")
				return
			}
			fmt.Println("→ Importing from JSONL...")
			if err := importFromJSONLInline(ctx, jsonlPath, false, false, false); err != nil {
				FatalError("import failed: %v", err)
			}
			fmt.Println("✓ Import complete")
			return
		}

		// --flush-only: just export, no git ops
		if flushOnly {
			if dryRun {
				fmt.Println("→ [DRY RUN] Would export to JSONL")
				return
			}
			if err := exportToJSONL(ctx, jsonlPath); err != nil {
				FatalError("export failed: %v", err)
			}
			fmt.Println("✓ Export complete")
			return
		}

		// DEFAULT: export → git add → git commit → git push
		if dryRun {
			fmt.Println("→ [DRY RUN] Would export to JSONL")
			fmt.Println("→ [DRY RUN] Would git add .binds/")
			fmt.Println("→ [DRY RUN] Would git commit")
			if !noPush {
				fmt.Println("→ [DRY RUN] Would git push")
			}
			fmt.Println("\n✓ Dry run complete (no changes made)")
			return
		}

		// Pull first unless suppressed
		if !noPull && hasGitRemote(ctx) {
			fmt.Println("→ Pulling from remote...")
			if err := gitPull(ctx, ""); err != nil {
				// Non-fatal: warn and continue with push
				fmt.Fprintf(os.Stderr, "Warning: git pull failed: %v\n", err)
			}
		}

		// Export issues to JSONL
		fmt.Println("→ Exporting to JSONL...")
		if err := exportToJSONL(ctx, jsonlPath); err != nil {
			FatalError("export failed: %v", err)
		}

		// Check for changes to commit
		hasChanges, err := gitHasBeadsChanges(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: checking git status: %v\n", err)
		}

		if !hasChanges {
			fmt.Println("→ No changes to commit")
			fmt.Println("\n✓ Sync complete (nothing to push)")
			return
		}

		// Build commit message
		if message == "" {
			// Count issues for the commit message
			issueCount := countExportedIssues(ctx)
			message = fmt.Sprintf("binds: sync %d issues", issueCount)
		}

		fmt.Println("→ Committing changes...")
		if err := gitCommitBeadsDir(ctx, message); err != nil {
			FatalError("git commit failed: %v", err)
		}

		if !noPush {
			fmt.Println("→ Pushing to remote...")
			if err := gitPush(ctx, ""); err != nil {
				FatalError("git push failed: %v", err)
			}
		}

		fmt.Println("\n✓ Sync complete")
	},
}

// showSimpleSyncStatus shows dirty issue count and git status of .beads/.
func showSimpleSyncStatus(ctx context.Context, jsonlPath string) error {
	beadsDir := filepath.Dir(jsonlPath)

	// Dirty issue count
	if store != nil {
		dirtyIDs, err := store.GetDirtyIssues(ctx)
		if err != nil {
			fmt.Printf("Pending changes: unknown (%v)\n", err)
		} else if len(dirtyIDs) == 0 {
			fmt.Println("Pending changes: none")
		} else {
			fmt.Printf("Pending changes: %d issues modified since last export\n", len(dirtyIDs))
		}
	}

	// Git status of .beads/ dir (using RepoContext for worktree awareness)
	rc, err := beads.GetRepoContext()
	if err != nil {
		fmt.Printf("Git status: unavailable (%v)\n", err)
		return nil
	}

	relBeadsDir, err := filepath.Rel(rc.RepoRoot, beadsDir)
	if err != nil {
		relBeadsDir = beadsDir
	}

	statusCmd := rc.GitCmd(ctx, "status", "--short", relBeadsDir)
	output, err := statusCmd.Output()
	if err != nil {
		fmt.Printf("Git status: error (%v)\n", err)
		return nil
	}

	statusStr := strings.TrimSpace(string(output))
	if statusStr == "" {
		fmt.Println("Git status: clean (no uncommitted changes in .binds/)")
	} else {
		fmt.Println("Git status:")
		for _, line := range strings.Split(statusStr, "\n") {
			fmt.Printf("  %s\n", line)
		}
	}

	// Show whether remote is ahead/behind
	if hasGitRemote(ctx) {
		aheadBehindCmd := rc.GitCmd(ctx, "status", "--short", "--branch")
		abOutput, err := aheadBehindCmd.Output()
		if err == nil {
			firstLine := strings.Split(strings.TrimSpace(string(abOutput)), "\n")[0]
			if strings.Contains(firstLine, "ahead") || strings.Contains(firstLine, "behind") {
				fmt.Printf("Remote: %s\n", strings.TrimPrefix(firstLine, "## "))
			}
		}
	}

	return nil
}

// countExportedIssues returns the count of non-tombstone issues for commit messages.
func countExportedIssues(ctx context.Context) int {
	if store == nil {
		return 0
	}
	issues, err := store.SearchIssues(ctx, "", beads.IssueFilter{})
	if err != nil {
		return 0
	}
	return len(issues)
}

func init() {
	syncCmd.Flags().StringP("message", "m", "", "Commit message (default: auto-generated)")
	syncCmd.Flags().Bool("dry-run", false, "Preview sync without making changes")
	syncCmd.Flags().Bool("no-push", false, "Skip pushing to remote")
	syncCmd.Flags().Bool("no-pull", false, "Skip pulling from remote before push")
	syncCmd.Flags().Bool("flush-only", false, "Only export pending changes to JSONL (skip git operations)")
	syncCmd.Flags().Bool("import-only", false, "Only import from JSONL (skip git operations, useful after git pull)")
	syncCmd.Flags().Bool("import", false, "Import from JSONL (shorthand for --import-only)")
	syncCmd.Flags().Bool("pull", false, "git pull then import from JSONL")
	syncCmd.Flags().Bool("status", false, "Show pending changes and git status of .beads/")
	rootCmd.AddCommand(syncCmd)
}
