package ui

import "github.com/charmbracelet/lipgloss"

// Adaptive colors that work on both light and dark terminals.
var (
	accentColor = lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#AD8CFF"}
	dimColor    = lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"}
	textColor   = lipgloss.AdaptiveColor{Light: "#333333", Dark: "#DDDDDD"}
	sepColor    = lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#444444"}
	hlBg        = lipgloss.AdaptiveColor{Light: "#E8E0FF", Dark: "#3A2D6B"}

	// Claude Code-inspired preview colors
	userPromptColor = lipgloss.AdaptiveColor{Light: "#0077B6", Dark: "#56B6F7"} // Cyan-blue like Claude's ❯
	assistTextColor = lipgloss.AdaptiveColor{Light: "#333333", Dark: "#D4D4D4"} // Default text
	toolIconColor   = lipgloss.AdaptiveColor{Light: "#B8860B", Dark: "#E5A84B"} // Amber for tool calls
	toolLabelColor  = lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"} // Dim for tool details
	tsColor         = lipgloss.AdaptiveColor{Light: "#AAAAAA", Dark: "#555555"} // Very dim for timestamps
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accentColor)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accentColor).
			Background(hlBg)

	selectedDimStyle = lipgloss.NewStyle().
				Foreground(dimColor).
				Background(hlBg)

	normalStyle = lipgloss.NewStyle().
			Foreground(textColor)

	dimStyle = lipgloss.NewStyle().
			Foreground(dimColor)

	// Preview styles — designed to match Claude Code CLI appearance
	userMsgStyle = lipgloss.NewStyle().
			Foreground(userPromptColor).
			Bold(true)

	assistMsgStyle = lipgloss.NewStyle().
			Foreground(assistTextColor)

	toolCallStyle = lipgloss.NewStyle().
			Foreground(toolLabelColor)

	toolIconStyle = lipgloss.NewStyle().
			Foreground(toolIconColor)

	timestampStyle = lipgloss.NewStyle().
			Foreground(tsColor)

	separatorStyle = lipgloss.NewStyle().
			Foreground(sepColor)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(dimColor).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accentColor).
			Padding(0, 1)

	searchStyle = lipgloss.NewStyle().
			Padding(0, 1)

	paneStyle = lipgloss.NewStyle().
			Padding(0, 1)

	activePaneBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(accentColor)

	inactivePaneBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(sepColor)
)
