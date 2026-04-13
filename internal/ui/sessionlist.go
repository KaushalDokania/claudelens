package ui

import (
	"fmt"
	"strings"

	"github.com/KaushalDokania/claudelens/internal/data"
	"github.com/charmbracelet/lipgloss"
)

// renderSessionList renders the left pane showing the session list.
// Each session item is 2 lines (title + metadata). Scrolling is line-aware.
func renderSessionList(sessions []data.Session, memSessions []data.Session, cursor int, width, height int) string {
	if len(sessions) == 0 && len(memSessions) == 0 {
		msg := dimStyle.Render("No sessions found.\nStart a Claude Code session first.")
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, msg)
	}

	// Build all items as rendered line-pairs
	type listItem struct {
		lines    string // Rendered content (may be 1 or 2 lines)
		isCursor bool
	}
	var items []listItem

	for i, s := range sessions {
		items = append(items, listItem{
			lines:    renderSessionItem(s, i == cursor, width, i+1),
			isCursor: i == cursor,
		})
	}

	if len(memSessions) > 0 {
		sep := separatorStyle.Render(fmt.Sprintf("── claude-mem (%d) ", len(memSessions)) + strings.Repeat("─", width/2))
		items = append(items, listItem{lines: sep})

		for i, s := range memSessions {
			memCursorIdx := len(sessions) + 1 + i
			items = append(items, listItem{
				lines:    renderSessionItem(s, memCursorIdx == cursor, width, len(sessions)+i+1),
				isCursor: memCursorIdx == cursor,
			})
		}
	}

	// Flatten items into lines, tracking which line range is the cursor
	var allLines []string
	cursorLineStart := 0
	cursorLineEnd := 0
	lineCount := 0

	for _, item := range items {
		itemLines := strings.Split(item.lines, "\n")
		if item.isCursor {
			cursorLineStart = lineCount
			cursorLineEnd = lineCount + len(itemLines)
		}
		allLines = append(allLines, itemLines...)
		lineCount += len(itemLines)
	}

	// Scroll so cursor item is visible
	scrollStart := 0
	if cursorLineEnd > height {
		scrollStart = cursorLineEnd - height
	}
	if cursorLineStart < scrollStart {
		scrollStart = cursorLineStart
	}

	// Extract visible window
	visibleEnd := scrollStart + height
	if visibleEnd > len(allLines) {
		visibleEnd = len(allLines)
	}
	visible := allLines[scrollStart:visibleEnd]

	// Pad to fill height
	for len(visible) < height {
		visible = append(visible, "")
	}

	return strings.Join(visible, "\n")
}

// renderSessionItem renders a single session entry (2 lines) with a number prefix.
func renderSessionItem(s data.Session, selected bool, width int, number int) string {
	numPrefix := fmt.Sprintf("%3d. ", number)
	indent := "     " // same width as "  1. "

	title := s.DisplayTitle()
	maxTitle := width - len(numPrefix) - 2
	if maxTitle > 0 && len(title) > maxTitle {
		title = title[:maxTitle-3] + "..."
	}

	meta := fmt.Sprintf("%s · %s · ~%d msgs",
		s.ShortProject(), s.RelativeTime(), s.MessageCount)
	if s.GitBranch != "" && s.GitBranch != "HEAD" {
		meta += " · " + s.GitBranch
	}
	maxMeta := width - len(indent) - 2
	if maxMeta > 0 && len(meta) > maxMeta {
		meta = meta[:maxMeta-3] + "..."
	}

	if selected {
		titleLine := selectedStyle.Render(numPrefix + title)
		metaLine := selectedDimStyle.Render(indent + meta)
		return titleLine + "\n" + metaLine
	}

	titleLine := normalStyle.Render(numPrefix + title)
	metaLine := dimStyle.Render(indent + meta)
	return titleLine + "\n" + metaLine
}

// sessionAtCursor returns the session at the given cursor position,
// accounting for the claude-mem separator line.
func sessionAtCursor(sessions []data.Session, memSessions []data.Session, cursor int) *data.Session {
	if cursor < len(sessions) {
		return &sessions[cursor]
	}

	// Skip the separator line
	memIdx := cursor - len(sessions) - 1
	if len(memSessions) > 0 && memIdx >= 0 && memIdx < len(memSessions) {
		return &memSessions[memIdx]
	}

	return nil
}

// totalItems returns the total number of navigable items
// (sessions + separator + memSessions).
func totalItems(sessions []data.Session, memSessions []data.Session) int {
	total := len(sessions)
	if len(memSessions) > 0 {
		total += 1 + len(memSessions) // separator + items
	}
	return total
}

// isSeparator returns true if the cursor is on the claude-mem separator line.
func isSeparator(sessions []data.Session, memSessions []data.Session, cursor int) bool {
	return len(memSessions) > 0 && cursor == len(sessions)
}
