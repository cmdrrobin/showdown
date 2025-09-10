# Agents Guide for Showdown

A file for [guiding coding agents](https://agents.md/).

## Build Commands

- **Build**: `go build -o showdown .` (produces `showdown` binary)
- **Run**: `go run .` or `./showdown` 
- **Test**: `go test ./...` (run all tests) or `go test -v ./...` (verbose)
- **Single test**: `go test -run TestFunctionName`
- **Lint**: Use `golangci-lint run` (if available) or `go vet ./...`

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
