package cli

import (
	"errors"
)

type Action int

const (
	ActionInstall Action = iota
	ActionAdd
	ActionRemove
	ActionRun
	ActionDev
)

type ParsedCLI struct {
	Action       Action
	PackageNames []string
	ScriptName   string
	ScriptArgs   []string
}

func ParseCLI(args []string) (ParsedCLI, error) {
	if len(args) == 0 {
		return ParsedCLI{}, errors.New("missing command")
	}
	switch args[0] {
	case "install":
		return ParsedCLI{Action: ActionInstall}, nil
	case "add":
		if len(args) < 2 {
			return ParsedCLI{}, errors.New("usage: miso add <pkg> [<pkg>...]")
		}
		return ParsedCLI{Action: ActionAdd, PackageNames: args[1:]}, nil
	case "remove":
		if len(args) < 2 {
			return ParsedCLI{}, errors.New("usage: miso remove <pkg> [<pkg>...]")
		}
		return ParsedCLI{Action: ActionRemove, PackageNames: args[1:]}, nil
	case "run":
		if len(args) < 2 {
			return ParsedCLI{}, errors.New("usage: miso run <script> [-- <args...>]")
		}
		script, scriptArgs := splitScriptArgs(args[1:])
		if script == "" {
			return ParsedCLI{}, errors.New("usage: miso run <script> [-- <args...>]")
		}
		return ParsedCLI{Action: ActionRun, ScriptName: script, ScriptArgs: scriptArgs}, nil
	case "dev":
		_, scriptArgs := splitScriptArgs(args[1:])
		return ParsedCLI{Action: ActionDev, ScriptArgs: scriptArgs}, nil
	default:
		return ParsedCLI{}, errors.New("unknown command: " + args[0])
	}
}

func splitScriptArgs(rest []string) (string, []string) {
	if len(rest) == 0 {
		return "", nil
	}
	script := rest[0]
	for i, a := range rest {
		if a == "--" {
			return script, rest[i+1:]
		}
	}
	return script, nil
}


