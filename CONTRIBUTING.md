# Contributing to binds

Thank you for your interest in contributing to binds! This document provides guidelines and instructions for contributing.

## Development Setup

### Prerequisites

- Go 1.24 or later
- Git
- (Optional) golangci-lint for local linting

### Getting Started

```bash
# Clone the repository
git clone https://github.com/IkuTri/binds
cd binds

# Build the project
go build -o binds ./cmd/binds

# Run tests
go test ./...

# Run with race detection
go test -race ./...

# Build and install locally
go install ./cmd/binds
```

## Project Structure

```
binds/
├── cmd/binds/           # CLI entry point and commands
├── internal/
│   ├── types/           # Core data types (Issue, Dependency, etc.)
│   ├── storage/         # Storage interface and implementations
│   │   └── sqlite/      # SQLite backend
│   └── server/          # Coordination server (mail, registry, rooms)
├── .golangci.yml        # Linter configuration
└── .github/workflows/   # CI/CD pipelines
```

## Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific package tests
go test ./internal/storage/sqlite -v

# Run tests with race detection
go test -race ./...
```

## Code Style

We follow standard Go conventions:

- Use `gofmt` to format your code (runs automatically in most editors)
- Follow the [Effective Go](https://golang.org/doc/effective_go) guidelines
- Keep functions small and focused
- Write clear, descriptive variable names
- Add comments for exported functions and types

### Linting

We use golangci-lint for code quality checks:

```bash
# Install golangci-lint
brew install golangci-lint  # macOS
# or
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
golangci-lint run ./...
```

**Note**: The linter currently reports ~100 warnings. These are documented false positives and idiomatic Go patterns (deferred cleanup, Cobra interface requirements, etc.). See [docs/LINTING.md](docs/LINTING.md) for details. When contributing, focus on avoiding *new* issues rather than the baseline warnings.

CI will automatically run linting on all pull requests.

## Making Changes

### Workflow

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Make your changes
4. Add tests for new functionality
5. Run tests and linter locally
6. Commit your changes with clear messages
7. Push to your fork
8. Open a pull request

### Commit Messages

Write clear, concise commit messages:

```
Add cycle detection for dependency graphs

- Implement recursive CTE-based cycle detection
- Add tests for simple and complex cycles
- Update documentation with examples
```

### Important: Don't Include .binds/issues.jsonl Changes

The `.binds/issues.jsonl` file is the project's issue database. **Do not include changes to this file in your PR.** CI will fail if this file is modified.

If you accidentally committed changes to this file, fix it with:

```bash
git checkout origin/main -- .binds/issues.jsonl
git commit --amend
git push --force
```

### Pull Requests

- Keep PRs focused on a single feature or fix
- Include tests for new functionality
- Update documentation as needed
- Ensure CI passes before requesting review
- Respond to review feedback promptly
- **Do not include `.binds/issues.jsonl` changes** (see above)

## Testing Guidelines

### Test Strategy

We use a two-tier testing approach:

- **Fast tests** (unit tests): Run on every PR via CI with `-short` flag (~2s)
- **Slow tests** (integration tests): Run nightly with full git operations (~14s)

Slow tests use `testing.Short()` to skip when `-short` flag is present.

### Running Tests

```bash
# Fast tests (recommended for development - skips slow tests)
go test -short ./...

# Full test suite (before committing - includes all tests)
go test ./...

# With race detection and coverage
go test -race -coverprofile=coverage.out ./...
```

### Writing Tests

- Write table-driven tests when testing multiple scenarios
- Use descriptive test names that explain what is being tested
- Clean up resources (database files, etc.) in test teardown
- Use `t.Run()` for subtests to organize related test cases
- Mark slow tests with `if testing.Short() { t.Skip("slow test") }`

## Documentation

- Update README.md for user-facing changes
- Update relevant .md files in the project root
- Add inline code comments for complex logic
- Include examples in documentation

## Feature Requests and Bug Reports

### Reporting Bugs

Include in your bug report:
- Steps to reproduce
- Expected behavior
- Actual behavior
- Version of binds (`binds version`)
- Operating system and Go version

### Feature Requests

When proposing new features:
- Explain the use case
- Describe the proposed solution
- Consider backwards compatibility
- Discuss alternatives you've considered

## Code Review Process

All contributions go through code review:

1. Automated checks (tests, linting) must pass
2. At least one maintainer approval required
3. Address review feedback
4. Maintainer will merge when ready

## Development Tips

### Testing Locally

```bash
# Build and test your changes quickly
go build -o binds ./cmd/binds && ./binds init --prefix test

# Test specific functionality
./binds create "Test issue" -p 1 -t bug
./binds dep add test-2 test-1
./binds ready
```

### Database Inspection

```bash
# Inspect the SQLite database directly
sqlite3 .binds/beads.db

# Useful queries
SELECT * FROM issues;
SELECT * FROM dependencies;
SELECT * FROM events WHERE issue_id = 'test-1';
```

### Debugging

Use Go's built-in debugging tools:

```bash
# Run with verbose logging
go run ./cmd/binds -v create "Test"

# Use delve for debugging
dlv debug ./cmd/binds -- create "Test issue"
```

## Release Process

(For maintainers)

1. Update version in code
2. Update CHANGELOG.md
3. Tag release: `git tag v0.x.0`
4. Push tag: `git push origin v0.x.0`
5. GitHub Actions will build and publish

## Questions?

- Check existing [issues](https://github.com/IkuTri/binds/issues)
- Open a new issue for questions
- Review [README.md](README.md) and other documentation

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

## Code of Conduct

Be respectful and professional in all interactions. We're here to build something great together.
