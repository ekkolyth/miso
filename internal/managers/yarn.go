package managers

import "github.com/ekkolyth/miso/internal/cli"

type Yarn struct{}

func (Yarn) Name() string { return "yarn" }

func (Yarn) BuildInstall() cli.ExecSpec {
	return cli.ExecSpec{Command: "yarn", Args: []string{"install"}}
}
func (Yarn) BuildAdd(pkgs []string) cli.ExecSpec {
	return cli.ExecSpec{Command: "yarn", Args: append([]string{"add"}, pkgs...)}
}
func (Yarn) BuildRemove(pkgs []string) cli.ExecSpec {
	return cli.ExecSpec{Command: "yarn", Args: append([]string{"remove"}, pkgs...)}
}
func (Yarn) BuildRun(script string, scriptArgs []string) cli.ExecSpec {
	return cli.ExecSpec{Command: "yarn", Args: append([]string{"run", script}, scriptArgs...)}
}
func (Yarn) BuildVersion() cli.ExecSpec {
	return cli.ExecSpec{Command: "yarn", Args: []string{"--version"}}
}



