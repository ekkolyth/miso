package scripting

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ekkolyth/miso/internal/proc"
)

// shebang, then extension (manager-aware for .ts), then defaultShell (or sh);
// shell interpreters get -e
func ResolveInterpreter(scriptPath, defaultShell, managerName string) (string, []string, error) {
	file, err := os.Open(scriptPath)
	if err != nil {
		return "", nil, fmt.Errorf("open script file: %w", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	var firstLine string
	if scanner.Scan() {
		firstLine = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		return "", nil, fmt.Errorf("read script file: %w", err)
	}

	var interpreter string
	var interpreterArgs []string

	if strings.HasPrefix(firstLine, "#!") {
		shebang := strings.TrimSpace(strings.TrimPrefix(firstLine, "#!"))
		parts := strings.Fields(shebang)
		if len(parts) > 0 {
			interpreter = parts[0]
			if len(parts) > 1 {
				interpreterArgs = parts[1:]
			}
		}
	}

	if interpreter == "" {
		interpreter = getInterpreterByExtension(strings.ToLower(filepath.Ext(scriptPath)), managerName)
	}

	if interpreter == "" {
		if defaultShell != "" {
			interpreter = defaultShell
		} else {
			interpreter = "sh"
		}
	}

	if isShell(interpreter) {
		interpreterArgs = append([]string{"-e"}, interpreterArgs...)
	}

	if _, err := exec.LookPath(interpreter); err != nil {
		return "", nil, fmt.Errorf("interpreter %q not found in PATH. install it or update script shebang", interpreter)
	}

	return interpreter, interpreterArgs, nil
}

// spawn the resolved interpreter in its own process group, reaping it on signal
func ExecScriptFile(scriptPath string, args []string, workDir, defaultShell, managerName string, environ []string) error {
	interpreter, interpreterArgs, err := ResolveInterpreter(scriptPath, defaultShell, managerName)
	if err != nil {
		return err
	}

	cmdArgs := append(interpreterArgs, scriptPath)
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command(interpreter, cmdArgs...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	if environ != nil {
		cmd.Env = environ
	}
	proc.SetGroup(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start script: %w", err)
	}

	// Foreground TTY exec: hand the child's own process group control of the
	// terminal so interactive children (next dev/build, etc.) can enter raw
	// mode instead of SIGTTOU-stalling in a background group. Restored on exit.
	if fi, statErr := os.Stdin.Stat(); statErr == nil && fi.Mode()&os.ModeCharDevice != 0 {
		if prev, ok := proc.GiveTerminal(os.Stdin.Fd(), cmd.Process.Pid); ok {
			defer proc.RestoreTerminal(os.Stdin.Fd(), prev)
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case sig := <-sigCh:
		pid := cmd.Process.Pid
		_ = proc.KillGroup(pid, syscall.SIGTERM)
		killTimer := time.AfterFunc(2*time.Second, func() { _ = proc.KillGroup(pid, syscall.SIGKILL) })
		<-done
		killTimer.Stop() // child already exited; don't SIGKILL a recycled pid
		return fmt.Errorf("interrupted by signal %v", sig)
	case err := <-done:
		return err
	}
}

func isShell(interpreter string) bool {
	switch interpreter {
	case "sh", "bash", "zsh", "dash", "ksh":
		return true
	}
	return false
}

// get interpreter by file extension
func getInterpreterByExtension(ext, managerName string) string {
	switch ext {
	case ".sh":
		return "sh"
	case ".bash":
		return "bash"
	case ".zsh":
		return "zsh"
	case ".js", ".mjs":
		return "node"
	case ".ts":
		if managerName == "bun" {
			return "bun"
		}
		return "node"
	case ".py":
		return "python3"
	case ".rb":
		return "ruby"
	case ".pl":
		return "perl"
	case ".lua":
		return "lua"
	case ".php":
		return "php"
	default:
		return ""
	}
}
