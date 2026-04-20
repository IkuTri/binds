package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var registryCmd = &cobra.Command{
	Use:   "registry",
	Short: "Manage registered agents on the coordination server",
	Long: `Register, list, and revoke agents on the binds coordination server.

Requires a running binds server (binds serve).

Commands:
  binds registry register <name> --type <type>   Register a new agent
  binds registry list                             List registered agents
  binds registry revoke <name>                    Revoke an agent's token`,
}

var registryRegisterCmd = &cobra.Command{
	Use:   "register <name>",
	Short: "Register a new agent and receive an API token",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		agentType, _ := cmd.Flags().GetString("type")
		if agentType == "" {
			agentType = "generic"
		}

		resp, err := serverPost("/api/agents/register", map[string]interface{}{
			"name":       name,
			"agent_type": agentType,
		})
		if err != nil {
			return err
		}

		if jsonOutput {
			fmt.Println(string(resp))
			return nil
		}

		var data struct {
			Name  string `json:"name"`
			Token string `json:"token"`
		}
		json.Unmarshal(resp, &data)
		fmt.Printf("Agent %q registered\n", data.Name)
		fmt.Printf("Token: %s\n", data.Token)
		fmt.Println("Store this token — it cannot be retrieved later.")
		return nil
	},
}

var registryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := serverGet("/api/agents")
		if err != nil {
			return err
		}

		if jsonOutput {
			fmt.Println(string(resp))
			return nil
		}

		var data struct {
			Agents []struct {
				Name      string `json:"name"`
				AgentType string `json:"agent_type"`
				Status    string `json:"status"`
				Workspace string `json:"workspace"`
				LastSeen  string `json:"last_seen"`
				CreatedAt string `json:"created_at"`
			} `json:"agents"`
		}
		json.Unmarshal(resp, &data)

		if len(data.Agents) == 0 {
			fmt.Println("No registered agents")
			return nil
		}

		for _, a := range data.Agents {
			status := "○"
			if a.Status == "online" {
				status = "●"
			} else if a.Status == "busy" {
				status = "◐"
			}
			line := fmt.Sprintf("  %s %-20s (%s)", status, a.Name, a.AgentType)
			if a.LastSeen != "" {
				if t, err := time.Parse(time.RFC3339, a.LastSeen); err == nil {
					line += fmt.Sprintf("  seen %s ago", time.Since(t).Round(time.Second))
				}
			}
			fmt.Println(line)
		}
		return nil
	},
}

var registryRevokeCmd = &cobra.Command{
	Use:   "revoke <name>",
	Short: "Revoke an agent's API token",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := serverDelete("/api/agents/" + args[0])
		if err != nil {
			return err
		}
		if jsonOutput {
			fmt.Printf(`{"status":"revoked","name":"%s"}`+"\n", args[0])
			return nil
		}
		fmt.Printf("Agent %q revoked\n", args[0])
		return nil
	},
}

func init() {
	registryRegisterCmd.Flags().StringP("type", "t", "generic", "Agent type (e.g., claude-code, codex, generic)")

	registryCmd.AddCommand(registryRegisterCmd)
	registryCmd.AddCommand(registryListCmd)
	registryCmd.AddCommand(registryRevokeCmd)

	rootCmd.AddCommand(registryCmd)
}
