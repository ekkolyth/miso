package commands

import "github.com/ekkolyth/miso/internal/manager"

// ParseSkillsFlags scans args for --add and --rm flags.
// Returns (add, rm) booleans. Does not modify args.
func ParseSkillsFlags(args []string) (add bool, rm bool) {
	for _, arg := range args {
		switch arg {
		case "--add":
			add = true
		case "--rm":
			rm = true
		}
	}
	return
}

// RunSkillsAdd runs: npx skills add "<repo-url>" --skill '*' --yes
func RunSkillsAdd() error {
	spec := manager.ExecSpec{
		Command: "npx",
		Args:    []string{"skills", "add", "https://github.com/ekkolyth/miso/tree/main/packages/skills", "--skill", "*", "--yes"},
	}
	return manager.Exec(spec, "")
}

// RunSkillsRemove runs: npx skills remove miso-config miso-scripting miso-tui miso-env
func RunSkillsRemove() error {
	spec := manager.ExecSpec{
		Command: "npx",
		Args:    []string{"skills", "remove", "miso-config", "miso-scripting", "miso-tui", "miso-env"},
	}
	return manager.Exec(spec, "")
}
