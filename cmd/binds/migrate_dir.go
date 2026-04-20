package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var migrateDirCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Rename .beads/ to .binds/ in the current repository",
	Long: `Migrate the local issue database directory from .beads/ to .binds/.

This renames the directory, updates .gitignore entries, and updates the
beads-sync branch reference if present.

The binds CLI already discovers .binds/ with priority over .beads/,
so after migration all commands work transparently.

Examples:
  binds migrate dir              # Migrate current repo
  binds migrate dir --dry-run    # Show what would change`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getwd: %w", err)
		}

		oldDir := filepath.Join(cwd, ".beads")
		newDir := filepath.Join(cwd, ".binds")

		if _, err := os.Stat(newDir); err == nil {
			return fmt.Errorf(".binds/ already exists — already migrated?")
		}

		if _, err := os.Stat(oldDir); os.IsNotExist(err) {
			return fmt.Errorf("no .beads/ directory found in %s", cwd)
		}

		fmt.Printf("Migrating .beads/ → .binds/ in %s\n", cwd)

		if dryRun {
			fmt.Println("  [dry-run] would rename .beads/ → .binds/")
		} else {
			if err := os.Rename(oldDir, newDir); err != nil {
				return fmt.Errorf("rename .beads/ → .binds/: %w", err)
			}
			fmt.Println("  Renamed .beads/ → .binds/")
		}

		gitignorePath := filepath.Join(cwd, ".gitignore")
		if data, err := os.ReadFile(gitignorePath); err == nil {
			content := string(data)
			if strings.Contains(content, ".beads/") {
				updated := strings.ReplaceAll(content, ".beads/", ".binds/")
				if dryRun {
					fmt.Println("  [dry-run] would update .gitignore (.beads/ → .binds/)")
				} else {
					if err := os.WriteFile(gitignorePath, []byte(updated), 0644); err != nil {
						fmt.Fprintf(os.Stderr, "  Warning: could not update .gitignore: %v\n", err)
					} else {
						fmt.Println("  Updated .gitignore")
					}
				}
			}
		}

		if dryRun {
			fmt.Println("\nDry run complete. Run without --dry-run to apply.")
		} else {
			fmt.Println("\nMigration complete. Run 'binds doctor' to verify.")
		}

		return nil
	},
}

func init() {
	migrateDirCmd.Flags().Bool("dry-run", false, "Show what would change without applying")
	rootCmd.AddCommand(migrateDirCmd)
}
