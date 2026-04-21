package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/IkuTri/binds/internal/server"
)

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Workspace management commands",
	Long:  `Commands for inspecting binds workspace configuration.`,
}

var workspaceCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Print the workspace name for the current directory",
	Long: `Check if the current working directory is inside a registered workspace.

Prints the workspace name (basename of the matching path) if found.
Exits silently with code 1 if the cwd is not inside any workspace.

Workspace paths are read from ~/.config/binds/config.toml [workspaces] paths.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("user home: %w", err)
		}

		configDir := filepath.Join(home, ".config", "binds")
		cfg, err := server.LoadConfigFile(configDir)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get cwd: %w", err)
		}

		for _, wpath := range cfg.Workspaces.Paths {
			// Expand ~ in workspace paths.
			if strings.HasPrefix(wpath, "~/") {
				wpath = filepath.Join(home, wpath[2:])
			}
			// Resolve to absolute path.
			abs, err := filepath.Abs(wpath)
			if err != nil {
				continue
			}
			// Ensure trailing separator for prefix check.
			prefix := abs
			if !strings.HasSuffix(prefix, string(filepath.Separator)) {
				prefix += string(filepath.Separator)
			}
			if cwd == abs || strings.HasPrefix(cwd, prefix) {
				fmt.Println(filepath.Base(abs))
				return nil
			}
		}

		// No match — exit 1 silently.
		os.Exit(1)
		return nil
	},
}

func init() {
	workspaceCmd.AddCommand(workspaceCurrentCmd)
	rootCmd.AddCommand(workspaceCmd)
}
