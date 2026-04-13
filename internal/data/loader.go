package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// sessionIndexFile represents the structure of sessions-index.json.
type sessionIndexFile struct {
	Version int                `json:"version"`
	Entries []sessionIndexEntry `json:"entries"`
}

type sessionIndexEntry struct {
	SessionID    string `json:"sessionId"`
	FullPath     string `json:"fullPath"`
	FileMtime    int64  `json:"fileMtime"`
	FirstPrompt  string `json:"firstPrompt"`
	Summary      string `json:"summary"`
	MessageCount int    `json:"messageCount"`
	Created      string `json:"created"`
	Modified     string `json:"modified"`
	GitBranch    string `json:"gitBranch"`
	ProjectPath  string `json:"projectPath"`
	IsSidechain  bool   `json:"isSidechain"`
}

// activeSessionFile represents ~/.claude/sessions/{pid}.json.
type activeSessionFile struct {
	PID       int    `json:"pid"`
	SessionID string `json:"sessionId"`
	CWD       string `json:"cwd"`
	StartedAt int64  `json:"startedAt"`
	Name      string `json:"name"`
}

// LoadSessions loads all Claude Code sessions from the local filesystem.
// It merges data from sessions-index.json files across all projects with
// active session metadata (which may contain /rename names).
func LoadSessions(claudeDir string) ([]Session, error) {
	if claudeDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		claudeDir = filepath.Join(home, ".claude")
	}

	namesByID := loadActiveSessionNames(claudeDir)
	sessions := loadFromIndexFiles(claudeDir, namesByID)

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Modified.After(sessions[j].Modified)
	})

	return sessions, nil
}

// loadActiveSessionNames reads ~/.claude/sessions/*.json to build a map
// of sessionId -> name for any sessions that have been renamed.
func loadActiveSessionNames(claudeDir string) map[string]string {
	names := make(map[string]string)

	pattern := filepath.Join(claudeDir, "sessions", "*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return names
	}

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var active activeSessionFile
		if err := json.Unmarshal(data, &active); err != nil {
			continue
		}
		if active.SessionID != "" && active.Name != "" {
			names[active.SessionID] = active.Name
		}
	}

	return names
}

// loadFromIndexFiles reads all sessions-index.json files under the projects
// directory and converts them to Session structs.
func loadFromIndexFiles(claudeDir string, namesByID map[string]string) []Session {
	var sessions []Session
	seen := make(map[string]bool)

	pattern := filepath.Join(claudeDir, "projects", "*", "sessions-index.json")
	indexFiles, err := filepath.Glob(pattern)
	if err != nil {
		return sessions
	}

	for _, indexFile := range indexFiles {
		data, err := os.ReadFile(indexFile)
		if err != nil {
			continue
		}

		var idx sessionIndexFile
		if err := json.Unmarshal(data, &idx); err != nil {
			continue
		}

		for _, entry := range idx.Entries {
			if entry.IsSidechain || seen[entry.SessionID] {
				continue
			}
			seen[entry.SessionID] = true

			s := Session{
				SessionID:    entry.SessionID,
				Summary:      entry.Summary,
				FirstPrompt:  entry.FirstPrompt,
				FullPath:     entry.FullPath,
				GitBranch:    entry.GitBranch,
				MessageCount: entry.MessageCount,
				ProjectPath:  entry.ProjectPath,
				Source:       "index",
			}

			if name, ok := namesByID[entry.SessionID]; ok {
				s.Name = name
			}

			s.Project = extractProjectName(entry.ProjectPath)
			s.Created = parseTime(entry.Created)
			s.Modified = parseTime(entry.Modified)

			if s.Modified.IsZero() && entry.FileMtime > 0 {
				s.Modified = time.UnixMilli(entry.FileMtime)
			}

			sessions = append(sessions, s)
		}
	}

	return sessions
}

// extractProjectName returns a short project name from the full path.
// For paths like "/Users/user/workspace/my-project", returns "my-project".
func extractProjectName(projectPath string) string {
	if projectPath == "" {
		return ""
	}
	base := filepath.Base(projectPath)
	if base == "." || base == "/" {
		return projectPath
	}
	return base
}

// parseTime parses an ISO 8601 time string, returning zero time on failure.
func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05.000Z",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// MatchesQuery checks if the session matches a search query (case-insensitive substring).
func (s Session) MatchesQuery(query string) bool {
	if query == "" {
		return true
	}
	q := strings.ToLower(query)
	return strings.Contains(strings.ToLower(s.DisplayTitle()), q) ||
		strings.Contains(strings.ToLower(s.Summary), q) ||
		strings.Contains(strings.ToLower(s.FirstPrompt), q) ||
		strings.Contains(strings.ToLower(s.GitBranch), q) ||
		strings.Contains(strings.ToLower(s.Project), q) ||
		strings.Contains(strings.ToLower(s.Name), q)
}
