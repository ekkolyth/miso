package managers

import "github.com/ekkolyth/miso/internal/cli/core"

type Npm struct{}

func (Npm) Name() string { return "npm" }

func (Npm) BuildInstall() core.ExecSpec {
	return core.ExecSpec{Command: "npm", Args: []string{"install"}}
}
func (Npm) BuildAdd(pkgs []string) core.ExecSpec {
	return core.ExecSpec{Command: "npm", Args: append([]string{"install"}, pkgs...)}
}
func (Npm) BuildRemove(pkgs []string) core.ExecSpec {
	return core.ExecSpec{Command: "npm", Args: append([]string{"uninstall"}, pkgs...)}
}
func (Npm) BuildRun(script string, scriptArgs []string) core.ExecSpec {
	return core.ExecSpec{Command: "npm", Args: append([]string{"run", script}, scriptArgs...)}
}
func (Npm) BuildVersion() core.ExecSpec {
	return core.ExecSpec{Command: "npm", Args: []string{"--version"}}
}
func (Npm) BuildMisox(packageName string, args []string) core.ExecSpec {
	return core.ExecSpec{Command: "npx", Args: append([]string{packageName}, args...)}
}
