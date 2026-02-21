package yarn

import "github.com/ekkolyth/miso/internal/manager"

type Yarn struct{}

func (Yarn) Name() string { return "yarn" }

func (Yarn) BuildInstall() manager.ExecSpec {
	return manager.ExecSpec{Command: "yarn", Args: []string{"install"}}
}
func (Yarn) BuildAdd(pkgs []string) manager.ExecSpec {
	return manager.ExecSpec{Command: "yarn", Args: append([]string{"add"}, pkgs...)}
}
func (Yarn) BuildRemove(pkgs []string) manager.ExecSpec {
	return manager.ExecSpec{Command: "yarn", Args: append([]string{"remove"}, pkgs...)}
}
func (Yarn) BuildRun(script string, scriptArgs []string) manager.ExecSpec {
	return manager.ExecSpec{Command: "yarn", Args: append([]string{"run", script}, scriptArgs...)}
}
func (Yarn) BuildVersion() manager.ExecSpec {
	return manager.ExecSpec{Command: "yarn", Args: []string{"--version"}}
}
func (Yarn) BuildMisox(packageName string, args []string) manager.ExecSpec {
	return manager.ExecSpec{Command: "yarn", Args: append([]string{"dlx", packageName}, args...)}
}
