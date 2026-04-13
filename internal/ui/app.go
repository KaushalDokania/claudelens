package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/KaushalDokania/claudelens/internal/data"
	"github.com/KaushalDokania/claudelens/internal/terminal"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Pane identifies which pane has focus.
type Pane int

const (
	PaneList Pane = iota
	PanePreview
	PaneSearch
)

// App is the root Bubbletea model.
type App struct {
	sessions     []data.Session // All loaded sessions (from index)
	filtered     []data.Session // Currently visible (after search filter)
	memSessions  []data.Session // Semantic search results from claude-mem
	memClient    *data.ClaudeMemClient
	memAvailable bool
	semEnabled   bool // Whether semantic search is toggled on

	searchQuery string
	cursor      int
	focusedPane Pane
	width       int
	height      int

	preview      []data.ConversationMessage
	previewErr   string
	previewID    string // SessionID currently shown in preview
	previewReady bool

	statusMsg string // Transient message in status bar
	showHelp  bool

	claudeDir    string // Path to ~/.claude
	ResumeCmd    string // Set on resume — printed after TUI exits
}

// Messages
type sessionsLoadedMsg struct {
	sessions []data.Session
}

type memCheckMsg struct {
	available bool
}

type memSearchMsg struct {
	sessions []data.Session
	query    string
}

type previewLoadedMsg struct {
	sessionID string
	messages  []data.ConversationMessage
	err       error
}

type statusClearMsg struct{}

// NewApp creates the initial App model.
func NewApp(claudeDir, memURL, initialSearch string) *App {
	memClient := data.NewClaudeMemClient(memURL)
	return &App{
		claudeDir:   claudeDir,
		memClient:   memClient,
		focusedPane: PaneList,
		searchQuery: initialSearch,
	}
}

// Init loads sessions on startup.
func (a *App) Init() tea.Cmd {
	return tea.Batch(
		loadSessionsCmd(a.claudeDir),
		checkMemCmd(a.memClient),
	)
}

func loadSessionsCmd(claudeDir string) tea.Cmd {
	return func() tea.Msg {
		sessions, _ := data.LoadSessions(claudeDir)
		return sessionsLoadedMsg{sessions: sessions}
	}
}

func checkMemCmd(client *data.ClaudeMemClient) tea.Cmd {
	return func() tea.Msg {
		return memCheckMsg{available: client.IsAvailable()}
	}
}

func searchMemCmd(client *data.ClaudeMemClient, query string) tea.Cmd {
	return func() tea.Msg {
		results, err := client.SearchSessions(query, 10)
		if err != nil {
			return memSearchMsg{query: query}
		}
		var sessions []data.Session
		for _, r := range results {
			summary, err := client.GetSession(r.ID)
			if err != nil {
				continue
			}
			sessions = append(sessions, data.SummaryToSession(*summary))
		}
		return memSearchMsg{sessions: sessions, query: query}
	}
}

func loadPreviewCmd(path, sessionID string) tea.Cmd {
	return func() tea.Msg {
		msgs, err := data.ParseConversation(path, 50)
		return previewLoadedMsg{sessionID: sessionID, messages: msgs, err: err}
	}
}

// Update handles all messages.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case sessionsLoadedMsg:
		a.sessions = msg.sessions
		a.applyFilter()
		return a, a.loadPreviewForCursor()

	case memCheckMsg:
		a.memAvailable = msg.available
		return a, nil

	case memSearchMsg:
		if msg.query == a.searchQuery {
			a.memSessions = msg.sessions
		}
		return a, nil

	case previewLoadedMsg:
		if msg.sessionID == a.previewID {
			a.preview = msg.messages
			a.previewReady = true
			if msg.err != nil {
				a.previewErr = msg.err.Error()
			} else {
				a.previewErr = ""
			}
		}
		return a, nil

	case resumeMsg:
		a.ResumeCmd = msg.command
		return a, tea.Quit

	case statusSetMsg:
		a.statusMsg = msg.text
		return a, clearStatusAfter(3 * time.Second)

	case statusClearMsg:
		a.statusMsg = ""
		return a, nil

	case tea.KeyMsg:
		return a.handleKey(msg)
	}

	return a, nil
}

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global keys
	switch key {
	case "ctrl+c":
		return a, tea.Quit
	case "?":
		if a.focusedPane != PaneSearch {
			a.showHelp = !a.showHelp
			return a, nil
		}
	}

	if a.showHelp {
		a.showHelp = false
		return a, nil
	}

	// Search pane
	if a.focusedPane == PaneSearch {
		return a.handleSearchKey(msg)
	}

	// List/Preview common
	switch key {
	case "q":
		return a, tea.Quit
	case "/":
		a.focusedPane = PaneSearch
		return a, nil
	case "tab":
		if a.focusedPane == PaneList {
			a.focusedPane = PanePreview
		} else {
			a.focusedPane = PaneList
		}
		return a, nil
	case "s":
		if a.memAvailable {
			a.semEnabled = !a.semEnabled
			if a.semEnabled && a.searchQuery != "" {
				return a, searchMemCmd(a.memClient, a.searchQuery)
			}
			if !a.semEnabled {
				a.memSessions = nil
			}
		}
		return a, nil
	}

	// List-specific
	if a.focusedPane == PaneList {
		return a.handleListKey(msg)
	}

	return a, nil
}

