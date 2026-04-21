package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/IkuTri/binds/internal/server"
)

var whoCmd = &cobra.Command{
	Use:   "who",
	Short: "Show online agents and active reservations",
	Long: `Show which agents are currently online, their workspaces, and any active file reservations.

Requires a running binds server (binds serve).

Examples:
  binds who              # Show online agents
  binds who --json       # JSON output`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, base, err := serverClient()
		if err != nil {
			return err
		}

		req, _ := http.NewRequest("GET", base+"/api/presence", nil)
		addAuth(req)
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("server unreachable (is 'binds serve' running?): %w", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			return fmt.Errorf("server error: %s", string(body))
		}

		var data struct {
			Agents []struct {
				Name      string `json:"name"`
				AgentType string `json:"agent_type"`
				Status    string `json:"status"`
				Workspace string `json:"workspace"`
				LastSeen  string `json:"last_seen"`
			} `json:"agents"`
			Reservations []struct {
				ID        int64  `json:"id"`
				Holder    string `json:"holder"`
				Pattern   string `json:"pattern"`
				Reason    string `json:"reason"`
				Exclusive bool   `json:"exclusive"`
				Expires   string `json:"expires"`
			} `json:"reservations"`
		}
		if err := json.Unmarshal(body, &data); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}

		if jsonOutput {
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(data)
		}

		if len(data.Agents) == 0 {
			fmt.Println("No agents online")
		} else {
			fmt.Printf("Online agents (%d):\n", len(data.Agents))
			for _, a := range data.Agents {
				status := a.Status
				if status == "online" {
					status = "●"
				} else if status == "busy" {
					status = "◐"
				}
				line := fmt.Sprintf("  %s %s (%s)", status, a.Name, a.AgentType)
				if a.Workspace != "" {
					line += fmt.Sprintf("  %s", shortenPath(a.Workspace))
				}
				if a.LastSeen != "" {
					if t, err := time.Parse(time.RFC3339, a.LastSeen); err == nil {
						line += fmt.Sprintf("  %s ago", time.Since(t).Round(time.Second))
					}
				}
				fmt.Println(line)
			}
		}

		if len(data.Reservations) > 0 {
			fmt.Printf("\nActive reservations (%d):\n", len(data.Reservations))
			for _, r := range data.Reservations {
				line := fmt.Sprintf("  %s → %s", r.Holder, r.Pattern)
				if r.Reason != "" {
					line += fmt.Sprintf(" (%s)", r.Reason)
				}
				if r.Expires != "" {
					if t, err := time.Parse(time.RFC3339, r.Expires); err == nil {
						remaining := time.Until(t).Round(time.Second)
						line += fmt.Sprintf("  expires in %s", remaining)
					}
				}
				fmt.Println(line)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(whoCmd)
}

func shortenPath(p string) string {
	home, _ := os.UserHomeDir()
	if home != "" && strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}

func serverClient() (*http.Client, string, error) {
	port := 8890
	home, _ := os.UserHomeDir()
	if home != "" {
		if cfg, err := server.LoadConfigFile(filepath.Join(home, ".config", "binds")); err == nil && cfg.Server.Port > 0 {
			port = cfg.Server.Port
		}
	}
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	return &http.Client{Timeout: 10 * time.Second}, base, nil
}

func addAuth(req *http.Request) {
	home, _ := os.UserHomeDir()
	tokenPath := home + "/.config/binds/.local-token"
	if data, err := os.ReadFile(tokenPath); err == nil {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(data)))
	}
}
