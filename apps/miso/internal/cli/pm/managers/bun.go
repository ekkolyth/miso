package managers

import "github.com/ekkolyth/miso/internal/cli/core"

type Bun struct{}

func (Bun) Name() string { return "bun" }

func (Bun) BuildInstall() core.ExecSpec {
	return core.ExecSpec{Command: "bun", Args: []string{"install"}}
}
func (Bun) BuildAdd(pkgs []string) core.ExecSpec {
	return core.ExecSpec{Command: "bun", Args: append([]string{"add"}, pkgs...)}
}
func (Bun) BuildRemove(pkgs []string) core.ExecSpec {
	return core.ExecSpec{Command: "bun", Args: append([]string{"remove"}, pkgs...)}
}
func (Bun) BuildRun(script string, scriptArgs []string) core.ExecSpec {
	return core.ExecSpec{Command: "bun", Args: append([]string{"run", script}, scriptArgs...)}
}
func (Bun) BuildVersion() core.ExecSpec {
	return core.ExecSpec{Command: "bun", Args: []string{"--version"}}
}
func (Bun) BuildMisox(packageName string, args []string) core.ExecSpec {
	return core.ExecSpec{Command: "bunx", Args: append([]string{packageName}, args...)}
}
