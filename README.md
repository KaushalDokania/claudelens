# ClaudeLens

Fast terminal UI for discovering and resuming [Claude Code](https://claude.ai/code) sessions.

## The Problem

Claude Code's built-in `/resume` picker has limited search capabilities. If you
rename sessions with `/rename`, the names aren't properly indexed for search
([known issue](https://github.com/anthropics/claude-code/issues/26249)). Finding
a specific past session — especially across multiple projects — is frustrating.

## Features

- **Layered search** — instant index search + optional semantic search via
  [claude-mem](https://www.npmjs.com/package/claude-mem)
- **Cross-project browsing** — see sessions from all your projects in one view
- **Conversation preview** — read the conversation before committing to a resume
- **One-keypress resume** — opens a new terminal tab with the session loaded
- **Clipboard copy** — grab the `claude --resume <id>` command if you prefer

## Installation

```bash
go install github.com/KaushalDokania/claudelens@latest
```

Or build from source:

```bash
git clone https://github.com/KaushalDokania/claudelens.git
cd claudelens
go build -o claudelens .
```

## Usage

```bash
claudelens                            # Launch TUI
claudelens --search "auth refactor"   # Pre-fill search query
claudelens --project myapp            # Filter to a specific project
claudelens --recent 7d                # Only sessions from last 7 days
claudelens --no-claude-mem            # Disable semantic search
```

## Key Bindings

| Key | Action |
|-----|--------|
| `↑↓` or `jk` | Navigate session list |
| `Enter` | Resume session (opens new terminal tab) |
| `c` | Copy resume command to clipboard |
| `/` | Focus search input |
| `s` | Toggle semantic search |
| `Tab` | Toggle focus between list and preview |
| `?` | Help overlay |
| `q` | Quit |

## How It Works

ClaudeLens reads session data from Claude Code's local storage (`~/.claude/`)
and optionally queries [claude-mem](https://www.npmjs.com/package/claude-mem)'s
HTTP API for richer semantic search across session history.

### Data Sources

1. **Session index** (`~/.claude/projects/*/sessions-index.json`) — session
   summaries, timestamps, message counts, git branches
2. **Active sessions** (`~/.claude/sessions/*.json`) — session names from
   `/rename` (for currently active sessions)
3. **claude-mem API** (optional, `localhost:37777`) — AI-generated session
   summaries with request/learned/completed fields, semantic search
4. **Conversation files** (`~/.claude/projects/*/*.jsonl`) — full conversation
   content for the preview pane

## Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `CLAUDELENS_MEM_URL` | `http://localhost:37777` | claude-mem API endpoint |
| `CLAUDELENS_CLAUDE_DIR` | `~/.claude` | Claude Code config directory |

## License

[MIT](LICENSE)
