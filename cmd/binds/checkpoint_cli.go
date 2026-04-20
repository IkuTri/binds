package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var checkpointCmd = &cobra.Command{
	Use:   "checkpoint",
	Short: "Milestone checkpoints for coordinated work",
	Long: `Create and manage milestone checkpoints that track coordinated work across agents.

Requires a running binds server (binds serve).

Commands:
  binds checkpoint create <title>              Create a checkpoint
  binds checkpoint list [--status <s>]         List checkpoints
  binds checkpoint show <id-or-slug>           Show checkpoint details
  binds checkpoint update <id> --status <s>    Update status
  binds checkpoint complete <id>               Mark complete`,
}

var checkpointCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a new checkpoint",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title := strings.Join(args, " ")
		description, _ := cmd.Flags().GetString("description")
		priority, _ := cmd.Flags().GetString("priority")
		slug, _ := cmd.Flags().GetString("slug")
		parentID, _ := cmd.Flags().GetInt64("parent")
		workspace, _ := cmd.Flags().GetString("workspace")
		bindsIDs, _ := cmd.Flags().GetString("binds-ids")

		payload := map[string]interface{}{
			"title": title,
		}
		if description != "" {
			payload["description"] = description
		}
		if priority != "" {
			payload["priority"] = priority
		}
		if slug != "" {
			payload["slug"] = slug
		}
		if parentID > 0 {
			payload["parent_id"] = parentID
		}
		if workspace != "" {
			payload["workspace_id"] = workspace
		}
		if bindsIDs != "" {
			payload["binds_ids"] = bindsIDs
		}

		resp, err := serverPost("/api/checkpoints", payload)
		if err != nil {
			return err
		}

		if jsonOutput {
			fmt.Println(string(resp))
			return nil
		}

		var data struct {
			ID   int64  `json:"id"`
			Slug string `json:"slug"`
		}
		json.Unmarshal(resp, &data)
		label := fmt.Sprintf("#%d", data.ID)
		if data.Slug != "" {
			label = data.Slug
		}
		fmt.Printf("Checkpoint %s created: %s\n", label, title)
		return nil
	},
}

var checkpointListCmd = &cobra.Command{
	Use:   "list",
	Short: "List checkpoints",
	RunE: func(cmd *cobra.Command, args []string) error {
		status, _ := cmd.Flags().GetString("status")
		params := ""
		if status != "" {
			params = "?status=" + status
		}

		resp, err := serverGet("/api/checkpoints" + params)
		if err != nil {
			return err
		}

		if jsonOutput {
			fmt.Println(string(resp))
			return nil
		}

		var data struct {
			Checkpoints []struct {
				ID          int64  `json:"id"`
				Title       string `json:"title"`
				Priority    string `json:"priority"`
				Status      string `json:"status"`
				Slug        string `json:"slug"`
				CreatedAt   string `json:"created_at"`
				CompletedAt string `json:"completed_at"`
			} `json:"checkpoints"`
		}
		json.Unmarshal(resp, &data)

		if len(data.Checkpoints) == 0 {
			fmt.Println("No checkpoints")
			return nil
		}

		for _, c := range data.Checkpoints {
			icon := "○"
			switch c.Status {
			case "in_progress":
				icon = "◐"
			case "completed":
				icon = "●"
			case "blocked":
				icon = "✕"
			}
			label := fmt.Sprintf("#%d", c.ID)
			if c.Slug != "" {
				label = c.Slug
			}
			ts := ""
			if c.CompletedAt != "" {
				if t, err := time.Parse(time.RFC3339, c.CompletedAt); err == nil {
					ts = fmt.Sprintf("  completed %s ago", time.Since(t).Round(time.Second))
				}
			} else if c.CreatedAt != "" {
				if t, err := time.Parse(time.RFC3339, c.CreatedAt); err == nil {
					ts = fmt.Sprintf("  created %s ago", time.Since(t).Round(time.Second))
				}
			}
			fmt.Printf("  %s %-12s [%s] %s%s\n", icon, label, c.Priority, c.Title, ts)
		}
		return nil
	},
}

