package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var reserveCmd = &cobra.Command{
	Use:   "reserve <patterns...>",
	Short: "Advisory file reservations",
	Long: `Reserve file patterns to signal intent to other agents.

Reservations are advisory — nothing blocks writes. They are used to
communicate which files an agent intends to modify, reducing merge conflicts.

Requires a running binds server (binds serve).

Examples:
  binds reserve "src/auth/*.go"           # Reserve auth files for 30m
  binds reserve "*.proto" --ttl 1h        # Reserve proto files for 1h
  binds reserve --check "src/auth/*.go"   # Check for conflicts
  binds reserve --list                    # List active reservations
  binds reserve --release 42              # Release reservation #42`,
	RunE: func(cmd *cobra.Command, args []string) error {
		checkFlag, _ := cmd.Flags().GetBool("check")
		listFlag, _ := cmd.Flags().GetBool("list")
		releaseFlag, _ := cmd.Flags().GetInt64("release")
		reason, _ := cmd.Flags().GetString("reason")
		ttl, _ := cmd.Flags().GetString("ttl")

		if listFlag {
			return reserveList()
		}

		if releaseFlag > 0 {
			return reserveRelease(releaseFlag)
		}

		if len(args) == 0 {
			return fmt.Errorf("provide file patterns to reserve")
		}

		if checkFlag {
			return reserveCheck(args)
		}

		return reserveCreate(args, reason, ttl)
	},
}

func reserveCreate(patterns []string, reason, ttl string) error {
	payload := map[string]interface{}{
		"patterns": patterns,
	}
	if reason != "" {
		payload["reason"] = reason
	}
	if ttl != "" {
		payload["ttl"] = ttl
	}

	resp, err := serverPost("/api/reservations", payload)
	if err != nil {
		return err
	}

	if jsonOutput {
		fmt.Println(string(resp))
		return nil
	}

	var data struct {
		Status       string `json:"status"`
		Reservations []struct {
			ID        int64  `json:"id"`
			Pattern   string `json:"pattern"`
			ExpiresAt string `json:"expires_at"`
		} `json:"reservations"`
		Conflicts []struct {
			Holder  string `json:"holder"`
			Pattern string `json:"pattern"`
			Reason  string `json:"reason"`
		} `json:"conflicts"`
	}
	json.Unmarshal(resp, &data)

	if data.Status == "conflict" {
		fmt.Println("Conflicts detected:")
		for _, c := range data.Conflicts {
			line := fmt.Sprintf("  %s holds %s", c.Holder, c.Pattern)
			if c.Reason != "" {
				line += fmt.Sprintf(" (%s)", c.Reason)
			}
			fmt.Println(line)
		}
		return nil
	}

	for _, r := range data.Reservations {
		exp := ""
		if t, err := time.Parse(time.RFC3339, r.ExpiresAt); err == nil {
			exp = fmt.Sprintf(" (expires in %s)", time.Until(t).Round(time.Second))
		}
		fmt.Printf("Reserved #%d: %s%s\n", r.ID, r.Pattern, exp)
	}
	return nil
}

func reserveCheck(patterns []string) error {
	resp, err := serverPost("/api/reservations/check", map[string]interface{}{
		"patterns": patterns,
	})
	if err != nil {
		return err
	}

	if jsonOutput {
		fmt.Println(string(resp))
		return nil
	}

	var data struct {
		Clear     bool `json:"clear"`
		Conflicts []struct {
			Holder  string `json:"holder"`
			Pattern string `json:"pattern"`
			Reason  string `json:"reason"`
		} `json:"conflicts"`
	}
	json.Unmarshal(resp, &data)

	if data.Clear {
		fmt.Println("No conflicts — clear to proceed")
	} else {
		fmt.Println("Conflicts found:")
		for _, c := range data.Conflicts {
			line := fmt.Sprintf("  %s holds %s", c.Holder, c.Pattern)
			if c.Reason != "" {
				line += fmt.Sprintf(" (%s)", c.Reason)
			}
			fmt.Println(line)
		}
	}
	return nil
}

func reserveList() error {
	resp, err := serverGet("/api/reservations")
	if err != nil {
		return err
	}

	if jsonOutput {
		fmt.Println(string(resp))
		return nil
	}

	var data struct {
		Reservations []struct {
			ID        int64  `json:"id"`
			Holder    string `json:"holder"`
			Pattern   string `json:"pattern"`
			Reason    string `json:"reason"`
			Exclusive bool   `json:"exclusive"`
			ExpiresAt string `json:"expires_at"`
		} `json:"reservations"`
	}
	json.Unmarshal(resp, &data)

	if len(data.Reservations) == 0 {
		fmt.Println("No active reservations")
		return nil
	}

	for _, r := range data.Reservations {
		exp := ""
		if t, err := time.Parse(time.RFC3339, r.ExpiresAt); err == nil {
			remaining := time.Until(t).Round(time.Second)
			exp = fmt.Sprintf("  expires in %s", remaining)
		}
		reason := ""
		if r.Reason != "" {
			reason = fmt.Sprintf(" (%s)", r.Reason)
		}
		fmt.Printf("#%-4d %s → %s%s%s\n", r.ID, r.Holder, r.Pattern, reason, exp)
	}
	return nil
}

func reserveRelease(id int64) error {
	_, err := serverDelete(fmt.Sprintf("/api/reservations/%d", id))
	if err != nil {
		return err
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.Encode(map[string]interface{}{"status": "released", "id": id})
		return nil
	}

	fmt.Printf("Reservation #%d released\n", id)
	return nil
}

func init() {
	reserveCmd.Flags().Bool("check", false, "Check for conflicts without reserving")
	reserveCmd.Flags().Bool("list", false, "List active reservations")
	reserveCmd.Flags().Int64("release", 0, "Release reservation by ID")
	reserveCmd.Flags().String("reason", "", "Reason for reservation")
	reserveCmd.Flags().String("ttl", "30m", "Time-to-live (e.g., 30m, 1h)")
	rootCmd.AddCommand(reserveCmd)
}
