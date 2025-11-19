package cli

type ExecSpec struct {
	Command string
	Args    []string
}

type Manager interface {
	Name() string
	BuildInstall() ExecSpec
	BuildAdd(packageNames []string) ExecSpec
	BuildRemove(packageNames []string) ExecSpec
	BuildRun(scriptName string, scriptArgs []string) ExecSpec
	BuildVersion() ExecSpec
	BuildMisox(packageName string, args []string) ExecSpec
}



