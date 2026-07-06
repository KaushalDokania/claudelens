# ClaudeLens — Design Document

## Problem

Claude Code's `/rename` command stores session names in ephemeral PID files
(`~/.claude/sessions/{pid}.json`), but the `/resume` picker only searches
`sessions-index.json` which lacks the `name` field. This is a confirmed bug
(anthropics/claude-code#26249, #43963, #31394, #25090).

Result: renamed sessions are invisible to the resume picker.

## Solution

A terminal UI that provides layered search across all Claude Code session data
sources, with conversation preview and direct resume capability.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                   ClaudeLens TUI                    │
│            (Go + Bubbletea + Lipgloss)              │
├─────────────────────────────────────────────────────┤
│                  Search Engine                      │
│  ┌──────────┐  ┌──────────────────────────────┐    │
│  │  Index    │  │  claude-mem Semantic Search   │    │
│  │  Search   │  │  (optional, HTTP :37777)     │    │
│  │  (fast)   │  │                              │    │
│  └────┬─────┘  └──────────────┬───────────────┘    │
│       │                       │                     │
│  sessions-index.json     /api/summaries             │
│  sessions/{pid}.json     /api/search/sessions       │
│                          /api/session/:id           │
└──────────────────────┬──────────────────────────────┘
                       │
              claude --resume <id>
```

## Data Sources

### Session Index (Primary — Fast)

Path: `~/.claude/projects/{projectPath}/sessions-index.json`

Fields: sessionId, summary, firstPrompt, messageCount, created, modified,
gitBranch, projectPath, fullPath

### Session Metadata (For Names)

Path: `~/.claude/sessions/{pid}.json`

Fields: pid, sessionId, cwd, startedAt, name (optional — only if renamed)

### claude-mem API (Optional — Semantic Search)

Endpoint: `http://localhost:37777`

#### Structured Data Endpoints

| Endpoint | Returns |
|----------|---------|
| `GET /api/summaries?limit=N&offset=N` | Paginated session summaries |
| `GET /api/session/:id` | Single session detail |
| `GET /api/projects` | Project list |
| `POST /api/observations/batch` | Observation details by IDs |
| `GET /api/health` | Worker status |

#### Search Endpoints (return markdown text with IDs)

| Endpoint | Returns |
|----------|---------|
| `GET /api/search/sessions?query=Q&limit=N` | Session matches |
| `GET /api/search/observations?query=Q&limit=N` | Observation matches |
| `GET /api/search/prompts?query=Q&limit=N` | Prompt matches |

### Conversation Files (For Preview)

Path: `~/.claude/projects/{projectPath}/{sessionId}.jsonl`

JSONL format with message role, content blocks, timestamps, tool calls.

## Search Strategy

1. **Index search** (<10ms) — fuzzy match on summary, firstPrompt, name,
   branch in the in-memory session list
2. **Semantic search** (~200ms, async) — query claude-mem API, merge results
   below index results
3. **Content search** (on-demand) — grep JSONL files for the search term

## Project Path Resolution

`claude --resume` scopes its lookup to `~/.claude/projects/<encoded-cwd>/`,
where the encoding replaces every character outside `[A-Za-z0-9]` with `-`
(case preserved, runs not collapsed — so `/.claude` encodes to `--claude`).
The encoding is lossy: folder names cannot be decoded back into paths.

A session's JSONL records a per-line `cwd` that drifts as the conversation
moves through directories (worktrees, subrepos). The first cwd seen is NOT
reliable — in worktree sessions the storage-matching cwd can first appear
hundreds of lines in. Resolution algorithm (`internal/data/loader.go`):

1. Scan cwd values (unbounded, early exit) for one whose encoding matches
   the JSONL's parent folder name — that is the only resumable directory
2. Other metadata (prompt/summary/branch/created) keeps the 30-line fast path
3. No match at EOF → fall back to the most frequent cwd (never empty)

Index-sourced sessions (`sessions-index.json`) get the same validation via
`resolveIndexProjectPath`.

## Resume Action

| Terminal | Method |
|----------|--------|
| Warp | 1. In-place: `chdir` + `syscall.Exec` replaces the process with `claude --resume` in the current tab · 2. Fallback: tab config (`~/.warp/tab_configs/claudelens_resume.toml` + `warp://tab_config/`) auto-runs in a new tab of the active window · 3. Fallback: clipboard |
| iTerm2 | AppleScript new tab |
| Terminal.app | AppleScript do script |
| tmux | tmux new-window |
| Fallback | Copy full `cd && claude --resume` to clipboard |

Warp constraints (why the ladder exists): no AppleScript support; the
`warp://action/new_tab` URI accepts only `path=`, never a command
(warpdotdev/warp#3959, #2110). Tab configs are the one Warp mechanism
whose `commands` run automatically in a tab of the active window (launch
configurations do too, but always open a new window — Warp docs mark
them legacy).

## Multi-File Sessions

Resuming a session from a different directory makes Claude Code write a
new transcript file under that directory's encoded folder — one session
ID can have files in several project folders, and a continuation file may
never mention its own launch directory in any `cwd` line (Claude Code
restores the previous session cwd on resume). `Session.PathVerified`
records whether encode-matching succeeded; the loader dedupes by session
ID keeping verified entries over fallbacks, then most recent.
