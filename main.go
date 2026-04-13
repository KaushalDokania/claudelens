package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/KaushalDokania/claudelens/internal/terminal"
	"github.com/KaushalDokania/claudelens/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

const version = "0.1.0"

func main() {
	showVersion := flag.Bool("version", false, "Print version and exit")
	search := flag.String("search", "", "Pre-fill search query")
	memURL := flag.String("mem-url", os.Getenv("CLAUDELENS_MEM_URL"), "claude-mem API URL")
	claudeDir := flag.String("claude-dir", os.Getenv("CLAUDELENS_CLAUDE_DIR"), "Claude Code config directory")
	flag.Parse()

	if *showVersion {
		fmt.Printf("claudelens %s\n", version)
		os.Exit(0)
	}

	app := ui.NewApp(*claudeDir, *memURL, *search)
	p := tea.NewProgram(app, tea.WithAltScreen())

	model, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// If user selected a session to resume, handle it AFTER the TUI has exited
	// (alt-screen is gone, terminal is back to normal)
	a, ok := model.(*ui.App)
	if !ok || a.ResumeSessionID == "" {
		return
	}

	cmd := terminal.BuildResumeCommand(a.ResumeSessionID, a.ResumeProjectPath)

	// Try to open in a new terminal tab (works for iTerm2, Terminal.app, tmux)
	err = terminal.ResumeInNewTab(a.ResumeSessionID, a.ResumeProjectPath)
	if err == nil {
		fmt.Printf("\n  Resuming in new %s tab.\n\n", terminal.DetectedTerminal())
		return
	}

	// Fallback for Warp and unsupported terminals: print + clipboard
	_ = terminal.CopyResumeCommand(a.ResumeSessionID)
	fmt.Printf("\n  Paste (Cmd+V) and run:\n\n    %s\n\n", cmd)
}
