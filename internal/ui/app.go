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

	preview       []data.ConversationMessage
	previewErr    string
	previewID     string // SessionID currently shown in preview
	previewReady  bool
	previewScroll int    // Line offset for preview scrolling
	previewLines  int    // Total rendered lines in preview

	statusMsg string // Transient message in status bar
	showHelp  bool

	claudeDir         string // Path to ~/.claude
	ResumeSessionID   string // Set on resume — used after TUI exits
	ResumeProjectPath string // Project directory for the resumed session
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
		msgs, err := data.ParseConversation(path, 200)
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
		a.ResumeSessionID = msg.sessionID
		a.ResumeProjectPath = msg.projectPath
		return a, tea.Quit

	case statusSetMsg:
		a.statusMsg = msg.text
		return a, clearStatusAfter(3 * time.Second)

	case statusClearMsg:
		a.statusMsg = ""
		return a, nil

	case tea.MouseMsg:
		return a.handleMouse(msg)

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

	// Preview-specific (scrolling)
	if a.focusedPane == PanePreview {
		return a.handlePreviewKey(msg)
	}

	return a, nil
}

func (a *App) handlePreviewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	// Calculate visible height (same as in View)
	visibleHeight := a.height - 1 - 1 - 2 - 4
	if visibleHeight < 3 {
		visibleHeight = 3
	}
	maxScroll := a.previewLines - visibleHeight
	if maxScroll < 0 {
		maxScroll = 0
	}

	switch key {
	case "down", "j":
		if a.previewScroll < maxScroll {
			a.previewScroll++
		}
	case "up", "k":
		if a.previewScroll > 0 {
			a.previewScroll--
		}
	case "g":
		a.previewScroll = 0
	case "G":
		a.previewScroll = maxScroll
	case "d":
		// Half-page down
		a.previewScroll += visibleHeight / 2
		if a.previewScroll > maxScroll {
			a.previewScroll = maxScroll
		}
	case "u":
		// Half-page up
		a.previewScroll -= visibleHeight / 2
		if a.previewScroll < 0 {
			a.previewScroll = 0
		}
	case "enter":
		return a, a.resumeSession()
	case "c":
		return a, a.copyResumeCommand()
	}

	if msg.Type == tea.KeyEnter {
		return a, a.resumeSession()
	}

	return a, nil
}

func (a *App) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	scrollLines := 3

	// Determine which pane the mouse is over based on X position
	listWidth := a.width/3 - 2
	onPreview := msg.X > listWidth+3
	total := totalItems(a.filtered, a.memSessions)

	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if onPreview {
			a.previewScroll -= scrollLines
			if a.previewScroll < 0 {
				a.previewScroll = 0
			}
		} else {
			if a.cursor > 0 {
				a.cursor--
				if isSeparator(a.filtered, a.memSessions, a.cursor) && a.cursor > 0 {
					a.cursor--
				}
				return a, a.loadPreviewForCursor()
			}
		}
	case tea.MouseButtonWheelDown:
		if onPreview {
			visibleHeight := a.height - 1 - 1 - 2 - 4
			maxScroll := a.previewLines - visibleHeight
			if maxScroll < 0 {
				maxScroll = 0
			}
			a.previewScroll += scrollLines
			if a.previewScroll > maxScroll {
				a.previewScroll = maxScroll
			}
		} else {
			if a.cursor < total-1 {
				a.cursor++
				if isSeparator(a.filtered, a.memSessions, a.cursor) && a.cursor < total-1 {
					a.cursor++
				}
				return a, a.loadPreviewForCursor()
			}
		}
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
	a.previewScroll = 0
	return loadPreviewCmd(s.FullPath, s.SessionID)
}

// resumeMsg carries session info for resuming after the TUI exits.
type resumeMsg struct {
	sessionID   string
	projectPath string
}

