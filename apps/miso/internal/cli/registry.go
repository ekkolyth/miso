package cli

// built-in miso command: help, completion, dispatch
type Command struct {
	Name         string
	Aliases      []string
	Summary      string
	Usage        string // arg hint shown after the name in help, e.g. "<pkg>"
	Meta         bool   // handled in the pre-config global switch (no project/manager needed)
	NeedsProject bool
}

// Builtins is the source of truth for miso's built-in commands. It excludes the
// internal "__complete" protocol and the standalone "misox" shim — misox is a
// separate binary, not a miso subcommand (dropped in commit 71b4a4ca).
var Builtins = []Command{
	{Name: "install", Aliases: []string{"i"}, Summary: "install dependencies", NeedsProject: true},
	{Name: "add", Summary: "add dependencies", Usage: "<pkg>", NeedsProject: true},
	{Name: "remove", Aliases: []string{"rm"}, Summary: "remove dependencies", Usage: "<pkg>", NeedsProject: true},
	{Name: "run", Summary: "run a script", Usage: "<script>", NeedsProject: true},
	{Name: "dev", Summary: "run the dev script", NeedsProject: true},
	{Name: "scripts", Summary: "list scripts", NeedsProject: true},
	{Name: "env", Summary: "validate environment variables", NeedsProject: true},
	{Name: "init", Summary: "scaffold a miso.json in this project", Meta: true},
	{Name: "upgrade", Summary: "update miso to the latest version", Usage: "[--local]", Meta: true},
	{Name: "skills", Summary: "manage miso agent skills", Meta: true},
	{Name: "completion", Summary: "print a shell completion script", Usage: "<bash|zsh|fish>", Meta: true},
	{Name: "version", Aliases: []string{"v"}, Summary: "print the miso version", Meta: true},
	{Name: "help", Summary: "show this help", Meta: true},
}

// name or alias → canonical command
func LookupBuiltin(token string) (Command, bool) {
	for _, cmd := range Builtins {
		if cmd.Name == token {
			return cmd, true
		}
		for _, alias := range cmd.Aliases {
			if alias == token {
				return cmd, true
			}
		}
	}
	return Command{}, false
}

// every name + alias, for shell completion
func BuiltinNames() []string {
	names := make([]string, 0, len(Builtins))
	for _, cmd := range Builtins {
		names = append(names, cmd.Name)
		names = append(names, cmd.Aliases...)
	}
	return names
}
