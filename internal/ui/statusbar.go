package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// renderStatusBar renders the bottom key hints bar.
func renderStatusBar(width int, claudeMemAvailable bool, semanticEnabled bool, sessionCount int) string {
	left := statusBarStyle.Render("↑↓ Navigate  Enter Resume  c Copy  / Search  ? Help  q Quit")

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

	return left + padding + right
}