func (a *App) resumeSession() tea.Cmd {
	s := sessionAtCursor(a.filtered, a.memSessions, a.cursor)
	if s == nil {
		return nil
	}
	return func() tea.Msg {
		return resumeMsg{sessionID: s.SessionID, projectPath: s.ProjectPath}
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
// The output is always exactly a.height lines to prevent layout jitter.
func (a *App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	if a.showHelp {
		return clipToHeight(a.renderHelp(), a.height)
	}

	// Fixed layout: header(1) + search(1) + bordered panes(contentH+2) + status(2)
	borderLines := 2 // top + bottom border of rounded box
	statusHeight := 2
	contentHeight := a.height - 1 - 1 - borderLines - statusHeight

	if contentHeight < 3 {
		contentHeight = 3
	}

	// Header
	sessionCount := len(a.filtered)
	termName := terminal.DetectedTerminal()
	header := headerStyle.Render("ClaudeLens v0.1") +
		dimStyle.Render(fmt.Sprintf("  %d sessions · %s", sessionCount, termName))

	// Search bar
	searchPrefix := "  "
	if a.focusedPane == PaneSearch {
		searchPrefix = "▸ "
	}
	searchLine := searchStyle.Render(fmt.Sprintf("%sSearch: %s_", searchPrefix, a.searchQuery))

	// Content panes — list gets 1/3, preview gets 2/3
	listWidth := a.width/3 - 2
	previewWidth := a.width - listWidth - 6
	if listWidth < 20 {
		listWidth = 20
	}
	if previewWidth < 20 {
		previewWidth = 20
	}

	listContent := clipToHeight(
		renderSessionList(a.filtered, a.memSessions, a.cursor, listWidth, contentHeight),
		contentHeight)
	previewContent := clipToHeight(
		a.renderPreview(previewWidth, contentHeight),
		contentHeight)

	listBorder := inactivePaneBorder
	previewBorder := inactivePaneBorder
	if a.focusedPane == PaneList {
		listBorder = activePaneBorder
	} else if a.focusedPane == PanePreview {
		previewBorder = activePaneBorder
	}

	listPane := listBorder.
		Width(listWidth).MaxWidth(listWidth + 2).
		Height(contentHeight).
		Render(listContent)
	previewPane := previewBorder.
		Width(previewWidth).MaxWidth(previewWidth + 2).
		Height(contentHeight).
		Render(previewContent)
	panesHeight := contentHeight + borderLines
	content := clipToHeight(lipgloss.JoinHorizontal(lipgloss.Top, listPane, previewPane), panesHeight)

	// Status bar
	totalCount := len(a.filtered)
	if len(a.memSessions) > 0 {
		totalCount += len(a.memSessions)
	}
	selectedSession := sessionAtCursor(a.filtered, a.memSessions, a.cursor)
	var status string
	if a.statusMsg != "" {
		status = statusBarStyle.Render("  "+a.statusMsg) + "\n" +
			dimStyle.Render("  ↑↓ Navigate · Enter Resume · c Copy · / Search · q Quit")
	} else {
		status = renderStatusBar(a.width, selectedSession, a.memAvailable, a.semEnabled, totalCount, a.focusedPane)
	}

	// Assemble and hard-clip to terminal height
	full := lipgloss.JoinVertical(lipgloss.Left, header, searchLine, content, clipToHeight(status, statusHeight))
	return clipToHeight(full, a.height)
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
	var lastDate string
	for _, msg := range a.preview {
		// Date separator — show when the date changes between messages
		if !msg.Timestamp.IsZero() {
			msgDate := formatDate(msg.Timestamp.Local())
			if msgDate != lastDate {
				lastDate = msgDate
				dateLine := separatorStyle.Render(fmt.Sprintf("── %s ──", msgDate))
				lines = append(lines, "", dateLine, "")
			}
		}

		// Timestamp
		ts := ""
		if !msg.Timestamp.IsZero() {
			ts = timestampStyle.Render(msg.Timestamp.Local().Format("15:04"))
		}

		if msg.Role == "user" {
			lines = append(lines, userMsgStyle.Render("❯")+" "+ts)

			content := msg.Content
			if len(content) > width*4 {
				content = content[:width*4] + "..."
			}
			lines = append(lines, userMsgStyle.Render(wrapText(content, width)))
		} else {
			lines = append(lines, ts)

			content := msg.Content
			if len(content) > width*4 {
				content = content[:width*4] + "..."
			}
			lines = append(lines, wrapText(content, width))
		}

		for _, tc := range msg.ToolCalls {
			lines = append(lines, toolIconStyle.Render("⏺ ")+toolCallStyle.Render(tc))
		}

		lines = append(lines, "")
	}

	// Flatten into actual lines and truncate each to pane width
	result := strings.Join(lines, "\n")
	allLines := strings.Split(result, "\n")
	for i, line := range allLines {
		allLines[i] = truncateLine(line, width)
	}
	a.previewLines = len(allLines)

	// Clamp scroll offset
	maxScroll := len(allLines) - height
	if maxScroll < 0 {
		maxScroll = 0
	}
	if a.previewScroll > maxScroll {
		a.previewScroll = maxScroll
	}
	if a.previewScroll < 0 {
		a.previewScroll = 0
	}

	// Extract exactly `height` lines
	visible := make([]string, height)
	for i := 0; i < height; i++ {
		srcIdx := a.previewScroll + i
		if srcIdx < len(allLines) {
			visible[i] = allLines[srcIdx]
		}
	}

	// Overlay scroll indicators on first/last lines
	if a.previewScroll > 0 {
		visible[0] = dimStyle.Render(fmt.Sprintf("── ↑ %d lines above ──", a.previewScroll))
	}
	if a.previewScroll+height < len(allLines) {
		visible[height-1] = dimStyle.Render(fmt.Sprintf("── ↓ %d more lines ──", len(allLines)-a.previewScroll-height))
	}

	return strings.Join(visible, "\n")
}

// renderHelp renders the help overlay.
func (a *App) renderHelp() string {
	help := `
  ClaudeLens — Key Bindings

  Navigation
    ↑/k        Move up / scroll preview up
    ↓/j        Move down / scroll preview down
    d/u        Half-page down/up (preview)
    g/G        Top/bottom of preview
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

// clipToHeight ensures a string has exactly `height` lines.
// Truncates if too many, pads with empty lines if too few.
func clipToHeight(s string, height int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// truncateLine truncates a string to maxWidth visible characters.
// Handles ANSI escape sequences by measuring visual width.
func truncateLine(s string, maxWidth int) string {
	w := lipgloss.Width(s)
	if w <= maxWidth {
		return s
	}
	// Brute force: trim runes until it fits
	runes := []rune(s)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		if lipgloss.Width(string(runes)) <= maxWidth {
			return string(runes)
		}
	}
	return ""
}

// formatDate returns a human-friendly date string.
func formatDate(t time.Time) string {
	now := time.Now().Local()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	msgDay := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())

	diff := today.Sub(msgDay)
	switch {
	case diff < 0:
		return t.Format("Mon, Jan 2")
	case diff == 0:
		return "Today"
	case diff <= 24*time.Hour:
		return "Yesterday"
	case diff <= 7*24*time.Hour:
		return t.Format("Monday")
	default:
		return t.Format("Mon, Jan 2, 2006")
	}
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
