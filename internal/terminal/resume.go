package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/atotto/clipboard"
)

// ResumeInNewTab opens a new terminal tab and runs `claude --resume <sessionID>`
// in the given project directory. Falls back to clipboard copy if terminal
// detection fails.
func ResumeInNewTab(sessionID, projectPath string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("new tab not supported on %s", runtime.GOOS)
	}

	switch detectTerminal() {
	case "warp":
		return openWarpTab(sessionID, projectPath)
	case "iterm2":
		return openITerm2Tab(sessionID, projectPath)
	case "tmux":
		return openTmuxWindow(sessionID, projectPath)
	case "terminal":
		return openTerminalAppTab(sessionID, projectPath)
	default:
		return fmt.Errorf("unsupported terminal")
	}
}

// CopyResumeCommand copies `claude --resume <sessionID>` to the clipboard.
func CopyResumeCommand(sessionID string) error {
	cmd := fmt.Sprintf("claude --resume %s", sessionID)
	return clipboard.WriteAll(cmd)
}

// BuildResumeCommand creates the full cd + claude --resume command string.
func BuildResumeCommand(sessionID, projectPath string) string {
	if projectPath != "" {
		return fmt.Sprintf("cd %q && claude --resume %s", projectPath, sessionID)
	}
	return fmt.Sprintf("claude --resume %s", sessionID)
}

// DetectedTerminal returns the name of the detected terminal for display.
func DetectedTerminal() string {
	return detectTerminal()
}

func detectTerminal() string {
	if os.Getenv("TERM_PROGRAM") == "WarpTerminal" {
		return "warp"
	}
	if os.Getenv("ITERM_SESSION_ID") != "" {
		return "iterm2"
	}
	if os.Getenv("TMUX") != "" {
		return "tmux"
	}
	if os.Getenv("TERM_PROGRAM") == "Apple_Terminal" {
		return "terminal"
	}
	return "unknown"
}

func openWarpTab(sessionID, projectPath string) error {
	// Warp URL scheme: open new tab, then send the command
	// First open a new tab
	err := exec.Command("open", "warp://action/new_tab").Run()
	if err != nil {
		return err
	}
	// Give Warp a moment to create the tab
	time.Sleep(500 * time.Millisecond)

	// Use AppleScript to type the command in the new Warp tab
	var command string
	if projectPath != "" {
		command = fmt.Sprintf("cd '%s' && claude --resume %s", projectPath, sessionID)
	} else {
		command = fmt.Sprintf("claude --resume %s", sessionID)
	}

	script := fmt.Sprintf(`tell application "System Events"
	tell process "Warp"
		keystroke "%s"
		key code 36
	end tell
end tell`, command)

	return exec.Command("osascript", "-e", script).Run()
}

func openITerm2Tab(sessionID, projectPath string) error {
	// Build command with single quotes to avoid AppleScript escaping issues
	var command string
	if projectPath != "" {
		command = fmt.Sprintf("cd '%s' && claude --resume %s", projectPath, sessionID)
	} else {
		command = fmt.Sprintf("claude --resume %s", sessionID)
	}

	script := fmt.Sprintf(`tell application "iTerm2"
	tell current window
		create tab with default profile
		tell current session
			write text "%s"
		end tell
	end tell
end tell`, command)

	return exec.Command("osascript", "-e", script).Run()
}

func openTerminalAppTab(sessionID, projectPath string) error {
	var command string
	if projectPath != "" {
		command = fmt.Sprintf("cd '%s' && claude --resume %s", projectPath, sessionID)
	} else {
		command = fmt.Sprintf("claude --resume %s", sessionID)
	}

	script := fmt.Sprintf(`tell application "Terminal"
	activate
	do script "%s"
end tell`, command)

	return exec.Command("osascript", "-e", script).Run()
}

func openTmuxWindow(sessionID, projectPath string) error {
	command := fmt.Sprintf("claude --resume %s", sessionID)
	args := []string{"new-window"}
	if projectPath != "" {
		args = append(args, "-c", projectPath)
	}
	args = append(args, command)
	return exec.Command("tmux", args...).Run()
}
