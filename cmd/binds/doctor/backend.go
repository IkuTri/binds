package doctor

import (
	"path/filepath"

	"github.com/IkuTri/binds/internal/configfile"
)

// getBackendAndBeadsDir resolves the effective .beads directory (following redirects)
// and returns the configured storage backend ("sqlite" by default, or "dolt").
func getBackendAndBeadsDir(repoPath string) (backend string, beadsDir string) {
	beadsDir = resolveBeadsDir(filepath.Join(repoPath, ".binds"))

	cfg, err := configfile.Load(beadsDir)
	if err != nil || cfg == nil {
		return configfile.BackendSQLite, beadsDir
	}
	return cfg.GetBackend(), beadsDir
}