func (a *App) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle Enter by type (Warp sends different key codes)
	if msg.Type == tea.KeyEnter || key == "enter" {
		a.focusedPane = PaneList
		return a, nil
	}

	switch key {
	case "esc":
		a.searchQuery = ""
		a.focusedPane = PaneList
		a.applyFilter()
		a.memSessions = nil
		return a, a.loadPreviewForCursor()
	case "backspace":
		if len(a.searchQuery) > 0 {
			a.searchQuery = a.searchQuery[:len(a.searchQuery)-1]
			a.cursor = 0
			a.applyFilter()
			var cmds []tea.Cmd
			cmds = append(cmds, a.loadPreviewForCursor())
			if a.semEnabled && a.memAvailable && a.searchQuery != "" {
				cmds = append(cmds, searchMemCmd(a.memClient, a.searchQuery))
			}
			return a, tea.Batch(cmds...)
		}
		return a, nil
	default:
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			a.searchQuery += key
			a.cursor = 0
			a.applyFilter()
			var cmds []tea.Cmd
			cmds = append(cmds, a.loadPreviewForCursor())
			if a.semEnabled && a.memAvailable {
				cmds = append(cmds, searchMemCmd(a.memClient, a.searchQuery))
			}
			return a, tea.Batch(cmds...)
		}
	}
	return a, nil
}

func (a *App) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	total := totalItems(a.filtered, a.memSessions)

	switch key {
	case "up", "k":
		if a.cursor > 0 {
			a.cursor--
			if isSeparator(a.filtered, a.memSessions, a.cursor) && a.cursor > 0 {
				a.cursor--
			}
			return a, a.loadPreviewForCursor()
		}
	case "down", "j":
		if a.cursor < total-1 {
			a.cursor++
			if isSeparator(a.filtered, a.memSessions, a.cursor) && a.cursor < total-1 {
				a.cursor++
			}
			return a, a.loadPreviewForCursor()
		}
	case "enter":
		return a, a.resumeSession()
	case "c":
		return a, a.copyResumeCommand()
	}

	// Warp terminal and some others may send Return as a different key type.
	// Check by type as well as string representation.
	if msg.Type == tea.KeyEnter {
		return a, a.resumeSession()
	}

	return a, nil
}

func (a *App) applyFilter() {
	if a.searchQuery == "" {
		a.filtered = a.sessions
		return
	}
	a.filtered = nil
	for _, s := range a.sessions {
		if s.MatchesQuery(a.searchQuery) {
			a.filtered = append(a.filtered, s)
		}
	}
}

// loadPreviewForCursor updates preview state and returns a Cmd to load the
// conversation. Must be called on a pointer receiver so previewID is persisted.
func (a *App) loadPreviewForCursor() tea.Cmd {
	s := sessionAtCursor(a.filtered, a.memSessions, a.cursor)
	if s == nil || s.FullPath == "" {
		a.previewReady = false
		a.previewErr = ""
		a.preview = nil
		return nil
	}
	if s.SessionID == a.previewID {
		return nil
	}
	a.previewID = s.SessionID
	a.previewReady = false
	a.previewErr = ""
	return loadPreviewCmd(s.FullPath, s.SessionID)
}

// resumeMsg carries the command to print after the TUI exits.
type resumeMsg struct {
	command string
}

func (a *App) resumeSession() tea.Cmd {
	s := sessionAtCursor(a.filtered, a.memSessions, a.cursor)
	if s == nil {
		return nil
	}
	cmd := terminal.BuildResumeCommand(s.SessionID, s.ProjectPath)
	// Copy to clipboard as a convenience
	_ = terminal.CopyResumeCommand(s.SessionID)
	return func() tea.Msg {
		return resumeMsg{command: cmd}
	}
}

func (a *App) copyResumeCommand() tea.Cmd {
	s := sessionAtCursor(a.filtered, a.memSessions, a.cursor)
	if s == nil {
		return nil
	}
	sessionID := s.SessionID
	return func() tea.Msg {
		err := terminal.CopyResumeCommand(sessionID)
		if err != nil {
			return statusSetMsg{text: fmt.Sprintf("Copy failed: %v", err)}
		}
		return statusSetMsg{text: "Copied to clipboard!"}
	}
}

type statusSetMsg struct {
	text string
}

func clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return statusClearMsg{}
	})
}

