package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/IkuTri/binds/internal/beads"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push local issues to the binds coordination server",
	Long: `Sync local issue database to the binds server for cross-repo visibility.

Reads all issues from the local .binds/ (or .beads/) database and pushes
them to the binds server's cross-repo index. Uses upsert semantics —
existing issues are updated, new ones are created.

Requires a running binds server (binds serve) and a local beads database.

Examples:
  binds push              # Push all issues from current workspace
  binds push --status open # Push only open issues`,
	RunE: func(cmd *cobra.Command, args []string) error {
		statusFilter, _ := cmd.Flags().GetString("status")

		beadsDir := beads.FindBeadsDir()
		if beadsDir == "" {
			return fmt.Errorf("no .binds/ or .binds/ directory found")
		}

		cwd, _ := os.Getwd()
		workspace := inferWorkspace(cwd)
		workspacePath := cwd

		dbFiles, _ := filepath.Glob(filepath.Join(beadsDir, "*.db"))
		if len(dbFiles) == 0 {
			return fmt.Errorf("no database found in %s", beadsDir)
		}
		dbPath := dbFiles[0]

		issues, err := readLocalIssues(dbPath, statusFilter)
		if err != nil {
			return fmt.Errorf("read local issues: %w", err)
		}

		if len(issues) == 0 {
			fmt.Println("No issues to push")
			return nil
		}

		for i := range issues {
			issues[i]["workspace"] = workspace
			issues[i]["workspace_path"] = workspacePath
		}

		payload := map[string]interface{}{
			"issues": issues,
		}

		resp, err := serverPost("/api/issues/push", payload)
		if err != nil {
			return err
		}

		if jsonOutput {
			fmt.Println(string(resp))
			return nil
		}

		var result struct {
			Pushed int `json:"pushed"`
		}
		json.Unmarshal(resp, &result)
		fmt.Printf("Pushed %d issues from %s\n", result.Pushed, workspace)
		return nil
	},
}

func inferWorkspace(cwd string) string {
	parts := strings.Split(cwd, string(filepath.Separator))
	for i, p := range parts {
		if p == "IkuSoft-Drive" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return filepath.Base(cwd)
}

func readLocalIssues(dbPath, statusFilter string) ([]map[string]interface{}, error) {
	connStr := "file:" + dbPath + "?_pragma=journal_mode(wal)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := `SELECT id, title, status, priority, issue_type, assignee, created_at, updated_at, closed_at FROM issues WHERE deleted_at IS NULL`
	args := []interface{}{}
	if statusFilter != "" {
		query += ` AND status = ?`
		args = append(args, statusFilter)
	}
	query += ` ORDER BY priority ASC, updated_at DESC`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []map[string]interface{}
	for rows.Next() {
		var id, title, status, issueType string
		var priority int
		var assignee, createdAt, updatedAt, closedAt sql.NullString

		if err := rows.Scan(&id, &title, &status, &priority, &issueType, &assignee, &createdAt, &updatedAt, &closedAt); err != nil {
			return nil, err
		}

		issue := map[string]interface{}{
			"issue_id":   id,
			"title":      title,
			"status":     status,
			"priority":   priority,
			"issue_type": issueType,
		}
		if assignee.Valid {
			issue["assignee"] = assignee.String
		}
		if createdAt.Valid {
			issue["created_at"] = createdAt.String
		}
		if updatedAt.Valid {
			issue["updated_at"] = updatedAt.String
		}
		if closedAt.Valid {
			issue["closed_at"] = closedAt.String
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

func init() {
	pushCmd.Flags().String("status", "", "Only push issues with this status")
	rootCmd.AddCommand(pushCmd)
}
