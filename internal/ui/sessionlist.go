package ui

import (
	"fmt"
	"strings"

	"github.com/KaushalDokania/claudelens/internal/data"
	"github.com/charmbracelet/lipgloss"
)

// renderSessionList renders the left pane showing the session list.
func renderSessionList(sessions []data.Session, memSessions []data.Session, cursor int, width, height int) string {
	if len(sessions) == 0 && len(memSessions) == 0 {
		msg := dimStyle.Render("No sessions found.\nStart a Claude Code session first.")
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, msg)
	}

	var lines []string
	totalItems := len(sessions) + len(memSessions)
	if len(memSessions) > 0 {
		totalItems++ // for the separator line
	}

	// Determine visible window (scrolling)
	maxVisible := height
	scrollStart := 0
	if cursor >= maxVisible {
		scrollStart = cursor - maxVisible + 1
	}

	itemIdx := 0
	for i, s := range sessions {
		if itemIdx >= scrollStart+maxVisible {
			break
		}
		if itemIdx >= scrollStart {
			lines = append(lines, renderSessionItem(s, i == cursor, width))
		}
		itemIdx++
	}

	// claude-mem separator and results
	if len(memSessions) > 0 {
		if itemIdx >= scrollStart && itemIdx < scrollStart+maxVisible {
			sep := separatorStyle.Render(fmt.Sprintf("── claude-mem (%d) ", len(memSessions)) + strings.Repeat("─", width/2))
			if len(sep) > width {
				sep = sep[:width]
			}
			lines = append(lines, sep)
		}
		itemIdx++

		for i, s := range memSessions {
			if itemIdx >= scrollStart+maxVisible {
				break
			}
			memCursorIdx := len(sessions) + 1 + i // +1 for separator
			if itemIdx >= scrollStart {
				lines = append(lines, renderSessionItem(s, memCursorIdx == cursor, width))
			}
			itemIdx++
		}
	}

	// Pad remaining height
	for len(lines) < maxVisible {
		lines = append(lines, "")
	}

	_ = totalItems
	return strings.Join(lines[:maxVisible], "\n")
}

// renderSessionItem renders a single session entry (2 lines).
func renderSessionItem(s data.Session, selected bool, width int) string {
	title := s.DisplayTitle()
	if len(title) > width-4 {
		title = title[:width-7] + "..."
	}

	meta := fmt.Sprintf("%s · %s · %d msgs",
		s.ShortProject(), s.RelativeTime(), s.MessageCount)
	if s.GitBranch != "" {
		meta += " · " + s.GitBranch
	}
	if len(meta) > width-4 {
		meta = meta[:width-7] + "..."
	}

	if selected {
		indicator := "▸ "
		titleLine := selectedStyle.Render(indicator + title)
		metaLine := selectedDimStyle.Render("  " + meta)
		return titleLine + "\n" + metaLine
	}

	titleLine := normalStyle.Render("  " + title)
	metaLine := dimStyle.Render("  " + meta)
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
