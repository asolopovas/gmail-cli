package auth

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// LaunchAuthTerminal starts this CLI in a separate terminal window for the
// interactive browser OAuth flow. The child process performs the actual token
// exchange and stores the resulting token in the OS keyring.
func LaunchAuthTerminal(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate current executable: %w", err)
	}
	if runtime.GOOS == "windows" {
		return launchWindowsAuthTerminal(exe, args)
	}
	script := terminalScript(exe, args)
	if runtime.GOOS == "darwin" {
		return launchMacAuthTerminal(script)
	}
	return launchLinuxAuthTerminal(script)
}

func terminalScript(exe string, args []string) string {
	argv := append([]string{exe}, args...)
	return shellJoin(argv) + `
status=$?
echo
if [ "$status" -eq 0 ]; then
  echo "Gmail authorization complete. Token stored in the OS keyring."
else
  echo "Gmail authorization failed with exit code $status."
fi
echo "Press Enter to close this authorization window."
read _
exit "$status"`
}

func launchLinuxAuthTerminal(script string) error {
	candidates := []struct {
		name string
		args []string
	}{
		{name: "x-terminal-emulator", args: []string{"-e", "sh", "-lc", script}},
		{name: "gnome-terminal", args: []string{"--", "sh", "-lc", script}},
		{name: "konsole", args: []string{"-e", "sh", "-lc", script}},
		{name: "xfce4-terminal", args: []string{"-e", "sh -lc " + shellQuote(script)}},
		{name: "xterm", args: []string{"-e", "sh", "-lc", script}},
	}
	var misses []string
	for _, candidate := range candidates {
		path, err := exec.LookPath(candidate.name)
		if err != nil {
			misses = append(misses, candidate.name)
			continue
		}
		if err := exec.Command(path, candidate.args...).Start(); err != nil {
			return fmt.Errorf("start %s: %w", candidate.name, err)
		}
		return nil
	}
	return fmt.Errorf("no supported terminal emulator found; tried %s", strings.Join(misses, ", "))
}

func launchMacAuthTerminal(script string) error {
	osaScript := `tell application "Terminal" to do script ` + appleScriptQuote(script)
	if err := exec.Command("osascript", "-e", osaScript).Start(); err != nil {
		return fmt.Errorf("start macOS Terminal: %w", err)
	}
	return nil
}

func launchWindowsAuthTerminal(exe string, args []string) error {
	if exe == "" {
		return errors.New("executable path is empty")
	}
	cmdArgs := append([]string{"/K", quoteWindows(exe)}, args...)
	startArgs := append([]string{"/C", "start", "Gmail authorization", "cmd"}, cmdArgs...)
	if err := exec.Command("cmd", startArgs...).Start(); err != nil {
		return fmt.Errorf("start Windows terminal: %w", err)
	}
	return nil
}

func shellJoin(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func appleScriptQuote(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	return "\"" + value + "\""
}

func quoteWindows(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}
