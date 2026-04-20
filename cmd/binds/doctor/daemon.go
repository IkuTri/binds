package doctor

// ServerHealthResult holds the result of Dolt server mode health checks.
// Stub: server/daemon/federation checks removed in beads lobotomy.
type ServerHealthResult struct {
	Checks    []DoctorCheck `json:"checks"`
	OverallOK bool          `json:"overall_ok"`
}

// RunServerHealthChecks returns a stub result (Dolt server mode removed).
func RunServerHealthChecks(_ string) ServerHealthResult {
	return ServerHealthResult{
		OverallOK: true,
		Checks: []DoctorCheck{{
			Name:    "Server Mode",
			Status:  StatusOK,
			Message: "Dolt server mode not available in this build",
		}},
	}
}

// CheckGitSyncSetup checks git sync configuration.
// Stub: daemon/sync features removed.
func CheckGitSyncSetup(_ string) DoctorCheck {
	return DoctorCheck{
		Name:    "Git Sync Setup",
		Status:  StatusOK,
		Message: "n/a (daemon removed)",
	}
}

// CheckDaemonStatus checks if the beads daemon is running.
// Stub: daemon removed.
func CheckDaemonStatus(_ string, _ string) DoctorCheck {
	return DoctorCheck{
		Name:    "Daemon Status",
		Status:  StatusOK,
		Message: "n/a (daemon removed)",
	}
}

// CheckDaemonAutoSync checks if the daemon auto-sync is configured.
// Stub: daemon removed.
func CheckDaemonAutoSync(_ string) DoctorCheck {
	return DoctorCheck{
		Name:    "Daemon Auto-Sync",
		Status:  StatusOK,
		Message: "n/a (daemon removed)",
	}
}

// CheckLegacyDaemonConfig checks for deprecated daemon config options.
// Stub: daemon removed.
func CheckLegacyDaemonConfig(_ string) DoctorCheck {
	return DoctorCheck{
		Name:    "Legacy Daemon Config",
		Status:  StatusOK,
		Message: "n/a (daemon removed)",
	}
}

// CheckHydratedRepoDaemons checks if hydrated repo daemons are running.
// Stub: daemon removed.
func CheckHydratedRepoDaemons(_ string) DoctorCheck {
	return DoctorCheck{
		Name:    "Hydrated Repo Daemons",
		Status:  StatusOK,
		Message: "n/a (daemon removed)",
	}
}

// CheckFederationRemotesAPI checks Dolt remotesapi port accessibility.
// Stub: federation/Dolt removed.
func CheckFederationRemotesAPI(_ string) DoctorCheck {
	return DoctorCheck{
		Name:    "Federation RemotesAPI",
		Status:  StatusOK,
		Message: "n/a (federation removed)",
	}
}

// CheckFederationPeerConnectivity checks connectivity to federation peers.
// Stub: federation/Dolt removed.
func CheckFederationPeerConnectivity(_ string) DoctorCheck {
	return DoctorCheck{
		Name:    "Federation Peer Connectivity",
		Status:  StatusOK,
		Message: "n/a (federation removed)",
	}
}

// CheckFederationSyncStaleness checks for stale federation sync.
// Stub: federation/Dolt removed.
func CheckFederationSyncStaleness(_ string) DoctorCheck {
	return DoctorCheck{
		Name:    "Federation Sync Staleness",
		Status:  StatusOK,
		Message: "n/a (federation removed)",
	}
}

// CheckFederationConflicts checks for unresolved federation conflicts.
// Stub: federation/Dolt removed.
func CheckFederationConflicts(_ string) DoctorCheck {
	return DoctorCheck{
		Name:    "Federation Conflicts",
		Status:  StatusOK,
		Message: "n/a (federation removed)",
	}
}

// CheckDoltServerModeMismatch checks for Dolt init vs embedded mode mismatch.
// Stub: Dolt removed.
func CheckDoltServerModeMismatch(_ string) DoctorCheck {
	return DoctorCheck{
		Name:    "Dolt Server Mode",
		Status:  StatusOK,
		Message: "n/a (Dolt removed)",
	}
}

// CheckDoltLocks checks for uncommitted Dolt changes (lock files).
// Stub: Dolt removed.
func CheckDoltLocks(_ string) DoctorCheck {
	return DoctorCheck{
		Name:    "Dolt Locks",
		Status:  StatusOK,
		Message: "n/a (Dolt removed)",
	}
}

// MigrationValidationResult holds the result of a Dolt migration validation check.
// Stub: Dolt migration removed.
type MigrationValidationResult struct {
	Ready          bool     `json:"ready"`
	Backend        string   `json:"backend,omitempty"`
	JSONLCount     int      `json:"jsonl_count,omitempty"`
	SQLiteCount    int      `json:"sqlite_count,omitempty"`
	DoltCount      int      `json:"dolt_count,omitempty"`
	JSONLValid     bool     `json:"jsonl_valid,omitempty"`
	JSONLMalformed int      `json:"jsonl_malformed,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
	Errors         []string `json:"errors,omitempty"`
}

// CheckMigrationReadiness checks if the database is ready for Dolt migration.
// Stub: Dolt migration removed.
func CheckMigrationReadiness(_ string) (DoctorCheck, MigrationValidationResult) {
	return DoctorCheck{
		Name:    "Migration Readiness",
		Status:  StatusOK,
		Message: "n/a (Dolt migration removed)",
	}, MigrationValidationResult{Ready: true}
}

// CheckMigrationCompletion checks if a Dolt migration completed successfully.
// Stub: Dolt migration removed.
func CheckMigrationCompletion(_ string) (DoctorCheck, MigrationValidationResult) {
	return DoctorCheck{
		Name:    "Migration Completion",
		Status:  StatusOK,
		Message: "n/a (Dolt migration removed)",
	}, MigrationValidationResult{Ready: true}
}
