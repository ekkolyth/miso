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

// simple mode: folder scripts only, no package.json fallback.
// main.go short-circuits simple mode before ParseCLI reaches here — this keeps
// resolution correct for any other ParseCLI caller too.
func resolveScript(name, root string, cfg config.Config) (scripting.ResolvedScript, error) {
	if cfg.SimpleMode() {
		return scripting.ResolveScriptFolderOnly(name, root, cfg)
	}
	return scripting.ResolveScript(name, root, cfg)
}

// scriptOverride resolves cmd as a folder/package.json script. It returns
// (action, true, nil) when a single override exists, (_, false, nil) when none
// does, and a fatal error when the name is defined in more than one place
// (ErrAmbiguousScript). An ambiguous name must never fall through to the
// built-in or passthrough — miso refuses to silently guess which you meant.
func scriptOverride(cmd string, args []string, root string, cfg config.Config) (ParsedCLI, bool, error) {
	resolved, err := resolveScript(cmd, root, cfg)
	if err != nil {
		if errors.Is(err, scripting.ErrAmbiguousScript) {
			return ParsedCLI{}, false, err
		}
		return ParsedCLI{}, false, nil // other discovery errors are non-fatal
	}
	if resolved.Source == scripting.ScriptSourceNone {
		return ParsedCLI{}, false, nil
	}
	return buildScriptAction(resolved, cmd, args), true, nil
}

func ParseCLI(args []string, cfg config.Config, root string) (ParsedCLI, error) {
	if len(args) == 0 {
		return ParsedCLI{}, errors.New("missing command")
	}

	cmd := args[0]

	// check built-in commands (can be overridden)
	switch cmd {
	case "install", "i":
		if override, ok, err := scriptOverride(cmd, parseInlineArgs(args[1:]), root, cfg); err != nil {
			return ParsedCLI{}, err
		} else if ok {
			return override, nil
		}
		return ParsedCLI{Action: ActionInstall}, nil
	case "add":
		if override, ok, err := scriptOverride(cmd, parseInlineArgs(args[1:]), root, cfg); err != nil {
			return ParsedCLI{}, err
		} else if ok {
			return override, nil
		}
		if len(args) < 2 {
			return ParsedCLI{}, errors.New("usage: miso add <pkg> [<pkg>...]")
		}
		return ParsedCLI{Action: ActionAdd, PackageNames: args[1:]}, nil
	case "remove", "rm":
		if override, ok, err := scriptOverride(cmd, parseInlineArgs(args[1:]), root, cfg); err != nil {
			return ParsedCLI{}, err
		} else if ok {
			return override, nil
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
		if override, ok, err := scriptOverride(script, scriptArgs, root, cfg); err != nil {
			return ParsedCLI{}, err
		} else if ok {
			return override, nil
		}
		// fall back to package manager
		return ParsedCLI{Action: ActionRun, ScriptName: script, ScriptArgs: scriptArgs}, nil
	case "dev":
		inlineArgs := parseInlineArgs(args[1:])
		if override, ok, err := scriptOverride("dev", inlineArgs, root, cfg); err != nil {
			return ParsedCLI{}, err
		} else if ok {
			return override, nil
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
		if override, ok, err := scriptOverride("upgrade", parseInlineArgs(args[1:]), root, cfg); err != nil {
			return ParsedCLI{}, err
		} else if ok {
			return override, nil
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

	// resolve a folder/package.json script; ambiguous overlaps are fatal
	if override, ok, err := scriptOverride(cmd, parseInlineArgs(args[1:]), root, cfg); err != nil {
		return ParsedCLI{}, err
	} else if ok {
		return override, nil
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
