package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/log"

	"github.com/ekkolyth/miso/internal/cli"
	"github.com/ekkolyth/miso/internal/cli/commands"
	"github.com/ekkolyth/miso/internal/cli/completion"
	"github.com/ekkolyth/miso/internal/cli/env"
	"github.com/ekkolyth/miso/internal/cli/scripting"
	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/harness"
	"github.com/ekkolyth/miso/internal/manager"
	"github.com/ekkolyth/miso/internal/manager/bun"
	"github.com/ekkolyth/miso/internal/manager/npm"
	"github.com/ekkolyth/miso/internal/manager/pnpm"
	"github.com/ekkolyth/miso/internal/manager/yarn"
	"github.com/ekkolyth/miso/internal/tui"
	"github.com/ekkolyth/miso/internal/turbo"
	"github.com/ekkolyth/miso/internal/ui"
	"github.com/ekkolyth/miso/internal/workspace"
)

func init() {
	// Register manager drivers
	manager.RegisterManager("bun", bun.Bun{})
	manager.RegisterManager("npm", npm.Npm{})
	manager.RegisterManager("pnpm", pnpm.Pnpm{})
	manager.RegisterManager("yarn", yarn.Yarn{})
}

func main() {
	args := os.Args[1:]

	// if misox prepend "misox"
	invokedAsMisox := cli.InvokedAsMisox(os.Args[0])
	if invokedAsMisox {
		args = append([]string{"misox"}, args...)
	}

	styles := ui.Default()
	logger := log.NewWithOptions(os.Stderr, log.Options{
		Prefix:          "miso",
		ReportTimestamp: false,
	})
	debug := os.Getenv("MISO_DEBUG") == "1"
	if debug {
		logger.SetLevel(log.DebugLevel)
	}

	originalWorkDir, err := os.Getwd()
	if err != nil {
		cli.Fail(logger, fmt.Errorf("determine working directory: %w", err), false)
	}

	// help and bare invocation print the command reference — works with no project
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		commands.RunHelp()
		return
	}

	// handle global commands before loading config
	if len(args) > 0 {
		// internal completion protocol + standalone misox shim: not user-facing built-ins
		switch args[0] {
		case "__complete":
			completion.Complete(args, originalWorkDir)
			return
		case "misox":
			// only the standalone misox binary dispatches; typed `miso misox` falls through to passthrough
			if !invokedAsMisox {
				break
			}
			if len(args) < 2 {
				fmt.Fprintln(os.Stderr, "usage: misox <package> [args...]")
				os.Exit(1)
			}
			misoxManager := "npm"
			if detected, err := manager.DetectManager(originalWorkDir); err == nil {
				misoxManager = detected
			}
			if err := cli.RunMisox(misoxManager, args[1], args[2:], originalWorkDir); err != nil {
				cli.Fail(logger, err, false)
			}
			return
		}

		// user-facing built-ins, canonicalized (aliases resolved) via the registry
		if cmd, ok := cli.LookupBuiltin(args[0]); ok && cmd.Meta {
			switch cmd.Name {
			case "completion":
				if len(args) < 2 {
					fmt.Fprintln(os.Stderr, "Usage: miso completion [bash|zsh|fish]")
					os.Exit(1)
				}
				switch args[1] {
				case "bash":
					fmt.Print(completion.ScriptBash())
				case "zsh":
					fmt.Print(completion.ScriptZsh())
				case "fish":
					fmt.Print(completion.ScriptFish())
				default:
					fmt.Fprintf(os.Stderr, "Unknown shell: %s (use bash, zsh, or fish)\n", args[1])
					os.Exit(1)
				}
				return
			case "init":
				if err := commands.RunInit(originalWorkDir, styles, logger); err != nil {
					cli.Fail(logger, err, false)
				}
				return
			case "version":
				if err := commands.RunVersion(); err != nil {
					cli.Fail(logger, err, false)
				}
				return
			case "upgrade":
				if err := commands.Upgrade(args[1:]); err != nil {
					cli.Fail(logger, err, false)
				}
				return
			case "skills":
				add, rm, yes := commands.ParseSkillsFlags(args[1:])
				if add && rm {
					cli.Fail(logger, fmt.Errorf("--add and --rm are mutually exclusive"), false)
					return
				}
				if add || rm {
					managerName := "npm"
					if detected, derr := manager.DetectManager(originalWorkDir); derr == nil {
						managerName = detected
					}
					installed := harness.Detect()
					if len(installed) == 0 {
						cli.Fail(logger, fmt.Errorf("no supported harness found on PATH; install one of: Claude Code, OpenCode, Codex, Gemini CLI, Cursor"), false)
						return
					}
					var chosen []string
					if yes {
						for _, entry := range installed {
							chosen = append(chosen, entry.Agent)
						}
					} else {
						selected, serr := harness.Select(installed)
						if serr != nil {
							cli.Fail(logger, serr, false)
							return
						}
						chosen = selected
					}
					if len(chosen) == 0 {
						fmt.Fprintln(os.Stderr, "no harness selected; nothing to do")
						return
					}
					if add {
						if err := commands.RunSkillsAdd(managerName, originalWorkDir, chosen); err != nil {
							cli.Fail(logger, err, false)
						}
						return
					}
					if err := commands.RunSkillsRemove(managerName, originalWorkDir, chosen); err != nil {
						cli.Fail(logger, err, false)
					}
					return
				}
				// neither --add nor --rm: fall through to normal routing (PM passthrough)
			}
		}
	}

	// find project root
	projectRoot, err := cli.FindProjectRoot(originalWorkDir)
	if err != nil {
		cli.Fail(logger, err, false)
	}

	cfg, err := cli.LoadConfig(projectRoot)
	if err != nil {
		cli.Fail(logger, err, false)
	}

	// Simple mode: bypass ParseCLI and EnsureManager entirely.
	// Only meta-commands are handled; everything else is folder script resolution.
	if cfg.SimpleMode() {
		cmd := args[0]

		// Meta-commands that remain in simple mode
		// (init, version, upgrade, completion already handled above)
		switch cmd {
		case "env":
			if err := env.Run(projectRoot, cfg, logger); err != nil {
				os.Exit(1)
			}
			return
		case "scripts":
			if err := scripting.List(cfg, projectRoot, styles, logger); err != nil {
				cli.Fail(logger, err, false)
			}
			return
		}

		// Resolve as folder script only (no package.json fallback)
		resolved, err := scripting.ResolveScriptFolderOnly(cmd, projectRoot, cfg)
		if err != nil {
			cli.Fail(logger, err, false)
		}

		if resolved.Source == scripting.ScriptSourceNone {
			cli.Fail(logger, fmt.Errorf("script '%s' not found in %s", cmd, cfg.Scripts), false)
		}

		scriptArgs := args[1:]

		// Handle --env flag: run env validation before script execution
		if env.HasEnvFlag(scriptArgs) {
			if err := env.Run(projectRoot, cfg, logger); err != nil {
				os.Exit(1)
			}
			scriptArgs = env.StripEnvFlag(scriptArgs)
		}

		// TUI interception for simple mode
		if cfg.TuiEnabled() {
			ran, err := tui.Launch(cfg, cmd, projectRoot, nil)
			if err != nil {
				cli.Fail(logger, err, false)
			}
			if ran {
				return
			}
		}

		target, _ := workspace.ResolveTarget(cmd, nil, projectRoot, cfg)
		processEnv, envErr := env.BuildTargetEnv(projectRoot, cfg, target)
		if envErr != nil {
			cli.Fail(logger, envErr, false)
		}
		detected, _ := manager.DetectManager(projectRoot)
		if err := scripting.ExecScriptFile(resolved.Path, scriptArgs, originalWorkDir, cfg.Shell, detected, processEnv); err != nil {
			cli.Fail(logger, err, false)
		}
		return
	}

	parsed, err := cli.ParseCLI(args, cfg, projectRoot)
	if err != nil {
		cli.Fail(logger, err, true)
	}

	// CWD-aware workspace scoping: when inside a discovered member and the parsed
	// action is a plain script (not already workspace-scoped), try to resolve the
	// script from the current member's scripts folder first.
	if members, _ := workspace.DiscoverMembers(projectRoot, cfg); len(members) > 0 &&
		originalWorkDir != projectRoot && parsed.Action == cli.ActionScriptPackageJSON {
		if member, inWs := workspace.FromCWD(originalWorkDir, members); inWs {
			resolved, _, _, resolveErr := scripting.ResolveWorkspaceScript(
				member.Name, parsed.ScriptName, projectRoot, cfg,
			)
			if resolveErr == nil && (resolved.Source == scripting.ScriptSourceFolder || resolved.Source == scripting.ScriptSourcePackageJSON) {
				parsed.Action = cli.ActionWorkspaceScript
				parsed.WorkspaceName = member.Name
			}
		}
	}

	// env does not need package manager
	if parsed.Action == cli.ActionEnv {
		if err := env.Run(projectRoot, cfg, logger); err != nil {
			os.Exit(1)
		}
		return
	}

	// ensure manager configured
	managerName, cfg, err := cli.EnsureManager(projectRoot, cfg)
	if err != nil {
		cli.Fail(logger, err, false)
	}

	if debug {
		logger.Debug("resolved manager", "manager", managerName)
	}

	// --env flag: run env validation first, then strip from args before passing to command
	cfg, parsed = runEnvIfRequested(projectRoot, cfg, parsed, logger)

	// TUI interception — check if we should launch the TUI instead of normal execution
	if cfg.TuiEnabled() {
		// Only intercept when running from project root (not workspace subdirectory)
		isRoot := true
		if members, _ := workspace.DiscoverMembers(projectRoot, cfg); len(members) > 0 {
			if _, inWs := workspace.FromCWD(originalWorkDir, members); inWs {
				isRoot = false
			}
		}

		if isRoot {
			// ActionScriptFolder is deliberately absent: a folder script runs
			// literally (miso can't wrap its output in the turbo/nx chrome,
			// which needs miso to control the turbo invocation — see
			// tui.DelegateLaunch). Delegation is reached via a package.json
			// script or the built-in dev/run action, where miso reconstructs
			// `turbo run <name>` itself.
			switch parsed.Action {
			case cli.ActionDev, cli.ActionRun, cli.ActionScriptPackageJSON:
				scriptName := parsed.ScriptName
				if parsed.Action == cli.ActionDev {
					scriptName = "dev"
				}

				if cfg.IsDelegated() {
					// Check if this task is overridden by miso's direct orchestration
					_, taskOverridden := cfg.Tasks[scriptName]
					if taskOverridden {
						mgr, ok := manager.GetManager(managerName)
						if !ok {
							cli.Fail(logger, fmt.Errorf("unknown manager: %s", managerName), false)
						}
						ran, err := tui.Launch(cfg, scriptName, projectRoot, mgr)
						if err != nil {
							cli.Fail(logger, err, false)
						}
						if ran {
							return
						}
					} else {
						// Only delegate to turbo/nx if the script is actually a pipeline task
						turboCfg, turboErr := turbo.LoadConfig(projectRoot)
						if turboErr == nil {
							if _, isTurboTask := turboCfg.Tasks[scriptName]; isTurboTask {
								_, turboFlags := turbo.SplitFlags(parsed.ScriptArgs, cfg.TuiEnabled())
								ran, err := tui.DelegateLaunch(cfg, scriptName, projectRoot, turboFlags)
								if err != nil {
									cli.Fail(logger, err, false)
								}
								if ran {
									return
								}
							}
						}
					}
				} else {
					mgr, ok := manager.GetManager(managerName)
					if !ok {
						cli.Fail(logger, fmt.Errorf("unknown manager: %s", managerName), false)
					}
					ran, err := tui.Launch(cfg, scriptName, projectRoot, mgr)
					if err != nil {
						cli.Fail(logger, err, false)
					}
					if ran {
						return
					}
				}
				// TUI was not applicable — fall through to normal execution
			}
		}
	}

	// Route to command handlers
	switch parsed.Action {
	case cli.ActionWorkspaceScript:
		// @workspace/script syntax — resolve and execute in the workspace directory
		resolved, workDir, shell, err := scripting.ResolveWorkspaceScript(
			parsed.WorkspaceName, parsed.ScriptName, projectRoot, cfg,
		)
		if err != nil {
			cli.Fail(logger, err, false)
		}
		switch resolved.Source {
		case scripting.ScriptSourceFolder:
			target := workspace.Target{Kind: workspace.TargetMember, Name: parsed.WorkspaceName, Dir: workDir}
			processEnv, envErr := env.BuildTargetEnv(projectRoot, cfg, target)
			if envErr != nil {
				cli.Fail(logger, envErr, false)
			}
			if err := scripting.ExecScriptFile(resolved.Path, parsed.ScriptArgs, workDir, shell, managerName, processEnv); err != nil {
				cli.Fail(logger, err, false)
			}
		case scripting.ScriptSourcePackageJSON:
			if err := commands.Run(managerName, parsed.ScriptName, parsed.ScriptArgs, workDir); err != nil {
				cli.Fail(logger, err, false)
			}
		default:
			cli.Fail(logger, fmt.Errorf("script %q not found in workspace %q", parsed.ScriptName, parsed.WorkspaceName), false)
		}
		return
	case cli.ActionScripts:
		if err := scripting.List(cfg, projectRoot, styles, logger); err != nil {
			cli.Fail(logger, err, false)
		}
		return
	case cli.ActionRunMultiple:
		if err := commands.RunMultiple(managerName, parsed.ScriptNames, parsed.ScriptArgs, originalWorkDir); err != nil {
			cli.Fail(logger, err, false)
		}
		return
	case cli.ActionScriptOverride:
		target := workspace.Target{Kind: workspace.TargetScript, Name: parsed.ScriptName}
		processEnv, envErr := env.BuildTargetEnv(projectRoot, cfg, target)
		if envErr != nil {
			cli.Fail(logger, envErr, false)
		}
		if err := scripting.RunOverride(parsed.ScriptName, parsed.ScriptArgs, projectRoot, cfg, managerName, processEnv); err != nil {
			cli.Fail(logger, err, false)
		}
		return
	case cli.ActionScriptFolder:
		target := workspace.Target{Kind: workspace.TargetScript, Name: parsed.ScriptName}
		processEnv, envErr := env.BuildTargetEnv(projectRoot, cfg, target)
		if envErr != nil {
			cli.Fail(logger, envErr, false)
		}
		if err := scripting.ExecScriptFile(parsed.Command, parsed.ScriptArgs, originalWorkDir, cfg.Shell, managerName, processEnv); err != nil {
			cli.Fail(logger, err, false)
		}
		return
	case cli.ActionScriptPackageJSON:
		if err := commands.Run(managerName, parsed.ScriptName, parsed.ScriptArgs, originalWorkDir); err != nil {
			cli.Fail(logger, err, false)
		}
		return
	case cli.ActionInstall:
		if err := commands.Install(managerName, projectRoot, cfg); err != nil {
			cli.Fail(logger, err, false)
		}
		return
	case cli.ActionAdd:
		if err := commands.Add(managerName, parsed.PackageNames, originalWorkDir, cfg); err != nil {
			cli.Fail(logger, err, false)
		}
		return
	case cli.ActionRemove:
		if err := commands.Remove(managerName, parsed.PackageNames, originalWorkDir, cfg); err != nil {
			cli.Fail(logger, err, false)
		}
		return
	case cli.ActionRun:
		if err := commands.Run(managerName, parsed.ScriptName, parsed.ScriptArgs, originalWorkDir); err != nil {
			cli.Fail(logger, err, false)
		}
		return
	case cli.ActionDev:
		if err := commands.Dev(managerName, parsed.ScriptArgs, originalWorkDir, cfg); err != nil {
			cli.Fail(logger, err, false)
		}
		return
	case cli.ActionPassthrough:
		if err := cli.RunPassthrough(managerName, parsed.Command, parsed.Args, originalWorkDir); err != nil {
			cli.Fail(logger, err, false)
		}
		return
	default:
		cli.Fail(logger, fmt.Errorf("unknown action"), true)
	}
}

