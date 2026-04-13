package data

import (
	"fmt"
	"path/filepath"
	"time"
)

// Session represents a unified Claude Code session from any data source.
type Session struct {
	SessionID    string
	Name         string // From /rename (may be empty)
	Summary      string // AI-generated summary from index
	FirstPrompt  string
	Project      string // Short name (last path component)
	ProjectPath  string // Full path to project directory
	FullPath     string // Path to JSONL conversation file
	GitBranch    string
	MessageCount int
	Created      time.Time
	Modified     time.Time
	Source       string // "index", "claudemem"
}

// DisplayTitle returns the best available title for this session.
// Priority: Name (from /rename) > Summary (from index) > FirstPrompt > SessionID
func (s Session) DisplayTitle() string {
	if s.Name != "" {
		return s.Name
	}
	if s.Summary != "" {
		return s.Summary
	}
	if s.FirstPrompt != "" {
		if len(s.FirstPrompt) > 60 {
			return s.FirstPrompt[:60] + "..."
		}
		return s.FirstPrompt
	}
	return s.SessionID[:8] + "..."
}

// ShortProject returns the last component of the project path.
func (s Session) ShortProject() string {
	if s.Project != "" {
		return s.Project
	}
	if s.ProjectPath != "" {
		return filepath.Base(s.ProjectPath)
	}
	return "unknown"
}

// RelativeTime returns a human-readable relative time string.
func (s Session) RelativeTime() string {
	d := time.Since(s.Modified)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dw ago", int(d.Hours()/(24*7)))
	default:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/(24*30)))
	}
}
