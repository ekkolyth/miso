package cli

import (
	"errors"

	"github.com/ekkolyth/miso/internal/config"
)

type Action int

const (
	ActionInstall Action = iota
	ActionAdd
	ActionRemove
	ActionRun
	ActionDev
	ActionScriptOverride
)

type ParsedCLI struct {
	Action       Action
	PackageNames []string
	ScriptName   string
	ScriptArgs   []string
}

func ParseCLI(args []string, cfg config.Config) (ParsedCLI, error) {
	if len(args) == 0 {
		return ParsedCLI{}, errors.New("missing command")
	}

	if cmd := args[0]; hasScript(cfg, cmd) {
		return ParsedCLI{
			Action:     ActionScriptOverride,
			ScriptName: cmd,
			ScriptArgs: parseInlineArgs(args[1:]),
		}, nil
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
		if hasScript(cfg, script) {
			return ParsedCLI{Action: ActionScriptOverride, ScriptName: script, ScriptArgs: scriptArgs}, nil
		}
		return ParsedCLI{Action: ActionRun, ScriptName: script, ScriptArgs: scriptArgs}, nil
	case "dev":
		inlineArgs := parseInlineArgs(args[1:])
		if hasScript(cfg, "dev") {
			return ParsedCLI{Action: ActionScriptOverride, ScriptName: "dev", ScriptArgs: inlineArgs}, nil
		}
		return ParsedCLI{Action: ActionDev, ScriptArgs: inlineArgs}, nil
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

func parseInlineArgs(rest []string) []string {
	for i, a := range rest {
		if a == "--" {
			return rest[i+1:]
		}
	}
	return rest
}

func hasScript(cfg config.Config, name string) bool {
	if len(cfg.Scripts) == 0 {
		return false
	}
	_, ok := cfg.Scripts[name]
	return ok
}
