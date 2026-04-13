package main

import (
	"flag"
	"fmt"
	"os"

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

	// If user selected a session to resume, print the command after TUI exits
	if a, ok := model.(*ui.App); ok && a.ResumeCmd != "" {
		fmt.Println()
		fmt.Println("  Run this command to resume your session:")
		fmt.Println()
		fmt.Printf("    %s\n", a.ResumeCmd)
		fmt.Println()
		fmt.Println("  (also copied to clipboard)")
		fmt.Println()
	}
}