// View renders the entire UI.
func (a *App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	if a.showHelp {
		return a.renderHelp()
	}

	// Layout: header + search + (list | preview) + status bar (2 lines)
	headerHeight := 1
	searchHeight := 1
	statusHeight := 2
	contentHeight := a.height - headerHeight - searchHeight - statusHeight - 4 // borders

	if contentHeight < 3 {
		contentHeight = 3
	}

	// Header
	sessionCount := len(a.filtered)
	termName := terminal.DetectedTerminal()
	header := headerStyle.Render(fmt.Sprintf("ClaudeLens v0.1")) +
		dimStyle.Render(fmt.Sprintf("  %d sessions · %s", sessionCount, termName))
	header = lipgloss.PlaceHorizontal(a.width, lipgloss.Left, header)

	// Search bar
	searchIndicator := ""
	if a.focusedPane == PaneSearch {
		searchIndicator = "▸ "
	} else {
		searchIndicator = "  "
	}
	searchLine := searchStyle.Render(fmt.Sprintf("%sSearch: %s_", searchIndicator, a.searchQuery))

	// Content panes
	listWidth := a.width*2/5 - 2
	previewWidth := a.width - listWidth - 6 // borders + padding

	if listWidth < 20 {
		listWidth = 20
	}
	if previewWidth < 20 {
		previewWidth = 20
	}

	listContent := renderSessionList(a.filtered, a.memSessions, a.cursor, listWidth, contentHeight)
	previewContent := a.renderPreview(previewWidth, contentHeight)

	var listPane, previewPane string
	if a.focusedPane == PaneList {
		listPane = activePaneBorder.Width(listWidth).Height(contentHeight).Render(listContent)
		previewPane = inactivePaneBorder.Width(previewWidth).Height(contentHeight).Render(previewContent)
	} else if a.focusedPane == PanePreview {
		listPane = inactivePaneBorder.Width(listWidth).Height(contentHeight).Render(listContent)
		previewPane = activePaneBorder.Width(previewWidth).Height(contentHeight).Render(previewContent)
	} else {
		listPane = inactivePaneBorder.Width(listWidth).Height(contentHeight).Render(listContent)
		previewPane = inactivePaneBorder.Width(previewWidth).Height(contentHeight).Render(previewContent)
	}

	content := lipgloss.JoinHorizontal(lipgloss.Top, listPane, previewPane)

	// Status bar
	totalCount := len(a.filtered)
	if len(a.memSessions) > 0 {
		totalCount += len(a.memSessions)
	}
	selectedSession := sessionAtCursor(a.filtered, a.memSessions, a.cursor)
	var status string
	if a.statusMsg != "" {
		status = statusBarStyle.Render("  "+a.statusMsg) + "\n" + dimStyle.Render("  ↑↓ Navigate · Enter Resume · c Copy · / Search · q Quit")
	} else {
		status = renderStatusBar(a.width, selectedSession, a.memAvailable, a.semEnabled, totalCount, a.focusedPane)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, searchLine, content, status)
}

// renderPreview renders the right pane with conversation preview.
func (a *App) renderPreview(width, height int) string {
	s := sessionAtCursor(a.filtered, a.memSessions, a.cursor)
	if s == nil {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
			dimStyle.Render("No session selected"))
	}

	if !a.previewReady && a.previewErr == "" {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
			dimStyle.Render("Loading preview..."))
	}

	if a.previewErr != "" {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
			dimStyle.Render("Preview unavailable\n"+a.previewErr))
	}

	if len(a.preview) == 0 {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
			dimStyle.Render("Empty conversation"))
	}

	var lines []string
	for _, msg := range a.preview {
		var prefix string
		var style lipgloss.Style
		if msg.Role == "user" {
			prefix = "You: "
			style = userMsgStyle
		} else {
			prefix = "Claude: "
			style = assistMsgStyle
		}

		content := msg.Content
		if len(content) > width*3 {
			content = content[:width*3] + "..."
		}

		lines = append(lines, style.Render(prefix)+wrapText(content, width-len(prefix)))

		for _, tc := range msg.ToolCalls {
			lines = append(lines, toolCallStyle.Render("  "+tc))
		}

		lines = append(lines, "")
	}

	result := strings.Join(lines, "\n")
	resultLines := strings.Split(result, "\n")
	if len(resultLines) > height {
		resultLines = resultLines[:height]
	}

	return strings.Join(resultLines, "\n")
}

// renderHelp renders the help overlay.
func (a *App) renderHelp() string {
	help := `
  ClaudeLens — Key Bindings

  Navigation
    ↑/k        Move up
    ↓/j        Move down
    Tab        Toggle list/preview focus
    Enter      Resume selected session
    c          Copy resume command to clipboard

  Search
    /          Focus search bar
    s          Toggle semantic search (claude-mem)
    Esc        Clear search

  General
    ?          Toggle this help
    q          Quit

  Press any key to close this help.
`
	styled := lipgloss.NewStyle().
		Padding(2, 4).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentColor).
		Render(help)

	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, styled)
}

// wrapText wraps text to fit within the given width.
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	var result strings.Builder
	col := 0
	for _, r := range text {
		if r == '\n' {
			result.WriteRune(r)
			col = 0
			continue
		}
		if col >= width {
			result.WriteRune('\n')
			col = 0
		}
		result.WriteRune(r)
		col++
	}
	return result.String()
}
