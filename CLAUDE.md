# CLAUDE.md — ClaudeLens

## Project Overview

ClaudeLens is a Go TUI application for discovering and resuming Claude Code
sessions. Built with Bubbletea (Elm architecture) + Lipgloss + Bubbles.

## Development Commands

```bash
go build -o claudelens .    # Build binary
go run .                     # Run directly
go test ./...                # Run all tests
go vet ./...                 # Static analysis
```

## Code Standards

### Generic Code Only
This is a public open-source project. Code and documentation must be completely
generic. Never include references to any specific company, product, or internal
system. No hardcoded paths — use `os.UserHomeDir()` for home directory.

### Go Conventions
- Keep functions under 50 lines
- Maximum 2-3 levels of nesting
- No `continue` or `break` — refactor logic instead
- Minimal comments — only where logic is genuinely complex
- Use `internal/` for non-exported packages

### Project Structure
- `internal/data/` — Data loading, parsing, API clients
- `internal/ui/` — Bubbletea models, views, styles
- `internal/terminal/` — OS-level terminal interaction
- `docs/` — Architecture and design references

### Security
- Never log or hardcode API keys, tokens, or credentials
- All external endpoints configurable via environment variables
- No secrets in code, comments, or commit messages
