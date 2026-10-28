package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/x/term"

	"github.com/ekkolyth/miso/internal/cli"
	"github.com/ekkolyth/miso/internal/cli/commands"
	"github.com/ekkolyth/miso/internal/cli/completion"
	"github.com/ekkolyth/miso/internal/cli/env"
	"github.com/ekkolyth/miso/internal/cli/reference"
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
	var leading env.LeadingFlags

	// if misox prepend "misox"
	invokedAsMisox := cli.InvokedAsMisox(os.Args[0])
	if invokedAsMisox {
		args = append([]string{"misox"}, args...)
	} else {
		leading, args = env.ParseLeadingFlags(args, os.LookupEnv)
	}
	ci := leading.CI
	envValidated := leading.Validate

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
		if len(args) >= 2 && args[0] == "help" && !strings.HasPrefix(args[1], "-") {
			if err := reference.RunCommandHelp(args[1]); err != nil {
				os.Exit(1)
			}
			return
		}
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
						if !term.IsTerminal(os.Stdin.Fd()) {
							cli.Fail(logger, fmt.Errorf("no interactive terminal; re-run with --yes to install the miso skill into all detected harnesses"), false)
							return
						}
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
	if cfg.StandaloneMode() {
		cmd := args[0]

		// Meta-commands that remain in standalone mode
		// (init, version, upgrade, completion already handled above)
		switch cmd {
		case "env":
			if err := env.Command(projectRoot, cfg, logger, args[1:], ci); err != nil {
				cli.Fail(logger, err, false)
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

		// a task entry may have no script of its own and still orchestrate
		// companions or dependencies
		_, isTask := cfg.Tasks[cmd]
		if resolved.Source == scripting.ScriptSourceNone && !isTask {
			cli.Fail(logger, fmt.Errorf("script '%s' not found in %s", cmd, cfg.Scripts), false)
		}

		scriptArgs := args[1:]

		if envValidated {
			if err := env.Run(projectRoot, cfg, logger, ci); err != nil {
				cli.Fail(logger, err, false)
			}
		}

		// Orchestrate the folder script (and any root-scope concurrent) — chrome on
		// a TTY with tui != off, plain otherwise. Falls through to the literal exec
		// below only when nothing was orchestrated.
		switch tui.SelectRenderer(cfg, tui.InteractiveTTY()) {
		case tui.RendererChrome:
			ran, err := tui.Launch(cfg, cmd, projectRoot, nil, nil, scriptArgs, envValidated, ci)
			if err != nil {
				cli.Fail(logger, err, false)
			}
			if ran {
				return
			}
		case tui.RendererPlain:
			ran, err := tui.LaunchPlain(cfg, cmd, projectRoot, nil, nil, scriptArgs, envValidated, ci)
			if err != nil {
				cli.Fail(logger, err, false)
			}
			if ran {
				return
			}
		}

		if resolved.Source == scripting.ScriptSourceNone {
			cli.Fail(logger, fmt.Errorf("script '%s' not found in %s", cmd, cfg.Scripts), false)
		}

		target, _ := workspace.ResolveTarget(cmd, nil, projectRoot, cfg)
		processEnv, envErr := env.BuildTargetEnv(projectRoot, cfg, target, ci)
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

	// inside a member, its own scripts come first, then the root's, then the
	// package manager. A run with an explicit @scope is left as parsed
	var cwdMember *workspace.Member
	if members, _ := workspace.DiscoverMembersCached(projectRoot, cfg); len(members) > 0 && originalWorkDir != projectRoot {
		if member, inWs := workspace.FromCWD(originalWorkDir, members); inWs {
			cwdMember = &member
		}
	}
	if cwdMember != nil && len(parsed.Scopes) == 0 {
		scriptName, scriptArgs, rewritable := parsed.ScriptName, parsed.ScriptArgs, true
		// dev and run hand a member package.json script to the package manager,
		// with the member's flags, whatever the root defines under that name
		verb := args[0]
		folderOnly := verb == "dev" || verb == "run"
		switch parsed.Action {
		case cli.ActionScriptPackageJSON, cli.ActionScriptFolder, cli.ActionRun:
		case cli.ActionDev:
			scriptName = "dev"
		case cli.ActionPassthrough:
			scriptName = parsed.Command
			scriptArgs = cli.ParseInlineArgs(parsed.Args)
		default:
			rewritable = false
		}
		if rewritable {
			member := *cwdMember
			resolved, _, _, resolveErr := scripting.ResolveWorkspaceScript(
				member.Name, scriptName, projectRoot, cfg,
			)
			// an ambiguous member script must not fall back to the root's
			if resolveErr != nil {
				cli.Fail(logger, resolveErr, false)
			}
			switch {
			case resolved.Source == scripting.ScriptSourceFolder ||
				(resolved.Source == scripting.ScriptSourcePackageJSON && !folderOnly):
				parsed.Action = cli.ActionWorkspaceScript
				parsed.WorkspaceName = member.Name
				parsed.ScriptName = scriptName
				parsed.ScriptArgs = scriptArgs
			case resolved.Source == scripting.ScriptSourcePackageJSON && verb == "dev":
				parsed.Action = cli.ActionDev
				parsed.ScriptArgs = scriptArgs
			case resolved.Source == scripting.ScriptSourcePackageJSON:
				parsed.Action = cli.ActionRun
				parsed.ScriptName = scriptName
				parsed.ScriptArgs = scriptArgs
			}
		}
	}

	// env does not need package manager
	if parsed.Action == cli.ActionEnv {
		if err := env.Command(projectRoot, cfg, logger, args[1:], ci); err != nil {
			cli.Fail(logger, err, false)
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

	if envValidated {
		if err := env.Run(projectRoot, cfg, logger, ci); err != nil {
			cli.Fail(logger, err, false)
		}
	}

	// Resolve explicit @scope filters up front so an unknown or ambiguous scope
	// fails with a precise message regardless of TUI/root/action routing, and so
	// the intercept switch below doesn't re-discover members.
	var scopeFilters []string
	if len(parsed.Scopes) > 0 {
		scopeFilters = scopeFilterNames(parsed.Scopes, projectRoot, cfg, logger)
	}

	// Orchestration / delegation interception — gated on running from the project
	// root (not a workspace subdirectory), not on the TUI being enabled. Native
	// miso mode always orchestrates; the renderer is chosen per run below.
	isRoot := true
	if members, _ := workspace.DiscoverMembersCached(projectRoot, cfg); len(members) > 0 {
		if _, inWs := workspace.FromCWD(originalWorkDir, members); inWs {
			isRoot = false
		}
	}

	// a member's own script run from its directory launches like <script> @member
	// from the root, so the member's companions and tasks apply
	memberLaunch := parsed.Action == cli.ActionWorkspaceScript && cwdMember != nil

	if isRoot || memberLaunch {
		switch parsed.Action {
		case cli.ActionDev, cli.ActionRun, cli.ActionScriptPackageJSON, cli.ActionPassthrough, cli.ActionScriptFolder, cli.ActionWorkspaceScript:
			scriptName := parsed.ScriptName
			scriptArgs := parsed.ScriptArgs
			delegateArgs := scriptArgs
			switch parsed.Action {
			case cli.ActionDev:
				scriptName = "dev"
			case cli.ActionPassthrough:
				scriptName = parsed.Command
				// a member script takes args the way a root script does; turbo and
				// RunPassthrough keep the raw args, flags and -- included
				scriptArgs = cli.ParseInlineArgs(parsed.Args)
				delegateArgs = parsed.Args
			}

			filters := scopeFilters
			if memberLaunch {
				filters = []string{cwdMember.Name}
			}

			renderer := tui.SelectRenderer(cfg, tui.InteractiveTTY())

			if cfg.IsDelegated() {
				_, taskOverridden := cfg.Tasks[scriptName]
				if !taskOverridden {
					// no task entry — falls to the delegated pipeline's own task names
					turboCfg, turboErr := turbo.LoadConfig(projectRoot)
					if turboErr == nil {
						if _, isTurboTask := turboCfg.Tasks[scriptName]; isTurboTask {
							if delegatedFolderClash(scriptName, projectRoot, cfg) {
								cli.Fail(logger, fmt.Errorf(
									"%q is both a turbo task and a folder script — declare repo.tasks.%s to have miso own it, or rename the script so turbo owns it",
									scriptName, scriptName), false)
							}
							turboFlags := turbo.SplitFlags(delegateArgs, renderer == tui.RendererChrome)
							ran, err := tui.DelegateLaunch(cfg, scriptName, projectRoot, turboFlags, filters)
							if err != nil {
								cli.Fail(logger, err, false)
							}
							if ran {
								return
							}
						}
					}
				}
				// task override, or a miso-only name in a delegated repo —
				// miso orchestrates with chrome, same as native mode below
			}
			mgr, ok := manager.GetManager(managerName)
			if !ok {
				cli.Fail(logger, fmt.Errorf("unknown manager: %s", managerName), false)
			}
			switch renderer {
			case tui.RendererChrome:
				ran, err := tui.Launch(cfg, scriptName, projectRoot, mgr, filters, scriptArgs, envValidated, ci)
				if err != nil {
					cli.Fail(logger, err, false)
				}
				if ran {
					return
				}
			case tui.RendererPlain:
				ran, err := tui.LaunchPlain(cfg, scriptName, projectRoot, mgr, filters, scriptArgs, envValidated, ci)
				if err != nil {
					cli.Fail(logger, err, false)
				}
				if ran {
					return
				}
			}
			// Orchestration/delegation not applicable — fall through. The Task 4
			// fail-safe rejects any explicit scope that reached here.
		}
	}

	// An explicit @scope must have been honored by a scope-aware run above.
	// Reaching here with scopes set means none fired (TUI off, the target
	// isn't a turbo/nx task, or the script exists in no target member) —
	// refuse to run unscoped.
	if len(parsed.Scopes) > 0 {
		target := parsed.ScriptName
		switch parsed.Action {
		case cli.ActionDev:
			target = "dev"
		case cli.ActionPassthrough:
			target = parsed.Command
		case cli.ActionRunMultiple:
			target = strings.Join(parsed.ScriptNames, " ")
		}
		cli.Fail(logger, fmt.Errorf(
			"cannot scope %q to %s — scoped runs need an interactive terminal and a workspace-aware target (a turbo/nx task or a script defined in the target workspace)",
			target, strings.Join(parsed.Scopes, " "),
		), false)
	}

	// the member's flags apply to the package manager commands run inside it
	pmCfg := cfg
	if cwdMember != nil {
		pmCfg = workspace.EffectiveConfig(cfg, *cwdMember)
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
			processEnv, envErr := env.BuildTargetEnv(projectRoot, cfg, target, ci)
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
		processEnv, envErr := env.BuildTargetEnv(projectRoot, cfg, target, ci)
		if envErr != nil {
			cli.Fail(logger, envErr, false)
		}
		if err := scripting.RunOverride(parsed.ScriptName, parsed.ScriptArgs, projectRoot, cfg, managerName, processEnv); err != nil {
			cli.Fail(logger, err, false)
		}
		return
	case cli.ActionScriptFolder:
		target := workspace.Target{Kind: workspace.TargetScript, Name: parsed.ScriptName}
		processEnv, envErr := env.BuildTargetEnv(projectRoot, cfg, target, ci)
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
		if err := commands.Add(managerName, parsed.PackageNames, originalWorkDir, pmCfg); err != nil {
			cli.Fail(logger, err, false)
		}
		return
	case cli.ActionRemove:
		if err := commands.Remove(managerName, parsed.PackageNames, originalWorkDir, pmCfg); err != nil {
			cli.Fail(logger, err, false)
		}
		return
	case cli.ActionRun:
		if err := commands.Run(managerName, parsed.ScriptName, parsed.ScriptArgs, originalWorkDir); err != nil {
			cli.Fail(logger, err, false)
		}
		return
	case cli.ActionDev:
		if err := commands.Dev(managerName, parsed.ScriptArgs, originalWorkDir, pmCfg); err != nil {
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

// a folder script sharing a delegated pipeline task's name would never run —
// the delegate owns the name — so the two can't coexist. package.json scripts
// are entry points, not claimants: a root "build": "turbo run build" is the
// turbo convention, so they never count as a clash.
func delegatedFolderClash(scriptName, projectRoot string, cfg config.Config) bool {
	resolved, err := scripting.ResolveScript(scriptName, projectRoot, cfg)
	return err == nil && resolved.Source == scripting.ScriptSourceFolder
}

// scopeFilterNames resolves parsed.Scopes to full member package names. An
// unknown or ambiguous scope token the user typed is fatal — miso refuses to
// run when it can't honor an explicit scope.
func scopeFilterNames(scopes []string, projectRoot string, cfg config.Config, logger *log.Logger) []string {
	if len(scopes) == 0 {
		return nil
	}
	members, _ := workspace.DiscoverMembersCached(projectRoot, cfg)
	resolved, err := workspace.ResolveScopes(scopes, members, projectRoot)
	if err != nil {
		cli.Fail(logger, err, false)
	}
	names := make([]string, 0, len(resolved))
	for _, member := range resolved {
		names = append(names, member.Name)
	}
	return names
}
