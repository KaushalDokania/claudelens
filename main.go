package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/KaushalDokania/claudelens/internal/data"
	"github.com/KaushalDokania/claudelens/internal/terminal"
	"github.com/KaushalDokania/claudelens/internal/ui"
	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
)

const version = "0.1.0"

func main() {
	showVersion := flag.Bool("version", false, "Print version and exit")
	search := flag.String("search", "", "Pre-fill search query")
	memURL := flag.String("mem-url", os.Getenv("CLAUDELENS_MEM_URL"), "claude-mem API URL")
	claudeDir := flag.String("claude-dir", os.Getenv("CLAUDELENS_CLAUDE_DIR"), "Claude Code config directory")
	resumeID := flag.String("resume", "", "Resume a session by ID directly, skipping the picker (like `claude --resume`)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("claudelens %s\n", version)
		os.Exit(0)
	}

	if *resumeID != "" {
		resumeByID(*resumeID, *claudeDir)
		return
	}

	app := ui.NewApp(*claudeDir, *memURL, *search)
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())

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

	// Warp has no scriptable "new tab + run command", so resume in the
	// current tab instead: replace this process with claude itself. On
	// success ResumeInPlace never returns; on failure fall through to the
	// shared new-tab/clipboard fallback below.
	if terminal.DetectedTerminal() == "warp" {
		fmt.Printf("\n  Resuming session in this tab...\n\n")
		if err := terminal.ResumeInPlace(a.ResumeSessionID, a.ResumeProjectPath); err != nil {
			fmt.Printf("  Couldn't resume in place (%v), trying a new window instead.\n", err)
		}
	}

	resumeInNewTabOrClipboard(a.ResumeSessionID, a.ResumeProjectPath)
}

// resumeByID loads sessions, resolves projectPath for sessionID, and hands
// off to resumeInNewTabOrClipboard — used by --resume, which skips the TUI
// entirely. Never calls ResumeInPlace: the calling shell is often a live,
// working session (e.g. right after /branch), and process-replacing it
// would kill work in progress rather than open a separate one.
func resumeByID(sessionID, claudeDir string) {
	sessions, err := data.LoadSessions(claudeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading sessions: %v\n", err)
		os.Exit(1)
	}

	var projectPath string
	found := false
	for _, s := range sessions {
		if s.SessionID == sessionID {
			projectPath = s.ProjectPath
			found = true
			break
		}
	}
	if !found {
		fmt.Fprintf(os.Stderr, "Session %s not found\n", sessionID)
		os.Exit(1)
	}

	resumeInNewTabOrClipboard(sessionID, projectPath)
}

// resumeInNewTabOrClipboard opens sessionID in a new terminal tab/window,
// falling back to a clipboard-copy-and-print instruction. Shared by the
// TUI's post-pick flow (after the Warp same-tab attempt) and --resume.
func resumeInNewTabOrClipboard(sessionID, projectPath string) {
	cmd := terminal.BuildResumeCommand(sessionID, projectPath)

	err := terminal.ResumeInNewTab(sessionID, projectPath)
	if err == nil {
		fmt.Printf("\n  Resuming in new %s tab.\n\n", terminal.DetectedTerminal())
		return
	}

	if errors.Is(err, terminal.ErrWarpManualRun) {
		fmt.Printf("\n  Warp can't auto-run this — command copied to clipboard, paste (Cmd+V) and press Enter here:\n\n    %s\n\n", cmd)
		return
	}

	// Fallback for unsupported terminals: print + clipboard
	_ = clipboard.WriteAll(cmd)
	fmt.Printf("\n  Paste (Cmd+V) and run:\n\n    %s\n\n", cmd)
}
