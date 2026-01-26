package managers

import "github.com/ekkolyth/miso/internal/cli/core"

type Yarn struct{}

func (Yarn) Name() string { return "yarn" }

func (Yarn) BuildInstall() core.ExecSpec {
	return core.ExecSpec{Command: "yarn", Args: []string{"install"}}
}
func (Yarn) BuildAdd(pkgs []string) core.ExecSpec {
	return core.ExecSpec{Command: "yarn", Args: append([]string{"add"}, pkgs...)}
}
func (Yarn) BuildRemove(pkgs []string) core.ExecSpec {
	return core.ExecSpec{Command: "yarn", Args: append([]string{"remove"}, pkgs...)}
}
func (Yarn) BuildRun(script string, scriptArgs []string) core.ExecSpec {
	return core.ExecSpec{Command: "yarn", Args: append([]string{"run", script}, scriptArgs...)}
}
func (Yarn) BuildVersion() core.ExecSpec {
	return core.ExecSpec{Command: "yarn", Args: []string{"--version"}}
}
func (Yarn) BuildMisox(packageName string, args []string) core.ExecSpec {
	return core.ExecSpec{Command: "yarn", Args: append([]string{"dlx", packageName}, args...)}
}
