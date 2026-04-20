package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var issuesCmd = &cobra.Command{
	Use:   "issues",
	Short: "List issues across all workspaces from the binds server",
	Long: `Query the cross-repo issue index on the binds coordination server.

Shows issues from all workspaces that have been pushed via 'binds push'.
Use --workspace to filter by workspace, --status to filter by status.

Requires a running binds server (binds serve).

Examples:
  binds issues                        # All open issues
  binds issues --status in_progress   # In-progress only
  binds issues --workspace Source     # Source workspace only
  binds issues --all                  # Include closed`,
	RunE: func(cmd *cobra.Command, args []string) error {
		workspace, _ := cmd.Flags().GetString("workspace")
		status, _ := cmd.Flags().GetString("status")
		all, _ := cmd.Flags().GetBool("all")
		limit, _ := cmd.Flags().GetInt("limit")

		if status == "" && !all {
			status = "open"
		}

		params := fmt.Sprintf("?limit=%d", limit)
		if workspace != "" {
			params += "&workspace=" + workspace
		}
		if status != "" {
			params += "&status=" + status
		}

		resp, err := serverGet("/api/issues" + params)
		if err != nil {
			return err
		}

		if jsonOutput {
			fmt.Println(string(resp))
			return nil
		}

		var data struct {
			Issues []struct {
				Workspace string `json:"workspace"`
				IssueID   string `json:"issue_id"`
				Title     string `json:"title"`
				Status    string `json:"status"`
				Priority  int    `json:"priority"`
				IssueType string `json:"issue_type"`
				Assignee  string `json:"assignee"`
				UpdatedAt string `json:"updated_at"`
			} `json:"issues"`
			Count int `json:"count"`
		}
		if err := json.Unmarshal(resp, &data); err != nil {
			return err
		}

		if len(data.Issues) == 0 {
			fmt.Println("No issues found")
			return nil
		}

		lastWs := ""
		for _, i := range data.Issues {
			if i.Workspace != lastWs {
				if lastWs != "" {
					fmt.Println()
				}
				fmt.Printf("── %s ──\n", i.Workspace)
				lastWs = i.Workspace
			}

			icon := "○"
			switch i.Status {
			case "in_progress":
				icon = "◐"
			case "closed":
				icon = "●"
			}
			prio := fmt.Sprintf("P%d", i.Priority)
			line := fmt.Sprintf("  %s %-15s [%s] %s", icon, i.IssueID, prio, truncate(i.Title, 60))
			if i.Assignee != "" {
				line += fmt.Sprintf("  @%s", i.Assignee)
			}
			fmt.Println(line)
		}
		fmt.Printf("\n%d issues\n", data.Count)
		return nil
	},
}

var issueStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Issue statistics across workspaces",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := serverGet("/api/issues/stats")
		if err != nil {
			return err
		}

		if jsonOutput {
			fmt.Println(string(resp))
			return nil
		}

		var stats map[string]int
		json.Unmarshal(resp, &stats)

		workspaces := map[string]bool{}
		for k := range stats {
			parts := strings.SplitN(k, "_", 2)
			if len(parts) == 2 && parts[0] != "total" {
				workspaces[parts[0]] = true
			}
		}

		for ws := range workspaces {
			total := stats[ws+"_total"]
			open := stats[ws+"_open"]
			ip := stats[ws+"_in_progress"]
			closed := stats[ws+"_closed"]
			fmt.Printf("%-20s  %d total  %d open  %d in-progress  %d closed\n", ws, total, open, ip, closed)
		}
		if t, ok := stats["total"]; ok {
			fmt.Printf("\nTotal: %d issues\n", t)
		}
		return nil
	},
}

func init() {
	issuesCmd.Flags().String("workspace", "", "Filter by workspace")
	issuesCmd.Flags().String("status", "", "Filter by status (open|in_progress|closed)")
	issuesCmd.Flags().Bool("all", false, "Include closed issues")
	issuesCmd.Flags().Int("limit", 100, "Maximum issues to return")
	issuesCmd.AddCommand(issueStatsCmd)
	rootCmd.AddCommand(issuesCmd)
}
