package pnpm

import "github.com/ekkolyth/miso/internal/manager"

type Pnpm struct{}

func (Pnpm) Name() string { return "pnpm" }

func (Pnpm) BuildInstall() manager.ExecSpec {
	return manager.ExecSpec{Command: "pnpm", Args: []string{"install"}}
}
func (Pnpm) BuildAdd(pkgs []string) manager.ExecSpec {
	return manager.ExecSpec{Command: "pnpm", Args: append([]string{"add"}, pkgs...)}
}
func (Pnpm) BuildRemove(pkgs []string) manager.ExecSpec {
	return manager.ExecSpec{Command: "pnpm", Args: append([]string{"remove"}, pkgs...)}
}
func (Pnpm) BuildRun(script string, scriptArgs []string) manager.ExecSpec {
	return manager.ExecSpec{Command: "pnpm", Args: append([]string{"run", script}, scriptArgs...)}
}
func (Pnpm) BuildVersion() manager.ExecSpec {
	return manager.ExecSpec{Command: "pnpm", Args: []string{"--version"}}
}
func (Pnpm) BuildDlx(packageName string, args []string) manager.ExecSpec {
	return manager.ExecSpec{Command: "pnpm", Args: append([]string{"dlx", packageName}, args...)}
}