var checkpointShowCmd = &cobra.Command{
	Use:   "show <id-or-slug>",
	Short: "Show checkpoint details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := serverGet("/api/checkpoints/" + args[0])
		if err != nil {
			return err
		}

		if jsonOutput {
			fmt.Println(string(resp))
			return nil
		}

		var c struct {
			ID          int64  `json:"id"`
			ParentID    *int64 `json:"parent_id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Priority    string `json:"priority"`
			Status      string `json:"status"`
			Slug        string `json:"slug"`
			WorkspaceID string `json:"workspace_id"`
			BindsIDs    string `json:"binds_ids"`
			CreatedAt   string `json:"created_at"`
			UpdatedAt   string `json:"updated_at"`
			CompletedAt string `json:"completed_at"`
		}
		json.Unmarshal(resp, &c)

		label := fmt.Sprintf("#%d", c.ID)
		if c.Slug != "" {
			label = c.Slug
		}
		fmt.Printf("%s — %s\n", label, c.Title)
		fmt.Printf("Status: %s  Priority: %s\n", c.Status, c.Priority)
		if c.Description != "" {
			fmt.Printf("\n%s\n", c.Description)
		}
		if c.WorkspaceID != "" {
			fmt.Printf("Workspace: %s\n", c.WorkspaceID)
		}
		if c.BindsIDs != "" {
			fmt.Printf("Binds: %s\n", c.BindsIDs)
		}
		if c.ParentID != nil {
			fmt.Printf("Parent: #%d\n", *c.ParentID)
		}
		return nil
	},
}

var checkpointUpdateCmd = &cobra.Command{
	Use:   "update <id> --status <status>",
	Short: "Update checkpoint status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		status, _ := cmd.Flags().GetString("status")
		if status == "" {
			return fmt.Errorf("--status required")
		}

		resp, err := serverPatch("/api/checkpoints/" + args[0] + "?status=" + status)
		if err != nil {
			return err
		}

		if jsonOutput {
			fmt.Println(string(resp))
			return nil
		}

		fmt.Printf("Checkpoint #%s updated to %s\n", args[0], status)
		return nil
	},
}

var checkpointCompleteCmd = &cobra.Command{
	Use:   "complete <id>",
	Short: "Mark checkpoint as completed",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := serverPatch("/api/checkpoints/" + args[0] + "?status=completed")
		if err != nil {
			return err
		}

		if jsonOutput {
			fmt.Println(string(resp))
			return nil
		}

		fmt.Printf("Checkpoint #%s completed\n", args[0])
		return nil
	},
}

func init() {
	checkpointCreateCmd.Flags().StringP("description", "d", "", "Checkpoint description")
	checkpointCreateCmd.Flags().StringP("priority", "p", "P2", "Priority (P0-P4)")
	checkpointCreateCmd.Flags().String("slug", "", "Human-readable slug")
	checkpointCreateCmd.Flags().Int64("parent", 0, "Parent checkpoint ID")
	checkpointCreateCmd.Flags().String("workspace", "", "Workspace identifier")
	checkpointCreateCmd.Flags().String("binds-ids", "", "Associated binds issue IDs (comma-separated)")
	checkpointListCmd.Flags().String("status", "", "Filter by status (pending|in_progress|completed|blocked)")
	checkpointUpdateCmd.Flags().String("status", "", "New status (pending|in_progress|completed|blocked)")

	checkpointCmd.AddCommand(checkpointCreateCmd)
	checkpointCmd.AddCommand(checkpointListCmd)
	checkpointCmd.AddCommand(checkpointShowCmd)
	checkpointCmd.AddCommand(checkpointUpdateCmd)
	checkpointCmd.AddCommand(checkpointCompleteCmd)

	rootCmd.AddCommand(checkpointCmd)
}
