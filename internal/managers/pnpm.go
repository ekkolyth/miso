package managers

import "github.com/ekkolyth/miso/internal/cli"

type Pnpm struct{}

func (Pnpm) Name() string { return "pnpm" }

func (Pnpm) BuildInstall() cli.ExecSpec {
	return cli.ExecSpec{Command: "pnpm", Args: []string{"install"}}
}
func (Pnpm) BuildAdd(pkgs []string) cli.ExecSpec {
	return cli.ExecSpec{Command: "pnpm", Args: append([]string{"add"}, pkgs...)}
}
func (Pnpm) BuildRemove(pkgs []string) cli.ExecSpec {
	return cli.ExecSpec{Command: "pnpm", Args: append([]string{"remove"}, pkgs...)}
}
func (Pnpm) BuildRun(script string, scriptArgs []string) cli.ExecSpec {
	return cli.ExecSpec{Command: "pnpm", Args: append([]string{"run", script}, scriptArgs...)}
}
func (Pnpm) BuildVersion() cli.ExecSpec {
	return cli.ExecSpec{Command: "pnpm", Args: []string{"--version"}}
}



