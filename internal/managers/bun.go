package managers

import "github.com/ekkolyth/miso/internal/cli"

type Bun struct{}

func (Bun) Name() string { return "bun" }

func (Bun) BuildInstall() cli.ExecSpec {
	return cli.ExecSpec{Command: "bun", Args: []string{"install"}}
}
func (Bun) BuildAdd(pkgs []string) cli.ExecSpec {
	return cli.ExecSpec{Command: "bun", Args: append([]string{"add"}, pkgs...)}
}
func (Bun) BuildRemove(pkgs []string) cli.ExecSpec {
	return cli.ExecSpec{Command: "bun", Args: append([]string{"remove"}, pkgs...)}
}
func (Bun) BuildRun(script string, scriptArgs []string) cli.ExecSpec {
	return cli.ExecSpec{Command: "bun", Args: append([]string{"run", script}, scriptArgs...)}
}



