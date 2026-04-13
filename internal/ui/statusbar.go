package ui

import (
	"fmt"

	"github.com/KaushalDokania/claudelens/internal/data"
	"github.com/charmbracelet/lipgloss"
)

// renderStatusBar renders a 2-line status bar:
// Line 1: resume command for selected session
// Line 2: key hints and session count
func renderStatusBar(width int, session *data.Session, claudeMemAvailable bool, semanticEnabled bool, sessionCount int, focusedPane Pane) string {
	// Line 1: Resume command for selected session
	var cmdLine string
	if session != nil {
		cmd := fmt.Sprintf("claude --resume %s", session.SessionID)
		cmdLine = lipgloss.NewStyle().Foreground(accentColor).Render("  > " + cmd)
	} else {
		cmdLine = dimStyle.Render("  Select a session to see resume command")
	}

	// Line 2: Key hints
	var hints string
	if focusedPane == PaneSearch {
		hints = "  Type to search · Enter confirm · Esc clear"
	} else {
		hints = "  ↑↓/jk Navigate · Enter Resume · c Copy · / Search · s Semantic · ? Help · q Quit"
	}
	left := dimStyle.Render(hints)

	memStatus := ""
	if claudeMemAvailable {
		if semanticEnabled {
			memStatus = lipgloss.NewStyle().Foreground(accentColor).Render("[Semantic ON]")
		} else {
			memStatus = dimStyle.Render("[s Semantic]")
		}
	} else {
		memStatus = dimStyle.Render("[claude-mem offline]")
	}

	right := fmt.Sprintf("%s  %s", memStatus, dimStyle.Render(fmt.Sprintf("%d sessions", sessionCount)))

	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}

	padding := ""
	for i := 0; i < gap; i++ {
		padding += " "
	}

	keysLine := left + padding + right

	return cmdLine + "\n" + keysLine
}
