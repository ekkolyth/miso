package scripts

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// execute script file with shebang detection and extension-based interpreter selection
func ExecScriptFile(scriptPath string, args []string, workDir string) error {
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
		info, err := os.Stat(scriptPath)
		if err != nil {
			return fmt.Errorf("stat script file: %w", err)
		}
		if info.Mode().Perm()&0111 != 0 {
			cmd := exec.Command(scriptPath, args...)
			if workDir != "" {
				cmd.Dir = workDir
			}
			cmd.Stdout = os.Stdout
			cmd.Stdin = os.Stdin
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
		return fmt.Errorf("no interpreter found for script %q. add shebang or use known extension", scriptPath)
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
	return cmd.Run()
}

// get interpreter by file extension
func getInterpreterByExtension(ext string) string {
	switch ext {
	case ".sh", ".bash":
		return "sh"
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
