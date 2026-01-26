package managers

import "github.com/ekkolyth/miso/internal/cli/core"

type Pnpm struct{}

func (Pnpm) Name() string { return "pnpm" }

func (Pnpm) BuildInstall() core.ExecSpec {
	return core.ExecSpec{Command: "pnpm", Args: []string{"install"}}
}
func (Pnpm) BuildAdd(pkgs []string) core.ExecSpec {
	return core.ExecSpec{Command: "pnpm", Args: append([]string{"add"}, pkgs...)}
}
func (Pnpm) BuildRemove(pkgs []string) core.ExecSpec {
	return core.ExecSpec{Command: "pnpm", Args: append([]string{"remove"}, pkgs...)}
}
func (Pnpm) BuildRun(script string, scriptArgs []string) core.ExecSpec {
	return core.ExecSpec{Command: "pnpm", Args: append([]string{"run", script}, scriptArgs...)}
}
func (Pnpm) BuildVersion() core.ExecSpec {
	return core.ExecSpec{Command: "pnpm", Args: []string{"--version"}}
}
func (Pnpm) BuildMisox(packageName string, args []string) core.ExecSpec {
	return core.ExecSpec{Command: "pnpm", Args: append([]string{"dlx", packageName}, args...)}
}
