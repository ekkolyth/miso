package core

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type ExecSpec struct {
	Command string
	Args    []string
}

type Manager interface {
	Name() string
	BuildInstall() ExecSpec
	BuildAdd(packageNames []string) ExecSpec
	BuildRemove(packageNames []string) ExecSpec
	BuildRun(scriptName string, scriptArgs []string) ExecSpec
	BuildVersion() ExecSpec
	BuildMisox(packageName string, args []string) ExecSpec
}

func Exec(spec ExecSpec, detectedManager string, workDir string) error {
	if _, err := exec.LookPath(spec.Command); err != nil {
		return fmt.Errorf("%s is not on PATH", spec.Command)
	}

	cmd := exec.Command(spec.Command, spec.Args...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ExecScript(command string, args []string) error {
	full := command
	if len(args) > 0 {
		full = fmt.Sprintf("%s %s", command, shellJoin(args))
	}
	cmd := exec.Command("/bin/sh", "-c", full)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func shellJoin(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, a := range args {
		if a == "" {
			quoted = append(quoted, "''")
			continue
		}
		if strings.ContainsAny(a, " \t\n\"'`!$&|<>") {
			a = "'" + strings.ReplaceAll(a, "'", "'\"'\"'") + "'"
		}
		quoted = append(quoted, a)
	}
	return strings.Join(quoted, " ")
}
