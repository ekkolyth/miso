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
	ActionScripts
	ActionRunMultiple
	ActionPassthrough
	ActionInit
)

type ParsedCLI struct {
	Action       Action
	PackageNames []string
	ScriptName   string
	ScriptNames  []string // For multiple scripts
	ScriptArgs   []string
	Command      string
	Args         []string
}

func ParseCLI(args []string, cfg config.Config) (ParsedCLI, error) {
	if len(args) == 0 {
		return ParsedCLI{}, errors.New("missing command")
	}

	cmd := args[0]

	// Built-in commands are checked first (can be overridden by custom scripts)
	switch cmd {
	case "install", "i":
		// Check if custom script overrides this
		if hasScript(cfg, cmd) {
		return ParsedCLI{
			Action:     ActionScriptOverride,
			ScriptName: cmd,
			ScriptArgs: parseInlineArgs(args[1:]),
		}, nil
	}
		return ParsedCLI{Action: ActionInstall}, nil
	case "add":
		// Check if custom script overrides this
		if hasScript(cfg, cmd) {
			return ParsedCLI{
				Action:     ActionScriptOverride,
				ScriptName: cmd,
				ScriptArgs: parseInlineArgs(args[1:]),
			}, nil
		}
		if len(args) < 2 {
			return ParsedCLI{}, errors.New("usage: miso add <pkg> [<pkg>...]")
		}
		return ParsedCLI{Action: ActionAdd, PackageNames: args[1:]}, nil
	case "remove", "rm":
		// Check if custom script overrides this
		if hasScript(cfg, cmd) {
			return ParsedCLI{
				Action:     ActionScriptOverride,
				ScriptName: cmd,
				ScriptArgs: parseInlineArgs(args[1:]),
			}, nil
		}
		if len(args) < 2 {
			return ParsedCLI{}, errors.New("usage: miso remove <pkg> [<pkg>...]")
		}
		return ParsedCLI{Action: ActionRemove, PackageNames: args[1:]}, nil
	case "run":
		// Check if custom script overrides this
		if hasScript(cfg, cmd) {
			return ParsedCLI{
				Action:     ActionScriptOverride,
				ScriptName: cmd,
				ScriptArgs: parseInlineArgs(args[1:]),
			}, nil
		}
		if len(args) < 2 {
			return ParsedCLI{}, errors.New("usage: miso run <script> [<script>...] [-- <args...>]")
		}
		// Check if multiple scripts are provided
		scripts, scriptArgs := splitMultipleScripts(args[1:])
		if len(scripts) > 1 {
			return ParsedCLI{Action: ActionRunMultiple, ScriptNames: scripts, ScriptArgs: scriptArgs}, nil
		}
		if len(scripts) == 0 {
			return ParsedCLI{}, errors.New("usage: miso run <script> [-- <args...>]")
		}
		script := scripts[0]
		if hasScript(cfg, script) {
			return ParsedCLI{Action: ActionScriptOverride, ScriptName: script, ScriptArgs: scriptArgs}, nil
		}
		return ParsedCLI{Action: ActionRun, ScriptName: script, ScriptArgs: scriptArgs}, nil
	case "dev":
		// Check if custom script overrides this
		inlineArgs := parseInlineArgs(args[1:])
		if hasScript(cfg, "dev") {
			return ParsedCLI{Action: ActionScriptOverride, ScriptName: "dev", ScriptArgs: inlineArgs}, nil
		}
		// Built-in: dev is shortcut for "run dev"
		return ParsedCLI{Action: ActionDev, ScriptArgs: inlineArgs}, nil
	case "scripts":
		return ParsedCLI{Action: ActionScripts}, nil
	case "init":
		// Check if custom script overrides this
		if hasScript(cfg, "init") {
			return ParsedCLI{
				Action:     ActionScriptOverride,
				ScriptName: "init",
				ScriptArgs: parseInlineArgs(args[1:]),
			}, nil
		}
		return ParsedCLI{Action: ActionInit}, nil
	}

	// Not a built-in command - check if it's a custom script
	if hasScript(cfg, cmd) {
		return ParsedCLI{
			Action:     ActionScriptOverride,
			ScriptName: cmd,
			ScriptArgs: parseInlineArgs(args[1:]),
		}, nil
	}

	// Not a built-in and not a custom script - passthrough to package manager
	return ParsedCLI{
		Action:  ActionPassthrough,
		Command: cmd,
		Args:    args[1:],
	}, nil
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

func splitMultipleScripts(rest []string) ([]string, []string) {
	if len(rest) == 0 {
		return nil, nil
	}
	var scripts []string
	var scriptArgs []string
	for i, arg := range rest {
		if arg == "--" {
			scriptArgs = rest[i+1:]
			break
		}
		scripts = append(scripts, arg)
	}
	return scripts, scriptArgs
}
