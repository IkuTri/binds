package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/IkuTri/binds/internal/types"
	"github.com/IkuTri/binds/internal/ui"
	"github.com/IkuTri/binds/internal/utils"
)

var undeferCmd = &cobra.Command{
	Use:   "undefer [id...]",
	Short: "Undefer one or more issues (restore to open)",
	Long: `Undefer issues to restore them to open status.

This brings issues back from the icebox so they can be worked on again.
Issues will appear in 'binds ready' if they have no blockers.

Examples:
  bd undefer bd-abc        # Undefer a single issue
  bd undefer bd-abc bd-def # Undefer multiple issues`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		CheckReadonly("undefer")

		ctx := rootCtx

		if store == nil {
			fmt.Fprintln(os.Stderr, "Error: database not initialized")
			os.Exit(1)
		}

		resolvedIDs, err := utils.ResolvePartialIDs(ctx, store, args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		undeferredIssues := []*types.Issue{}

		for _, id := range resolvedIDs {
			updates := map[string]interface{}{
				"status":      string(types.StatusOpen),
				"defer_until": nil, // Clear defer_until timestamp (GH#820)
			}

			if err := store.UpdateIssue(ctx, id, updates, actor); err != nil {
				fmt.Fprintf(os.Stderr, "Error undeferring %s: %v\n", id, err)
				continue
			}

			if jsonOutput {
				issue, _ := store.GetIssue(ctx, id)
				if issue != nil {
					undeferredIssues = append(undeferredIssues, issue)
				}
			} else {
				fmt.Printf("%s Undeferred %s (now open)\n", ui.RenderPass("*"), id)
			}
		}

		// Schedule auto-flush if any issues were undeferred
		if len(args) > 0 {
			markDirtyAndScheduleFlush()
		}

		if jsonOutput && len(undeferredIssues) > 0 {
			outputJSON(undeferredIssues)
		}
	},
}

func init() {
	undeferCmd.ValidArgsFunction = issueIDCompletion
	rootCmd.AddCommand(undeferCmd)
}
