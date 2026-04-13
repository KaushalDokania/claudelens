package ui

import "github.com/charmbracelet/lipgloss"

// Adaptive colors that work on both light and dark terminals.
var (
	accentColor = lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#AD8CFF"}
	dimColor    = lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"}
	textColor   = lipgloss.AdaptiveColor{Light: "#333333", Dark: "#DDDDDD"}
	userColor   = lipgloss.AdaptiveColor{Light: "#2E86AB", Dark: "#82CFFF"}
	assistColor = lipgloss.AdaptiveColor{Light: "#A23B72", Dark: "#FF8CCF"}
	toolColor   = lipgloss.AdaptiveColor{Light: "#888888", Dark: "#555555"}
	sepColor    = lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#444444"}
	hlBg        = lipgloss.AdaptiveColor{Light: "#E8E0FF", Dark: "#3A2D6B"}
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

	userMsgStyle = lipgloss.NewStyle().
			Foreground(userColor).
			Bold(true)

	assistMsgStyle = lipgloss.NewStyle().
			Foreground(assistColor)

	toolCallStyle = lipgloss.NewStyle().
			Foreground(toolColor).
			Italic(true)

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
