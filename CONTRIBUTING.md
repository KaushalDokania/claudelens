# Contributing to ClaudeLens

Thanks for your interest in contributing! Here's how to get started.

## Development Setup

```bash
git clone https://github.com/KaushalDokania/claudelens.git
cd claudelens
go build .
./claudelens
```

**Requirements**: Go 1.21+ and a terminal emulator.

## Making Changes

1. Fork the repo and create a feature branch
2. Make your changes
3. Run `go vet ./...` to check for issues
4. Run `go build .` to verify it compiles
5. Test the TUI manually with `./claudelens`
6. Submit a pull request

## Code Style

- Keep functions under 50 lines
- Maximum 2-3 levels of nesting
- Use `internal/` packages — nothing is exported outside the module
- Minimal comments — only where logic is genuinely complex

## Project Structure

```
internal/data/       — Data loading, parsing, API clients
internal/ui/         — Bubbletea models, views, styles
internal/terminal/   — OS-level terminal interaction
```

## Reporting Issues

Open an issue on GitHub with:
- What you expected to happen
- What actually happened
- Your OS and terminal emulator
- Output of `claudelens --version`

## License

By contributing, you agree that your contributions will be licensed under the
MIT License.
