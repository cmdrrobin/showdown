# Agents Guide for Showdown

A file for [guiding coding agents](https://agents.md/).

## Build Commands

- **Build**: `go build -o showdown .` (produces `showdown` binary)
- **Build with version**: `go build -ldflags "-X main.Version=$(git describe --tags --always) -X main.CommitSHA=$(git rev-parse --short HEAD)" -o showdown .`
- **Run**: `go run .` or `./showdown` 
- **Test**: `go test ./...` (run all tests) or `go test -v ./...` (verbose)
- **Single test**: `go test -run TestFunctionName`
- **Lint**: Use `golangci-lint run` (if available) or `go vet ./...`

### Version Information

The application includes build-time version information:

- `Version`: Set from git tags using `git describe --tags --always`
- `CommitSHA`: Set from git commit hash using `git rev-parse --short HEAD`

These are injected at build time using Go's `-ldflags` and appear in the startup logs.

## Code Style Guidelines

- **Package**: Single package `main` for this CLI application
- **Imports**: Standard library first, then external packages (charmbracelet/*)
- **Constants**: Use catppuccin color constants, group related constants
- **Types**: PascalCase for exported, camelCase for unexported
- **Functions**: PascalCase for exported, camelCase for unexported  
- **Variables**: camelCase, descriptive names (e.g., `state`, `masterConn`)
- **Error handling**: Check errors explicitly, use `log.Error()` for logging
- **Concurrency**: Use `sync.RWMutex` for state protection, proper locking patterns
- **Style**: Use `lipgloss.NewStyle()` for UI styling, follow existing color scheme
- **Comments**: Minimal comments, prefer self-documenting code
- **SSH**: Use charmbracelet/ssh and wish middleware for terminal apps
