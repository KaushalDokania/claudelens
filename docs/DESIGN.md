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

## Resume Action

| Terminal | Method |
|----------|--------|
| iTerm2 | AppleScript new tab |
| Terminal.app | AppleScript do script |
| tmux | tmux new-window |
| Fallback | Copy to clipboard |
