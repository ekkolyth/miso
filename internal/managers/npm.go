package managers

import "github.com/ekkolyth/miso/internal/cli"

type Npm struct{}

func (Npm) Name() string { return "npm" }

func (Npm) BuildInstall() cli.ExecSpec {
	return cli.ExecSpec{Command: "npm", Args: []string{"install"}}
}
func (Npm) BuildAdd(pkgs []string) cli.ExecSpec {
	return cli.ExecSpec{Command: "npm", Args: append([]string{"install"}, pkgs...)}
}
func (Npm) BuildRemove(pkgs []string) cli.ExecSpec {
	return cli.ExecSpec{Command: "npm", Args: append([]string{"uninstall"}, pkgs...)}
}
func (Npm) BuildRun(script string, scriptArgs []string) cli.ExecSpec {
	return cli.ExecSpec{Command: "npm", Args: append([]string{"run", script}, scriptArgs...)}
}
func (Npm) BuildVersion() cli.ExecSpec {
	return cli.ExecSpec{Command: "npm", Args: []string{"--version"}}
}



