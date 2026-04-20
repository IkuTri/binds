package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/steveyegge/beads/internal/server"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the binds coordination server",
	Long: `Start an embedded HTTP server for multi-agent coordination.

The server provides:
  - Agent registry with API token auth
  - Persistent mail between agents
  - Named rooms with threaded replies
  - Heartbeat-based presence tracking
  - Advisory file reservations

Data is stored in ~/.config/binds/server.db.
A local credential is written to ~/.config/binds/.local-token for CLI access.

Examples:
  binds serve                    # Start on default :8889
  binds serve --port 9000        # Custom port
  binds serve --listen 0.0.0.0   # Listen on all interfaces (team mode)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("user home: %w", err)
		}

		configDir := filepath.Join(home, ".config", "binds")
		cfg := server.DefaultConfig()
		cfg.ConfigDir = configDir

		fileCfg, err := server.LoadConfigFile(configDir)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		fileCfg.ApplyToServerConfig(cfg)

		if cmd.Flags().Changed("port") {
			cfg.Port, _ = cmd.Flags().GetInt("port")
		}
		if cmd.Flags().Changed("listen") {
			cfg.Listen, _ = cmd.Flags().GetString("listen")
		}

		srv, err := server.New(cfg)
		if err != nil {
			return fmt.Errorf("init server: %w", err)
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		fmt.Printf("binds serve starting on %s:%d\n", cfg.Listen, cfg.Port)
		return srv.Start(ctx)
	},
}

func init() {
	serveCmd.Flags().Int("port", 8889, "Port to listen on")
	serveCmd.Flags().String("listen", "127.0.0.1", "Address to bind (0.0.0.0 for all interfaces)")
	rootCmd.AddCommand(serveCmd)
}
