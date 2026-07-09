package manager

import (
	"bytes"
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
	BuildDlx(packageName string, args []string) ExecSpec
}

func Exec(spec ExecSpec, workDir string) error {
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

// func ExecScript(command string, args []string) error {
// 	full := command
// 	if len(args) > 0 {
// 		full = fmt.Sprintf("%s %s", command, shellJoin(args))
// 	}
// 	cmd := exec.Command("/bin/sh", "-c", full)
// 	cmd.Stdout = os.Stdout
// 	cmd.Stdin = os.Stdin
// 	cmd.Stderr = os.Stderr
// 	return cmd.Run()
// }

// ResolveManagerVersion runs the manager binary's version command, captures its
// output, and returns "name@version" (e.g. "bun@1.2.3") for use in the
// package.json packageManager field (Corepack spec). Falls back to the bare
// managerName on any error (binary not found, unexpected output, etc.) so that
// init never fails just because the version cannot be determined.
func ResolveManagerVersion(managerName string) string {
	driver, ok := GetManager(managerName)
	if !ok {
		return managerName
	}
	spec := driver.BuildVersion()
	if _, err := exec.LookPath(spec.Command); err != nil {
		return managerName
	}
	var buf bytes.Buffer
	cmd := exec.Command(spec.Command, spec.Args...)
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		return managerName
	}
	version := strings.TrimSpace(buf.String())
	// Some managers (e.g. yarn) prefix output with "yarn/1.2.3 …"; normalise to
	// just the semver token (first word that looks like x.y…).
	for _, token := range strings.Fields(version) {
		token = strings.TrimPrefix(token, managerName+"/")
		if len(token) > 0 && (token[0] >= '0' && token[0] <= '9') {
			version = token
			break
		}
	}
	if version == "" {
		return managerName
	}
	return managerName + "@" + version
}
