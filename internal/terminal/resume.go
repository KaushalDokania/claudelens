package terminal

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"

	"github.com/atotto/clipboard"
)

// ErrWarpManualRun is returned by ResumeInNewTab when the resume command has
// been copied to the clipboard but still needs a manual paste — the launch
// configuration route failed and Warp offers no other scriptable way to run
// a command (no AppleScript support, and its warp:// URI scheme's
// new_tab/new_window actions have no exec/command parameter; see
// warpdotdev/warp#3959, #2110).
var ErrWarpManualRun = errors.New("warp: command copied, paste to run")

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

// ResumeInPlace replaces the current process with `claude --resume` running
// in projectPath — resuming the session in the tab the user is already in,
// with no terminal automation involved. On success it never returns; the
// caller must have restored the terminal (TUI exited) before calling. If
// Exec fails, the original working directory is restored so fallback paths
// don't inherit a surprise cwd change.
func ResumeInPlace(sessionID, projectPath string) error {
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude not found in PATH: %w", err)
	}

	oldWd, _ := os.Getwd()
	if projectPath != "" {
		if err := os.Chdir(projectPath); err != nil {
			return fmt.Errorf("cd %s: %w", projectPath, err)
		}
	}

	err = syscall.Exec(claudePath, []string{"claude", "--resume", sessionID}, os.Environ())
	if oldWd != "" {
		_ = os.Chdir(oldWd)
	}
	return fmt.Errorf("exec claude: %w", err)
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

const warpLaunchConfigName = "claudelens_resume"

// openWarpTab resumes a session in Warp via a launch configuration — the one
// Warp mechanism that can both open a tab AND run a command in it. Warp's
// simpler automation surfaces can't do this: no AppleScript support, and the
// warp:// new_tab/new_window actions accept only path=, never a command
// (warpdotdev/warp#3959, #2110). Launch configs (warp://launch/<file>) run
// their panes' `commands: [exec: ...]` automatically on open.
//
// A single fixed config file is overwritten per resume, so nothing
// accumulates in ~/.warp/launch_configurations/. The full cd+resume command
// is always copied to the clipboard first as a safety net; if writing or
// launching the config fails, ErrWarpManualRun signals the caller to show
// paste instructions instead.
func openWarpTab(sessionID, projectPath string) error {
	_ = clipboard.WriteAll(BuildResumeCommand(sessionID, projectPath))

	home, err := os.UserHomeDir()
	if err != nil {
		return ErrWarpManualRun
	}

	cfgDir := filepath.Join(home, ".warp", "launch_configurations")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return ErrWarpManualRun
	}

	cwd := projectPath
	if cwd == "" {
		cwd = home
	}

	// strconv.Quote produces double-quoted strings with backslash escapes,
	// which YAML double-quoted scalars parse identically for paths/UUIDs.
	config := fmt.Sprintf(`---
name: %s
windows:
  - tabs:
      - title: %s
        layout:
          cwd: %s
          commands:
            - exec: %s
`, warpLaunchConfigName, strconv.Quote("claude resume"), strconv.Quote(cwd),
		strconv.Quote(fmt.Sprintf("claude --resume %s", sessionID)))

	cfgPath := filepath.Join(cfgDir, warpLaunchConfigName+".yaml")
	if err := os.WriteFile(cfgPath, []byte(config), 0o644); err != nil {
		return ErrWarpManualRun
	}

	uri := fmt.Sprintf("warp://launch/%s.yaml", warpLaunchConfigName)
	if err := exec.Command("open", uri).Run(); err != nil {
		return ErrWarpManualRun
	}
	return nil
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
