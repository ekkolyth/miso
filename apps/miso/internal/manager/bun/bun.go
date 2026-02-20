package bun

import "github.com/ekkolyth/miso/internal/manager"

type Bun struct{}

func (Bun) Name() string { return "bun" }

func (Bun) BuildInstall() manager.ExecSpec {
	return manager.ExecSpec{Command: "bun", Args: []string{"install"}}
}
func (Bun) BuildAdd(pkgs []string) manager.ExecSpec {
	return manager.ExecSpec{Command: "bun", Args: append([]string{"add"}, pkgs...)}
}
func (Bun) BuildRemove(pkgs []string) manager.ExecSpec {
	return manager.ExecSpec{Command: "bun", Args: append([]string{"remove"}, pkgs...)}
}
func (Bun) BuildRun(script string, scriptArgs []string) manager.ExecSpec {
	return manager.ExecSpec{Command: "bun", Args: append([]string{"run", script}, scriptArgs...)}
}
func (Bun) BuildVersion() manager.ExecSpec {
	return manager.ExecSpec{Command: "bun", Args: []string{"--version"}}
}
func (Bun) BuildMisox(packageName string, args []string) manager.ExecSpec {
	return manager.ExecSpec{Command: "bunx", Args: append([]string{packageName}, args...)}
}
