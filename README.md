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
- **One-keypress resume** — session loads in a new tab (iTerm2, Terminal.app,
  tmux) or right in the current tab (Warp)
- **Worktree-aware** — resolves the directory Claude Code actually indexed the
  session under, even when the session moved through git worktrees
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

### Resume behavior by terminal

| Terminal | Behavior |
|----------|----------|
| iTerm2 | Auto-runs in a new tab (AppleScript) |
| Terminal.app | Auto-runs in a new window (AppleScript) |
| tmux | Auto-runs in a new window (`tmux new-window`) |
| Warp | Resumes **in the current tab** (process replacement); falls back to a launch-config window, then clipboard |
| Other | Full `cd && claude --resume` command copied to clipboard |

Warp has no AppleScript support and its `warp://` URI scheme can't execute
commands, so ClaudeLens replaces its own process with `claude --resume` in
the session's directory instead — one keypress, no automation needed.

### Resume directory resolution

`claude --resume <id>` only finds a session when run from the directory whose
encoded form matches the session's storage folder under `~/.claude/projects/`
(every non-alphanumeric character becomes `-`). Sessions drift across
directories mid-conversation (e.g. git worktrees), so ClaudeLens scans each
session's recorded `cwd` values and picks the one that encodes to the actual
storage folder — not just the first directory seen.

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
