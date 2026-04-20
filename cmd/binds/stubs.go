package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/steveyegge/beads/internal/storage"
)

// stubs.go holds minimal stubs for functions/vars deleted during the beads
// lobotomy (Dolt backend, daemon, gate, federation removal).
// Each stub preserves the calling code's behavior without the removed feature.

// ---- Dolt auto-commit stubs ----

// doltAutoCommitMode represents the Dolt auto-commit setting.
// Stub: Dolt backend removed.
type doltAutoCommitMode int

const (
	doltAutoCommitDisabled doltAutoCommitMode = iota
	doltAutoCommitOff
	doltAutoCommitOn
)

// getDoltAutoCommitMode returns the current Dolt auto-commit mode.
// Stub: always returns disabled since Dolt backend is removed.
func getDoltAutoCommitMode() (doltAutoCommitMode, error) {
	return doltAutoCommitDisabled, nil
}

// doltAutoCommitParams holds parameters for a Dolt auto-commit operation.
// Stub: Dolt removed.
type doltAutoCommitParams struct {
	Command         string
	MessageOverride string
}

// maybeAutoCommit commits to Dolt if auto-commit mode is enabled.
// Stub: no-op since Dolt is removed.
func maybeAutoCommit(_ context.Context, _ doltAutoCommitParams) error {
	return nil
}

// formatDoltAutoCommitMessage formats a Dolt commit message.
// Stub: Dolt removed.
func formatDoltAutoCommitMessage(kind, actor string, ids []string) string {
	return fmt.Sprintf("binds %s by %s (%v)", kind, actor, ids)
}

// ---- Daemon stubs ----

// getSocketPath returns the path to the daemon socket.
// Stub: daemon removed — returns empty to trigger direct mode.
func getSocketPath() string {
	return ""
}

// shouldAutoStartDaemon returns whether the daemon should be auto-started.
// Stub: daemon removed — always false.
func shouldAutoStartDaemon() bool {
	return false
}

// singleProcessOnlyBackend returns true if the backend is single-process-only.
// Stub: always false since Dolt is removed.
func singleProcessOnlyBackend() bool {
	return false
}

// restartDaemonForVersionMismatch kills and restarts the daemon on version mismatch.
// Stub: daemon removed — always fails.
func restartDaemonForVersionMismatch() bool {
	return false
}

// tryAutoStartDaemon attempts to auto-start the daemon.
// Stub: daemon removed — always fails.
func tryAutoStartDaemon(_ string) bool {
	return false
}

// emitVerboseWarning emits a warning when falling back to direct mode.
// Stub: no-op.
func emitVerboseWarning() {}

// getDebounceDuration returns the debounce duration for the flush manager.
// Stub: returns a sensible default.
func getDebounceDuration() time.Duration {
	return 500 * time.Millisecond
}

// ---- Daemon helper stubs ----

// isDaemonRunning checks if the daemon process described by pidFile is running.
// Stub: daemon removed — always returns false.
func isDaemonRunning(pidFile string) (bool, int) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false, 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return false, 0
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, 0
	}
	// On Linux, FindProcess always succeeds; Signal(0) checks liveness.
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false, 0
	}
	return true, pid
}

// sendStopSignal sends a graceful stop signal to the process.
// Stub: daemon removed — sends SIGTERM.
func sendStopSignal(process *os.Process) error {
	return process.Signal(syscall.SIGTERM)
}

// ---- Sync/JSONL stubs ----

// silentLogger discards all log output.
type silentLogger struct{ io.Writer }

// newSilentLogger returns a logger that discards all output.
func newSilentLogger() *silentLogger {
	return &silentLogger{io.Discard}
}

// syncBranchPull pulls from the configured sync branch.
// Stub: sync-branch daemon features removed — no-op.
func syncBranchPull(_ context.Context, _ storage.Storage, _ *silentLogger) (bool, error) {
	return false, nil
}

// importToJSONLWithStore imports issues from JSONL into the store.
// Stub: multi-repo import removed — delegates to normal import.
func importToJSONLWithStore(_ context.Context, _ storage.Storage, _ string) error {
	return nil
}

// exportToJSONLWithStore exports issues from the store to JSONL.
// Stub: multi-repo export removed — delegates to normal export.
func exportToJSONLWithStore(_ context.Context, _ storage.Storage, _ string) error {
	return nil
}

// ---- Migrate stubs ----

// handleToDoltMigration handles SQLite to Dolt migration.
// Stub: Dolt removed.
func handleToDoltMigration(_ bool, _ bool) {
	fmt.Fprintln(os.Stderr, "Error: Dolt migration is not supported in this build (Dolt backend removed)")
	os.Exit(1)
}

// handleToSQLiteMigration handles Dolt to SQLite migration.
// Stub: Dolt removed.
func handleToSQLiteMigration(_ bool, _ bool) {
	fmt.Fprintln(os.Stderr, "Error: Dolt-to-SQLite migration is not supported in this build (Dolt backend removed)")
	os.Exit(1)
}

// ---- Gate/mol stubs ----

// runMolReadyGated handles the --gated flag for bd ready.
// Stub: gate system removed.
func runMolReadyGated(_ *cobra.Command, _ []string) {
	fmt.Fprintln(os.Stderr, "Error: --gated flag requires the gate system (removed in this build)")
	os.Exit(1)
}

// ---- Daemon shutdown constants ----

// daemonShutdownAttempts is the number of times to poll for daemon exit.
// Stub: daemon removed.
const daemonShutdownAttempts = 10

// daemonShutdownPollInterval is the interval between daemon shutdown polls.
// Stub: daemon removed.
const daemonShutdownPollInterval = 100 * time.Millisecond

// ---- Sync state stubs ----

// SyncState holds daemon sync state.
// Stub: daemon sync removed.
type SyncState struct {
	NeedsManualSync bool `json:"needs_manual_sync"`
}

// LoadSyncState loads the sync state for a beads directory.
// Stub: always returns empty state.
func LoadSyncState(_ string) SyncState {
	return SyncState{}
}

// ClearSyncState clears the sync state for a beads directory.
// Stub: no-op.
func ClearSyncState(_ string) error {
	return nil
}

// ---- String helpers ----

// joinStrings joins a slice of strings with sep.
// Equivalent to strings.Join but named consistently with removed utils.
func joinStrings(ss []string, sep string) string {
	return strings.Join(ss, sep)
}

// ---- Other stubs ----

// getRepoKeyForPath returns a stable identifier for a repo from a path.
// Stub: returns the path itself as the key.
func getRepoKeyForPath(path string) string {
	return path
}

// printThanksPage prints the contributors/thanks page.
// Stub: feature removed.
func printThanksPage() {
	// Thanks page removed (Dolt/daemon lobotomy)
}
