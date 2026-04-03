package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ekkolyth/miso/internal/cli/scripting"
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
	ActionVersion
	ActionUpgrade
	ActionScriptFolder
	ActionScriptPackageJSON
	ActionEnv
	ActionWorkspaceScript
)

type ParsedCLI struct {
	Action        Action
	PackageNames  []string
	ScriptName    string
	ScriptNames   []string // For multiple scripts
	ScriptArgs    []string
	Command       string
	Args          []string
	WorkspaceName string // For @workspace/script syntax
}

func ParseCLI(args []string, cfg config.Config, root string) (ParsedCLI, error) {
	if len(args) == 0 {
		return ParsedCLI{}, errors.New("missing command")
	}

	cmd := args[0]

	// check built-in commands (can be overridden)
	switch cmd {
	case "install", "i":
		// check script override
		if resolved, err := scripting.ResolveScript(cmd, root, cfg); err == nil && resolved.Source != scripting.ScriptSourceNone {
			return buildScriptAction(resolved, cmd, parseInlineArgs(args[1:])), nil
		}
		return ParsedCLI{Action: ActionInstall}, nil
	case "add":
		// check script override
		if resolved, err := scripting.ResolveScript(cmd, root, cfg); err == nil && resolved.Source != scripting.ScriptSourceNone {
			return buildScriptAction(resolved, cmd, parseInlineArgs(args[1:])), nil
		}
		if len(args) < 2 {
			return ParsedCLI{}, errors.New("usage: miso add <pkg> [<pkg>...]")
		}
		return ParsedCLI{Action: ActionAdd, PackageNames: args[1:]}, nil
	case "remove", "rm":
		// check script override
		if resolved, err := scripting.ResolveScript(cmd, root, cfg); err == nil && resolved.Source != scripting.ScriptSourceNone {
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
		// check for multiple scripts
		scriptNames, scriptArgs := splitMultipleScripts(args[1:])
		if len(scriptNames) > 1 {
			return ParsedCLI{Action: ActionRunMultiple, ScriptNames: scriptNames, ScriptArgs: scriptArgs}, nil
		}
		if len(scriptNames) == 0 {
			return ParsedCLI{}, errors.New("usage: miso run <script> [-- <args...>]")
		}
		script := scriptNames[0]
		// check scripts folder and package.json
		if resolved, err := scripting.ResolveScript(script, root, cfg); err == nil && resolved.Source != scripting.ScriptSourceNone {
			return buildScriptAction(resolved, script, scriptArgs), nil
		}
		// fall back to package manager
		return ParsedCLI{Action: ActionRun, ScriptName: script, ScriptArgs: scriptArgs}, nil
	case "dev":
		inlineArgs := parseInlineArgs(args[1:])
		// check scripts folder and package.json
		if resolved, err := scripting.ResolveScript("dev", root, cfg); err == nil && resolved.Source != scripting.ScriptSourceNone {
			return buildScriptAction(resolved, "dev", inlineArgs), nil
		}
		// dev is shortcut for "run dev"
		return ParsedCLI{Action: ActionDev, ScriptArgs: inlineArgs}, nil
	case "scripts":
		return ParsedCLI{Action: ActionScripts}, nil
	case "env":
		return ParsedCLI{Action: ActionEnv}, nil
	case "init":
		return ParsedCLI{Action: ActionInit}, nil
	case "version", "v":
		return ParsedCLI{Action: ActionVersion}, nil
	case "upgrade":
		// check script override
		if resolved, err := scripting.ResolveScript("upgrade", root, cfg); err == nil && resolved.Source != scripting.ScriptSourceNone {
			return buildScriptAction(resolved, "upgrade", parseInlineArgs(args[1:])), nil
		}
		return ParsedCLI{
			Action: ActionUpgrade,
			Args:   args[1:],
		}, nil
	}

	// check for @workspace/script syntax
	if strings.HasPrefix(cmd, "@") {
		inner := cmd[1:] // strip leading @
		lastSlash := strings.LastIndex(inner, "/")
		if lastSlash < 0 {
			return ParsedCLI{}, fmt.Errorf("invalid workspace command %q: usage: @<workspace>/<script>", cmd)
		}
		workspaceName := inner[:lastSlash]
		scriptName := inner[lastSlash+1:]
		if workspaceName == "" || scriptName == "" {
			return ParsedCLI{}, fmt.Errorf("invalid workspace command %q: usage: @<workspace>/<script>", cmd)
		}
		return ParsedCLI{
			Action:        ActionWorkspaceScript,
			WorkspaceName: workspaceName,
			ScriptName:    scriptName,
			ScriptArgs:    parseInlineArgs(args[1:]),
		}, nil
	}

	// check scripts folder and package.json
	resolved, err := scripting.ResolveScript(cmd, root, cfg)
	if err != nil {
		// handle script conflicts
		if strings.Contains(err.Error(), "multiple scripts") {
			return ParsedCLI{}, err
		}
		// discovery errors are non-fatal
	}
	if resolved.Source != scripting.ScriptSourceNone {
		return buildScriptAction(resolved, cmd, parseInlineArgs(args[1:])), nil
	}

	// passthrough to package manager
	return ParsedCLI{
		Action:  ActionPassthrough,
		Command: cmd,
		Args:    args[1:],
	}, nil
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
func buildScriptAction(resolved scripting.ResolvedScript, name string, args []string) ParsedCLI {
	switch resolved.Source {
	case scripting.ScriptSourceFolder:
		return ParsedCLI{
			Action:     ActionScriptFolder,
			ScriptName: name,
			ScriptArgs: args,
			Command:    resolved.Path, // store file path in Command field
		}
	case scripting.ScriptSourcePackageJSON:
		return ParsedCLI{
			Action:     ActionScriptPackageJSON,
			ScriptName: name,
			ScriptArgs: args,
			Command:    resolved.Path, // store command in Command field
		}
	default:
		return ParsedCLI{
			Action:  ActionPassthrough,
			Command: name,
			Args:    args,
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
