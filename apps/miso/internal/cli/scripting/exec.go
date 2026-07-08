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

// execute script file with shebang detection and extension-based interpreter selection
// defaultShell: used when no shebang or known extension; empty means "sh"
func ExecScriptFile(scriptPath string, args []string, workDir string, defaultShell string, environ []string) error {
	// read first line to detect shebang
	file, err := os.Open(scriptPath)
	if err != nil {
		return fmt.Errorf("open script file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var firstLine string
	if scanner.Scan() {
		firstLine = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read script file: %w", err)
	}

	var interpreter string
	var interpreterArgs []string

	if strings.HasPrefix(firstLine, "#!") {
		shebang := strings.TrimPrefix(firstLine, "#!")
		shebang = strings.TrimSpace(shebang)
		parts := strings.Fields(shebang)
		if len(parts) > 0 {
			interpreter = parts[0]
			if len(parts) > 1 {
				interpreterArgs = parts[1:]
			}
		}
	}

	if interpreter == "" {
		ext := strings.ToLower(filepath.Ext(scriptPath))
		interpreter = getInterpreterByExtension(ext)
	}

	if interpreter == "" {
		if defaultShell != "" {
			interpreter = defaultShell
		} else {
			interpreter = "sh"
		}
	}

	// apply safe defaults for shell interpreters: -e (exit on error)
	if isShell(interpreter) {
		interpreterArgs = append([]string{"-e"}, interpreterArgs...)
	}

	if _, err := exec.LookPath(interpreter); err != nil {
		return fmt.Errorf("interpreter %q not found in PATH. install it or update script shebang", interpreter)
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

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case sig := <-sigCh:
		pid := cmd.Process.Pid
		_ = proc.KillGroup(pid, syscall.SIGTERM)
		time.AfterFunc(2*time.Second, func() { _ = proc.KillGroup(pid, syscall.SIGKILL) })
		<-done
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
func getInterpreterByExtension(ext string) string {
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
		return "ts-node"
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
