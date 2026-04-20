package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var heartbeatCmd = &cobra.Command{
	Use:   "heartbeat",
	Short: "Send a presence heartbeat to the binds server",
	Long: `Send a heartbeat to the coordination server to signal this agent is alive.

The server's presence reaper removes agents that haven't sent a heartbeat
within 90 seconds. Send heartbeats at least every 60 seconds to stay online.

The workspace is auto-detected from the current working directory.

Examples:
  binds heartbeat                    # Heartbeat with cwd as workspace
  binds heartbeat --workspace /path  # Explicit workspace
  binds heartbeat --status busy      # Mark as busy`,
	RunE: func(cmd *cobra.Command, args []string) error {
		workspace, _ := cmd.Flags().GetString("workspace")
		status, _ := cmd.Flags().GetString("status")

		if workspace == "" {
			workspace, _ = os.Getwd()
		}

		payload := map[string]interface{}{
			"workspace": workspace,
			"status":    status,
		}

		resp, err := serverPost("/api/presence/heartbeat", payload)
		if err != nil {
			return err
		}

		if jsonOutput {
			fmt.Println(string(resp))
			return nil
		}

		fmt.Println("Heartbeat sent")
		return nil
	},
}

func init() {
	heartbeatCmd.Flags().String("workspace", "", "Workspace path (default: cwd)")
	heartbeatCmd.Flags().String("status", "online", "Status (online|busy)")
	rootCmd.AddCommand(heartbeatCmd)
}
