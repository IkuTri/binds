package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var roomsCmd = &cobra.Command{
	Use:   "rooms",
	Short: "Named chat rooms for multi-agent planning",
	Long: `Create and use named chat channels with threaded replies.

Requires a running binds server (binds serve).

Commands:
  binds rooms create <name>          Create a room
  binds rooms list                   List active rooms
  binds rooms read <name>            Read room messages
  binds rooms post <name> <body>     Post to a room
  binds rooms archive <name>         Archive a room`,
}

var roomsCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a named room",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		topic, _ := cmd.Flags().GetString("topic")
		resp, err := serverPost("/api/rooms", map[string]interface{}{
			"name":  args[0],
			"topic": topic,
		})
		if err != nil {
			return err
		}
		if jsonOutput {
			fmt.Println(string(resp))
			return nil
		}
		fmt.Printf("Room '%s' created\n", args[0])
		return nil
	},
}

var roomsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active rooms",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := serverGet("/api/rooms")
		if err != nil {
			return err
		}
		if jsonOutput {
			fmt.Println(string(resp))
			return nil
		}

		var data struct {
			Rooms []struct {
				Name      string `json:"name"`
				Topic     string `json:"topic"`
				CreatedBy string `json:"created_by"`
				CreatedAt string `json:"created_at"`
			} `json:"rooms"`
		}
		json.Unmarshal(resp, &data)

		if len(data.Rooms) == 0 {
			fmt.Println("No active rooms")
			return nil
		}

		for _, r := range data.Rooms {
			line := r.Name
			if r.Topic != "" {
				line += " — " + r.Topic
			}
			fmt.Println(line)
		}
		return nil
	},
}

var roomsReadCmd = &cobra.Command{
	Use:   "read <name>",
	Short: "Read room messages",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		since, _ := cmd.Flags().GetString("since")
		limit, _ := cmd.Flags().GetInt("limit")

		params := fmt.Sprintf("?limit=%d", limit)
		if since != "" {
			params += "&since=" + since
		}

		resp, err := serverGet("/api/rooms/" + args[0] + params)
		if err != nil {
			return err
		}

		if jsonOutput {
			fmt.Println(string(resp))
			return nil
		}

		var data struct {
			Name     string `json:"name"`
			Topic    string `json:"topic"`
			Messages []struct {
				ID        int64  `json:"id"`
				Sender    string `json:"sender"`
				Body      string `json:"body"`
				CreatedAt string `json:"created_at"`
				ThreadID  *int64 `json:"thread_id"`
				ReplyTo   *int64 `json:"reply_to"`
			} `json:"messages"`
		}
		json.Unmarshal(resp, &data)

		header := fmt.Sprintf("# %s", data.Name)
		if data.Topic != "" {
			header += " — " + data.Topic
		}
		fmt.Println(header)
		fmt.Println()

		if len(data.Messages) == 0 {
			fmt.Println("  (no messages)")
			return nil
		}

		for _, m := range data.Messages {
			ts := ""
			if t, err := time.Parse(time.RFC3339, m.CreatedAt); err == nil {
				ts = t.Format("15:04")
			}
			indent := ""
			if m.ReplyTo != nil {
				indent = "  ↳ "
			}
			fmt.Printf("  %s[%s] %s%s\n", indent, ts, m.Sender, ":")
			for _, line := range strings.Split(m.Body, "\n") {
				fmt.Printf("    %s%s\n", indent, line)
			}
		}
		return nil
	},
}

var roomsPostCmd = &cobra.Command{
	Use:   "post <name> <body>",
	Short: "Post a message to a room",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		body := strings.Join(args[1:], " ")
		replyTo, _ := cmd.Flags().GetInt64("reply-to")

		payload := map[string]interface{}{
			"body": body,
		}
		if replyTo > 0 {
			payload["reply_to"] = replyTo
		}

		resp, err := serverPost("/api/rooms/"+name+"/send", payload)
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
		fmt.Printf("Posted #%d to %s\n", msg.ID, name)
		return nil
	},
}

var roomsArchiveCmd = &cobra.Command{
	Use:   "archive <name>",
	Short: "Archive a room",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := serverDelete("/api/rooms/" + args[0])
		if err != nil {
			return err
		}

		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.Encode(map[string]string{"status": "archived", "name": args[0]})
			return nil
		}

		fmt.Printf("Room '%s' archived\n", args[0])
		return nil
	},
}

func init() {
	roomsCreateCmd.Flags().String("topic", "", "Room topic")
	roomsReadCmd.Flags().String("since", "", "Only show messages after this time (RFC3339)")
	roomsReadCmd.Flags().Int("limit", 50, "Maximum messages to show")
	roomsPostCmd.Flags().Int64("reply-to", 0, "Reply to message ID")

	roomsCmd.AddCommand(roomsCreateCmd)
	roomsCmd.AddCommand(roomsListCmd)
	roomsCmd.AddCommand(roomsReadCmd)
	roomsCmd.AddCommand(roomsPostCmd)
	roomsCmd.AddCommand(roomsArchiveCmd)

	rootCmd.AddCommand(roomsCmd)
}
