package terminal

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"

	"github.com/atotto/clipboard"
)

// ResumeInNewTab opens a new terminal tab and runs `claude --resume <sessionID>`
// in the given project directory. Falls back to clipboard copy if terminal
// detection fails.
func ResumeInNewTab(sessionID, projectPath string) error {
	if runtime.GOOS != "darwin" {
		return CopyResumeCommand(sessionID)
	}

	cmd := buildResumeCommand(sessionID, projectPath)

	switch detectTerminal() {
	case "warp":
		return openWarpTab(cmd)
	case "iterm2":
		return openITerm2Tab(cmd)
	case "tmux":
		return openTmuxWindow(cmd, projectPath)
	case "terminal":
		return openTerminalAppTab(cmd)
	default:
		if err := CopyResumeCommand(sessionID); err != nil {
			return err
		}
		return fmt.Errorf("unknown terminal, command copied to clipboard")
	}
}

// CopyResumeCommand copies `claude --resume <sessionID>` to the clipboard.
func CopyResumeCommand(sessionID string) error {
	cmd := fmt.Sprintf("claude --resume %s", sessionID)
	return clipboard.WriteAll(cmd)
}

// DetectedTerminal returns the name of the detected terminal for display.
func DetectedTerminal() string {
	return detectTerminal()
}

func buildResumeCommand(sessionID, projectPath string) string {
	if projectPath != "" {
		return fmt.Sprintf("cd %q && claude --resume %s", projectPath, sessionID)
	}
	return fmt.Sprintf("claude --resume %s", sessionID)
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

func openWarpTab(command string) error {
	u := fmt.Sprintf("warp://action/new_tab?command=%s", url.QueryEscape(command))
	return exec.Command("open", u).Run()
}

func openITerm2Tab(command string) error {
	script := fmt.Sprintf(`tell application "iTerm2"
	tell current window
		create tab with default profile
		tell current session
			write text %q
		end tell
	end tell
end tell`, command)

	return exec.Command("osascript", "-e", script).Run()
}

func openTerminalAppTab(command string) error {
	script := fmt.Sprintf(`tell application "Terminal"
	activate
	do script %q
end tell`, command)

	return exec.Command("osascript", "-e", script).Run()
}

func openTmuxWindow(command, projectPath string) error {
	args := []string{"new-window"}
	if projectPath != "" {
		args = append(args, "-c", projectPath)
	}
	args = append(args, command)
	return exec.Command("tmux", args...).Run()
}
