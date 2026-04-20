package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var mailCmd = &cobra.Command{
	Use:   "mail",
	Short: "Send and receive messages between agents",
	Long: `Persistent, searchable message passing between agents.

Requires a running binds server (binds serve).

Commands:
  binds mail send <recipient> <body>   Send a message
  binds mail inbox [--unread]          Check inbox
  binds mail read <id>                 Mark message read
  binds mail read-all                  Mark all read
  binds mail history --with <agent>    History with agent
  binds mail threads                   Threaded view
  binds mail status                    Mailbox stats`,
}

var mailSendCmd = &cobra.Command{
	Use:   "send <recipient> <body>",
	Short: "Send a message to an agent",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		recipient := args[0]
		body := strings.Join(args[1:], " ")
		subject, _ := cmd.Flags().GetString("subject")
		priority, _ := cmd.Flags().GetString("priority")
		msgType, _ := cmd.Flags().GetString("type")

		payload := map[string]interface{}{
			"recipient": recipient,
			"body":      body,
		}
		if subject != "" {
			payload["subject"] = subject
		}
		if priority != "" {
			payload["priority"] = priority
		}
		if msgType != "" {
			payload["msg_type"] = msgType
		}

		resp, err := serverPost("/api/mail", payload)
		if err != nil {
			return err
		}

		if jsonOutput {
			fmt.Println(string(resp))
			return nil
		}

		var msg struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(resp, &msg)
		fmt.Printf("Message #%d sent to %s\n", msg.ID, recipient)
		return nil
	},
}

var mailInboxCmd = &cobra.Command{
	Use:   "inbox",
	Short: "Check inbox",
	RunE: func(cmd *cobra.Command, args []string) error {
		unread, _ := cmd.Flags().GetBool("unread")
		limit, _ := cmd.Flags().GetInt("limit")

		params := fmt.Sprintf("?limit=%d", limit)
		if unread {
			params += "&unread_only=true"
		}

		resp, err := serverGet("/api/mail/inbox" + params)
		if err != nil {
			return err
		}

		if jsonOutput {
			fmt.Println(string(resp))
			return nil
		}

		var data struct {
			Messages []struct {
				ID        int64  `json:"id"`
				Sender    string `json:"sender"`
				Subject   string `json:"subject"`
				Body      string `json:"body"`
				Priority  string `json:"priority"`
				IsRead    bool   `json:"is_read"`
				CreatedAt string `json:"created_at"`
			} `json:"messages"`
		}
		if err := json.Unmarshal(resp, &data); err != nil {
			return err
		}

		if len(data.Messages) == 0 {
			fmt.Println("Inbox empty")
			return nil
		}

		for _, m := range data.Messages {
			read := " "
			if !m.IsRead {
				read = "●"
			}
			subj := m.Subject
			if subj == "" {
				subj = truncate(m.Body, 60)
			}
			ts := ""
			if t, err := time.Parse(time.RFC3339, m.CreatedAt); err == nil {
				ts = time.Since(t).Round(time.Second).String() + " ago"
			}
			fmt.Printf("%s #%-4d %-16s %s  %s\n", read, m.ID, m.Sender, subj, ts)
		}
		return nil
	},
}

var mailReadCmd = &cobra.Command{
	Use:   "read <id>",
	Short: "Mark a message as read",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := serverPatch("/api/mail/" + args[0] + "/read")
		if err != nil {
			return err
		}
		fmt.Printf("Message %s marked as read\n", args[0])
		return nil
	},
}

var mailReadAllCmd = &cobra.Command{
	Use:   "read-all",
	Short: "Mark all messages as read",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := serverPatch("/api/mail/read-all")
		if err != nil {
			return err
		}
		var data struct {
			Marked int `json:"marked"`
		}
		json.Unmarshal(resp, &data)
		fmt.Printf("%d messages marked as read\n", data.Marked)
		return nil
	},
}

var mailHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Message history with an agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		with, _ := cmd.Flags().GetString("with")
		limit, _ := cmd.Flags().GetInt("limit")

		if with == "" {
			return fmt.Errorf("--with <agent> required")
		}

		resp, err := serverGet(fmt.Sprintf("/api/mail/history?with=%s&limit=%d", with, limit))
		if err != nil {
			return err
		}

		if jsonOutput {
			fmt.Println(string(resp))
			return nil
		}

		return printMessages(resp)
	},
}

var mailThreadsCmd = &cobra.Command{
	Use:   "threads",
	Short: "Show threaded conversations",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := serverGet("/api/mail/threads")
		if err != nil {
			return err
		}

		if jsonOutput {
			fmt.Println(string(resp))
			return nil
		}

		return printMessages(resp)
	},
}

var mailStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Mailbox stats",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := serverGet("/api/mail/status")
		if err != nil {
			return err
		}

		var data struct {
			Total  int `json:"total"`
			Unread int `json:"unread"`
		}
		json.Unmarshal(resp, &data)
		fmt.Printf("Total: %d  Unread: %d\n", data.Total, data.Unread)
		return nil
	},
}

func init() {
	mailSendCmd.Flags().StringP("subject", "s", "", "Message subject")
	mailSendCmd.Flags().StringP("priority", "p", "normal", "Priority (urgent|normal|low)")
	mailSendCmd.Flags().StringP("type", "t", "text", "Message type")
	mailInboxCmd.Flags().Bool("unread", false, "Show only unread messages")
	mailInboxCmd.Flags().Int("limit", 20, "Maximum messages to show")
	mailHistoryCmd.Flags().String("with", "", "Agent to show history with")
	mailHistoryCmd.Flags().Int("limit", 20, "Maximum messages to show")

	mailCmd.AddCommand(mailSendCmd)
	mailCmd.AddCommand(mailInboxCmd)
	mailCmd.AddCommand(mailReadCmd)
	mailCmd.AddCommand(mailReadAllCmd)
	mailCmd.AddCommand(mailHistoryCmd)
	mailCmd.AddCommand(mailThreadsCmd)
	mailCmd.AddCommand(mailStatusCmd)

	rootCmd.AddCommand(mailCmd)
}

// --- Server HTTP helpers ---

func serverGet(path string) ([]byte, error) {
	client, base, err := serverClient()
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequest("GET", base+path, nil)
	addAuth(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("server unreachable (is 'binds serve' running?): %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func serverPost(path string, payload interface{}) ([]byte, error) {
	client, base, err := serverClient()
	if err != nil {
		return nil, err
	}
	data, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", base+path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	addAuth(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("server unreachable (is 'binds serve' running?): %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func serverPatch(path string) ([]byte, error) {
	client, base, err := serverClient()
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequest("PATCH", base+path, nil)
	addAuth(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("server unreachable (is 'binds serve' running?): %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func serverDelete(path string) ([]byte, error) {
	client, base, err := serverClient()
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequest("DELETE", base+path, nil)
	addAuth(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("server unreachable (is 'binds serve' running?): %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func printMessages(resp []byte) error {
	var data struct {
		Messages []struct {
			ID        int64  `json:"id"`
			Sender    string `json:"sender"`
			Recipient string `json:"recipient"`
			Subject   string `json:"subject"`
			Body      string `json:"body"`
			CreatedAt string `json:"created_at"`
			ThreadID  *int64 `json:"thread_id"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(resp, &data); err != nil {
		return err
	}

	if len(data.Messages) == 0 {
		fmt.Println("No messages")
		return nil
	}

	for _, m := range data.Messages {
		ts := ""
		if t, err := time.Parse(time.RFC3339, m.CreatedAt); err == nil {
			ts = time.Since(t).Round(time.Second).String() + " ago"
		}
		prefix := ""
		if m.ThreadID != nil {
			prefix = fmt.Sprintf("[thread:%d] ", *m.ThreadID)
		}
		subj := m.Subject
		if subj != "" {
			subj = " — " + subj
		}
		fmt.Printf("#%-4d %s→%s%s  %s%s\n", m.ID, m.Sender, m.Recipient, subj, prefix, ts)
		fmt.Printf("  %s\n\n", m.Body)
	}
	return nil
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func intFromStr(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}
