package npm

import "github.com/ekkolyth/miso/internal/manager"

type Npm struct{}

func (Npm) Name() string { return "npm" }

func (Npm) BuildInstall() manager.ExecSpec {
	return manager.ExecSpec{Command: "npm", Args: []string{"install"}}
}
func (Npm) BuildAdd(pkgs []string) manager.ExecSpec {
	return manager.ExecSpec{Command: "npm", Args: append([]string{"install"}, pkgs...)}
}
func (Npm) BuildRemove(pkgs []string) manager.ExecSpec {
	return manager.ExecSpec{Command: "npm", Args: append([]string{"uninstall"}, pkgs...)}
}
func (Npm) BuildRun(script string, scriptArgs []string) manager.ExecSpec {
	return manager.ExecSpec{Command: "npm", Args: append([]string{"run", script}, scriptArgs...)}
}
func (Npm) BuildVersion() manager.ExecSpec {
	return manager.ExecSpec{Command: "npm", Args: []string{"--version"}}
}
func (Npm) BuildMisox(packageName string, args []string) manager.ExecSpec {
	return manager.ExecSpec{Command: "npx", Args: append([]string{packageName}, args...)}
}
