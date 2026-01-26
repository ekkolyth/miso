package core

import (
	"errors"
	"strings"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/cli/scripts"
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
	ActionVersion
	ActionMisox
	ActionUpgrade
	ActionScriptFolder
	ActionScriptPackageJSON
)

type ParsedCLI struct {
	Action       Action
	PackageNames []string
	PackageName  string // For misox command
	ScriptName   string
	ScriptNames  []string // For multiple scripts
	ScriptArgs   []string
	Command      string
	Args         []string
	Local        bool // For upgrade command --local flag
}

func ParseCLI(args []string, cfg config.Config, root string) (ParsedCLI, error) {
	if len(args) == 0 {
		return ParsedCLI{}, errors.New("missing command")
	}

	cmd := args[0]

	// Built-in commands are checked first (can be overridden by custom scripts)
	switch cmd {
	case "install", "i":
		// Check if script overrides this (scripts folder or package.json)
		if resolved, err := scripts.ResolveScript(cmd, root, cfg); err == nil && resolved.Source != scripts.ScriptSourceNone {
			return buildScriptAction(resolved, cmd, parseInlineArgs(args[1:])), nil
		}
		return ParsedCLI{Action: ActionInstall}, nil
	case "add":
		// Check if script overrides this
		if resolved, err := scripts.ResolveScript(cmd, root, cfg); err == nil && resolved.Source != scripts.ScriptSourceNone {
			return buildScriptAction(resolved, cmd, parseInlineArgs(args[1:])), nil
		}
		if len(args) < 2 {
			return ParsedCLI{}, errors.New("usage: miso add <pkg> [<pkg>...]")
		}
		return ParsedCLI{Action: ActionAdd, PackageNames: args[1:]}, nil
	case "remove", "rm":
		// Check if script overrides this
		if resolved, err := scripts.ResolveScript(cmd, root, cfg); err == nil && resolved.Source != scripts.ScriptSourceNone {
			return buildScriptAction(resolved, cmd, parseInlineArgs(args[1:])), nil
		}
		if len(args) < 2 {
			return ParsedCLI{}, errors.New("usage: miso remove <pkg> [<pkg>...]")
		}
		return ParsedCLI{Action: ActionRemove, PackageNames: args[1:]}, nil
	case "run":
		if len(args) < 2 {
			return ParsedCLI{}, errors.New("usage: miso run <script> [<script>...] [-- <args...>]")
		}
		// Check if multiple scripts are provided
		scriptNames, scriptArgs := splitMultipleScripts(args[1:])
		if len(scriptNames) > 1 {
			return ParsedCLI{Action: ActionRunMultiple, ScriptNames: scriptNames, ScriptArgs: scriptArgs}, nil
		}
		if len(scriptNames) == 0 {
			return ParsedCLI{}, errors.New("usage: miso run <script> [-- <args...>]")
		}
		script := scriptNames[0]
		// Check scripts folder and package.json
		if resolved, err := scripts.ResolveScript(script, root, cfg); err == nil && resolved.Source != scripts.ScriptSourceNone {
			return buildScriptAction(resolved, script, scriptArgs), nil
		}
		// Fall back to package manager
		return ParsedCLI{Action: ActionRun, ScriptName: script, ScriptArgs: scriptArgs}, nil
	case "dev":
		inlineArgs := parseInlineArgs(args[1:])
		// Check scripts folder and package.json
		if resolved, err := scripts.ResolveScript("dev", root, cfg); err == nil && resolved.Source != scripts.ScriptSourceNone {
			return buildScriptAction(resolved, "dev", inlineArgs), nil
		}
		// Built-in: dev is shortcut for "run dev"
		return ParsedCLI{Action: ActionDev, ScriptArgs: inlineArgs}, nil
	case "scripts":
		return ParsedCLI{Action: ActionScripts}, nil
	case "init":
		// Check if script overrides this
		if resolved, err := scripts.ResolveScript("init", root, cfg); err == nil && resolved.Source != scripts.ScriptSourceNone {
			return buildScriptAction(resolved, "init", parseInlineArgs(args[1:])), nil
		}
		return ParsedCLI{Action: ActionInit}, nil
	case "version", "v":
		// Check if script overrides this
		if resolved, err := scripts.ResolveScript("version", root, cfg); err == nil && resolved.Source != scripts.ScriptSourceNone {
			return buildScriptAction(resolved, "version", parseInlineArgs(args[1:])), nil
		}
		return ParsedCLI{Action: ActionVersion}, nil
	case "misox":
		// Check if script overrides this
		if resolved, err := scripts.ResolveScript("misox", root, cfg); err == nil && resolved.Source != scripts.ScriptSourceNone {
			return buildScriptAction(resolved, "misox", parseInlineArgs(args[1:])), nil
		}
		if len(args) < 2 {
			return ParsedCLI{}, errors.New("usage: miso misox <package> [args...]")
		}
		packageName := args[1]
		remainingArgs := args[2:]
		return ParsedCLI{
			Action:      ActionMisox,
			PackageName: packageName,
			Args:        remainingArgs,
		}, nil
	case "upgrade":
		// Check if script overrides this
		if resolved, err := scripts.ResolveScript("upgrade", root, cfg); err == nil && resolved.Source != scripts.ScriptSourceNone {
			return buildScriptAction(resolved, "upgrade", parseInlineArgs(args[1:])), nil
		}
		// Check for --local flag
		local := false
		remainingArgs := args[1:]
		for i, arg := range remainingArgs {
			if arg == "--local" {
				local = true
				remainingArgs = append(remainingArgs[:i], remainingArgs[i+1:]...)
				break
			}
		}
		return ParsedCLI{
			Action: ActionUpgrade,
			Local:  local,
			Args:   remainingArgs,
		}, nil
	}

	// Not a built-in command - check scripts folder and package.json
	resolved, err := scripts.ResolveScript(cmd, root, cfg)
	if err != nil {
		// If error contains "multiple scripts", it's a conflict - return the error
		if strings.Contains(err.Error(), "multiple scripts") {
			return ParsedCLI{}, err
		}
		// Other errors (like discovery errors) - log but continue to passthrough
		// Discovery errors are non-fatal and shouldn't block passthrough
	}
	if err == nil && resolved.Source != scripts.ScriptSourceNone {
		return buildScriptAction(resolved, cmd, parseInlineArgs(args[1:])), nil
	}

	// Not a built-in and not a script - passthrough to package manager
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

// build script action from resolved script
func buildScriptAction(resolved scripts.ResolvedScript, name string, args []string) ParsedCLI {
	switch resolved.Source {
	case scripts.ScriptSourceFolder:
		return ParsedCLI{
			Action:     ActionScriptFolder,
			ScriptName: name,
			ScriptArgs: args,
			Command:    resolved.Path, // store file path in Command field
		}
	case scripts.ScriptSourcePackageJSON:
		return ParsedCLI{
			Action:     ActionScriptPackageJSON,
			ScriptName: name,
			ScriptArgs: args,
			Command:    resolved.Path, // store command in Command field
		}
	default:
		return ParsedCLI{
			Action:     ActionPassthrough,
			Command:    name,
			Args:       args,
		}
	}
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
