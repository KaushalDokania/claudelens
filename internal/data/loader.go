package data

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// sessionIndexFile represents the structure of sessions-index.json.
type sessionIndexFile struct {
	Version int                 `json:"version"`
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
// It combines data from two sources:
//  1. sessions-index.json files (may be stale)
//  2. JSONL files on disk (always current — the primary source)
//
// Sessions found via JSONL scan take priority since the index can be stale.
func LoadSessions(claudeDir string) ([]Session, error) {
	if claudeDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		claudeDir = filepath.Join(home, ".claude")
	}

	namesByID := loadActiveSessionNames(claudeDir)
	indexSessions := loadFromIndexFiles(claudeDir, namesByID)
	jsonlSessions := loadFromJSONLFiles(claudeDir, namesByID)

	// Merge: JSONL-discovered sessions take priority, index fills gaps
	seen := make(map[string]bool)
	var sessions []Session

	for _, s := range jsonlSessions {
		seen[s.SessionID] = true
		sessions = append(sessions, s)
	}

	for _, s := range indexSessions {
		if seen[s.SessionID] {
			continue
		}
		sessions = append(sessions, s)
	}

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
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var active activeSessionFile
		if err := json.Unmarshal(raw, &active); err != nil {
			continue
		}
		if active.SessionID != "" && active.Name != "" {
			names[active.SessionID] = active.Name
		}
	}

	return names
}

// loadFromJSONLFiles scans all project directories for JSONL conversation files
// and extracts session metadata by reading the first few lines of each file.
// This is the primary discovery method since sessions-index.json can be stale.
func loadFromJSONLFiles(claudeDir string, namesByID map[string]string) []Session {
	var sessions []Session

	projectsDir := filepath.Join(claudeDir, "projects")
	projectDirs, err := os.ReadDir(projectsDir)
	if err != nil {
		return sessions
	}

	for _, pd := range projectDirs {
		if !pd.IsDir() {
			continue
		}

		// Skip internal/plugin session directories
		if isInternalProjectDir(pd.Name()) {
			continue
		}

		dirPath := filepath.Join(projectsDir, pd.Name())
		jsonlFiles, err := filepath.Glob(filepath.Join(dirPath, "*.jsonl"))
		if err != nil {
			continue
		}

		for _, jsonlPath := range jsonlFiles {
			base := filepath.Base(jsonlPath)
			sessionID := strings.TrimSuffix(base, ".jsonl")

			// Skip non-UUID filenames
			if len(sessionID) < 36 {
				continue
			}

			info, err := os.Stat(jsonlPath)
			if err != nil {
				continue
			}

			s := Session{
				SessionID: sessionID,
				FullPath:  jsonlPath,
				Modified:  info.ModTime(),
				Source:    "jsonl",
			}

			if name, ok := namesByID[sessionID]; ok {
				s.Name = name
			}

			// Extract metadata from the first few lines
			extractJSONLMetadata(jsonlPath, &s)

			sessions = append(sessions, s)
		}
	}

	return sessions
}

// jsonlMetaLine is used to extract metadata from the first lines of a JSONL file.
type jsonlMetaLine struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionId"`
	Timestamp string `json:"timestamp"`
	GitBranch string `json:"gitBranch"`
	CWD       string `json:"cwd"`
	Message   struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// extractJSONLMetadata reads the first few lines of a JSONL file to extract
// cwd (project path), first prompt, git branch, timestamps, and message count.
func extractJSONLMetadata(path string, s *Session) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 2*1024*1024)

	lineCount := 0
	maxScanLines := 30

	for scanner.Scan() && lineCount < maxScanLines {
		lineCount++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry jsonlMetaLine
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		// Extract cwd (the real project path) — typically on line 2
		if s.ProjectPath == "" && entry.CWD != "" {
			s.ProjectPath = entry.CWD
			s.Project = extractProjectName(entry.CWD)
		}

		// Set creation time from first timestamped entry
		if s.Created.IsZero() && entry.Timestamp != "" {
			s.Created = parseTime(entry.Timestamp)
		}

		// Capture git branch
		if s.GitBranch == "" && entry.GitBranch != "" {
			s.GitBranch = entry.GitBranch
		}

		// Capture first user message as the prompt/summary
		if s.FirstPrompt == "" && entry.Message.Role == "user" {
			s.FirstPrompt = extractTextFromContent(entry.Message.Content)
			if len(s.FirstPrompt) > 200 {
				s.FirstPrompt = s.FirstPrompt[:200]
			}
		}

		// Capture first assistant message for summary
		if s.Summary == "" && entry.Message.Role == "assistant" {
			text := extractTextFromContent(entry.Message.Content)
			if len(text) > 120 {
				text = text[:120]
			}
			s.Summary = text
		}
	}

	// Estimate message count from file size (rough: ~2KB per message average)
	if info, err := os.Stat(path); err == nil {
		s.MessageCount = int(info.Size() / 2048)
		if s.MessageCount < 1 {
			s.MessageCount = 1
		}
	}
}

// extractTextFromContent extracts plain text from a message content field,
// which can be a string or an array of content blocks.
func extractTextFromContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var plainText string
	if err := json.Unmarshal(raw, &plainText); err == nil {
		return strings.TrimSpace(plainText)
	}

	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				return strings.TrimSpace(b.Text)
			}
		}
	}

	return ""
}

// isInternalProjectDir returns true for project directories that contain
// internal/plugin sessions (not user-initiated conversations).
func isInternalProjectDir(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "observer-sessions") ||
		strings.Contains(lower, "claude-mem")
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
		raw, err := os.ReadFile(indexFile)
		if err != nil {
			continue
		}

		var idx sessionIndexFile
		if err := json.Unmarshal(raw, &idx); err != nil {
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
