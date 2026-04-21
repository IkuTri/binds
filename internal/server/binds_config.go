package server

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// BindsConfig represents ~/.config/binds/config.toml.
type BindsConfig struct {
	Server struct {
		Port   int    `toml:"port"`
		Listen string `toml:"listen"`
	} `toml:"server"`
	Identity struct {
		Name      string `toml:"name"`
		AgentType string `toml:"type"`
	} `toml:"identity"`
	Workspaces struct {
		Paths []string `toml:"paths"`
	} `toml:"workspaces"`
}

// LoadConfigFile reads config.toml from configDir. Returns zero-value config if missing.
func LoadConfigFile(configDir string) (*BindsConfig, error) {
	cfg := &BindsConfig{}
	path := filepath.Join(configDir, "config.toml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// ApplyToServerConfig merges TOML values into a server Config.
func (bc *BindsConfig) ApplyToServerConfig(cfg *Config) {
	if bc.Server.Port > 0 {
		cfg.Port = bc.Server.Port
	}
	if bc.Server.Listen != "" {
		cfg.Listen = bc.Server.Listen
	}
	if bc.Identity.Name != "" {
		cfg.LocalIdentity = bc.Identity.Name
	}
	if bc.Identity.AgentType != "" {
		cfg.LocalAgentType = bc.Identity.AgentType
	}
}
