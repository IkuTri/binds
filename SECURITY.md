# Security Policy

## Reporting Security Issues

If you discover a security vulnerability in binds, please report it responsibly:

**GitHub**: [Open a private security advisory](https://github.com/IkuTri/binds/security/advisories/new)

Please include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

We will respond within 48 hours and work with you to address the issue.

## Security Considerations

### Database Security

binds stores issue data locally in:
- SQLite databases (`.binds/*.db`) - local only, gitignored
- JSONL files (`.binds/issues.jsonl`) - committed to git

**Important**:
- Do not store sensitive information (passwords, API keys, secrets) in issue descriptions or metadata
- Issue data is committed to git and will be visible to anyone with repository access
- binds does not encrypt data at rest (it's a local development tool)

### Coordination Server Security

The binds coordination server (`binds serve`) provides multi-agent coordination:
- By default listens on `127.0.0.1` (localhost only)
- Token-based auth via `~/.config/binds/.local-token`
- When exposing on LAN (`listen = "0.0.0.0"`), ensure your network is trusted

### Git Workflow Security

- binds uses standard git operations (no custom protocols)
- Export/import operations read and write local files only
- No network communication except through git itself and the coordination server
- Git hooks (if used) run with your local user permissions

### Command Injection Protection

binds uses parameterized SQL queries to prevent SQL injection. However:
- Do not pass untrusted input directly to `binds` commands
- Issue IDs are validated against the pattern `^[a-z0-9-]+$`
- File paths are validated before reading/writing

### Dependency Security

binds has minimal dependencies:
- Go standard library
- SQLite (via ncruces/go-sqlite3 - wazero-based)
- Cobra CLI framework
- TOML config parsing

All dependencies are regularly updated. Run `go mod verify` to check integrity.

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |

## Best Practices

1. **Don't commit secrets** - Never put API keys, passwords, or credentials in issue descriptions
2. **Review before export** - Check `.binds/issues.jsonl` before committing sensitive project details
3. **Use private repos** - If your issues contain proprietary information, use private git repositories
4. **Validate git hooks** - If using automated export/import hooks, review them for safety
5. **Regular updates** - Keep binds updated to the latest version

## Known Limitations

- binds is designed for **development/internal use**, not production secret management
- Issue data is stored in plain text (both SQLite and JSONL)
- No built-in encryption or access control (relies on filesystem permissions)
- No audit logging beyond git history

For sensitive workflows, consider using binds only for non-sensitive task tracking.

## Security Updates

Security updates will be announced via:
- GitHub Security Advisories
- Release notes on GitHub
- Git commit messages (tagged with `[security]`)

Subscribe to the repository for notifications.
