package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

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
	flag.Parse()

	if *showVersion {
		fmt.Printf("claudelens %s\n", version)
		os.Exit(0)
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

	cmd := terminal.BuildResumeCommand(a.ResumeSessionID, a.ResumeProjectPath)

	// Warp has no scriptable "new tab + run command", so resume in the
	// current tab instead: replace this process with claude itself. On
	// success ResumeInPlace never returns; on failure fall through to the
	// launch-config route (new window) and clipboard fallback below.
	if terminal.DetectedTerminal() == "warp" {
		fmt.Printf("\n  Resuming session in this tab...\n\n")
		if err := terminal.ResumeInPlace(a.ResumeSessionID, a.ResumeProjectPath); err != nil {
			fmt.Printf("  Couldn't resume in place (%v), trying a new window instead.\n", err)
		}
	}

	// Try to open in a new terminal tab (iTerm2, Terminal.app, tmux) or a
	// new window via launch config (Warp fallback)
	err = terminal.ResumeInNewTab(a.ResumeSessionID, a.ResumeProjectPath)
	if err == nil {
		if terminal.DetectedTerminal() == "warp" {
			fmt.Printf("\n  Resuming in a new Warp window.\n\n")
		} else {
			fmt.Printf("\n  Resuming in new %s tab.\n\n", terminal.DetectedTerminal())
		}
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