// runEnvIfRequested checks for --env in effective args (config flags + CLI args).
// If present, runs env validation first; on success, strips --env from cfg and parsed.
func runEnvIfRequested(projectRoot string, cfg config.Config, parsed cli.ParsedCLI, logger *log.Logger) (config.Config, cli.ParsedCLI) {
	var effective []string
	switch parsed.Action {
	case cli.ActionAdd:
		effective = append(cfg.Flags["add"], parsed.PackageNames...)
	case cli.ActionRemove:
		effective = append(cfg.Flags["remove"], parsed.PackageNames...)
	case cli.ActionInstall:
		effective = cfg.Flags["install"]
	case cli.ActionDev:
		effective = append(cfg.Flags["dev"], parsed.ScriptArgs...)
	case cli.ActionRun, cli.ActionRunMultiple:
		effective = parsed.ScriptArgs
	case cli.ActionScriptOverride, cli.ActionScriptFolder, cli.ActionScriptPackageJSON:
		// Script overrides can have flags by script name (e.g. flags["dev"] for dev script)
		scriptFlags := cfg.Flags[parsed.ScriptName]
		effective = append(scriptFlags, parsed.ScriptArgs...)
	default:
		return cfg, parsed
	}

	if !env.HasEnvFlag(effective) {
		return cfg, parsed
	}

	if err := env.Run(projectRoot, cfg, logger); err != nil {
		os.Exit(1)
	}

	// Strip --env from cfg flags and parsed args
	cfg = env.StripEnvFromFlags(cfg)
	switch parsed.Action {
	case cli.ActionAdd:
		parsed.PackageNames = env.StripEnvFlag(parsed.PackageNames)
	case cli.ActionRemove:
		parsed.PackageNames = env.StripEnvFlag(parsed.PackageNames)
	case cli.ActionDev, cli.ActionRun, cli.ActionRunMultiple,
		cli.ActionScriptOverride, cli.ActionScriptFolder, cli.ActionScriptPackageJSON:
		parsed.ScriptArgs = env.StripEnvFlag(parsed.ScriptArgs)
	}
	return cfg, parsed
}
