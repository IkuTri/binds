package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/IkuTri/binds/internal/server"
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
  binds serve                    # Start on default :8890
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

var serveConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Show binds server configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, _ := os.UserHomeDir()
		configDir := filepath.Join(home, ".config", "binds")
		configPath := filepath.Join(configDir, "config.toml")

		cfg, err := server.LoadConfigFile(configDir)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
			fmt.Printf("No config file at %s (using defaults)\n\n", configPath)
		} else {
			fmt.Printf("Config: %s\n\n", configPath)
		}

		port := cfg.Server.Port
		if port == 0 {
			port = 8890
		}
		listen := cfg.Server.Listen
		if listen == "" {
			listen = "127.0.0.1"
		}

		fmt.Printf("[server]\n")
		fmt.Printf("  port   = %d\n", port)
		fmt.Printf("  listen = %s\n", listen)
		fmt.Printf("\n[identity]\n")
		if cfg.Identity.Name != "" {
			fmt.Printf("  name = %s\n", cfg.Identity.Name)
		} else {
			fmt.Printf("  name = (not set)\n")
		}
		if cfg.Identity.AgentType != "" {
			fmt.Printf("  type = %s\n", cfg.Identity.AgentType)
		} else {
			fmt.Printf("  type = (not set)\n")
		}

		tokenPath := filepath.Join(configDir, ".local-token")
		if _, statErr := os.Stat(tokenPath); statErr == nil {
			fmt.Printf("\n[auth]\n")
			fmt.Printf("  local-token = %s\n", tokenPath)
		}

		return nil
	},
}

func init() {
	serveCmd.Flags().Int("port", 8890, "Port to listen on")
	serveCmd.Flags().String("listen", "127.0.0.1", "Address to bind (0.0.0.0 for all interfaces)")
	serveCmd.AddCommand(serveConfigCmd)
	rootCmd.AddCommand(serveCmd)
}
