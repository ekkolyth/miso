package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/ekkolyth/miso/internal/cli"
	"github.com/ekkolyth/miso/internal/cli/commands"
	"github.com/ekkolyth/miso/internal/cli/completion"
	"github.com/ekkolyth/miso/internal/cli/env"
	"github.com/ekkolyth/miso/internal/cli/scripting"
	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/manager"
	"github.com/ekkolyth/miso/internal/manager/bun"
	"github.com/ekkolyth/miso/internal/manager/npm"
	"github.com/ekkolyth/miso/internal/manager/pnpm"
	"github.com/ekkolyth/miso/internal/manager/yarn"
	"github.com/ekkolyth/miso/internal/ui"
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
	if baseName := filepath.Base(os.Args[0]); baseName == "misox" || strings.HasPrefix(baseName, "misox-") {
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

	// handle global commands before loading config
	if len(args) > 0 {
		switch args[0] {
		case "__complete":
			completion.Complete(args, originalWorkDir)
			return
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
		case "version", "v":
			// load config if available
			var projectRoot string
			var cfg config.Config
			if root, err := cli.FindProjectRoot(originalWorkDir); err == nil {
				projectRoot = root
				if loadedCfg, err := cli.LoadConfig(projectRoot); err == nil {
					cfg = loadedCfg
				}
			}
			if err := commands.RunVersion(projectRoot, cfg); err != nil {
				cli.Fail(logger, err, false)
			}
			return
		case "upgrade":
			local, remainingArgs := cli.ParseLocalFlag(args[1:])
			if err := commands.Upgrade(local, remainingArgs); err != nil {
				cli.Fail(logger, err, false)
			}
			return
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

	parsed, err := cli.ParseCLI(args, cfg, projectRoot)
	if err != nil {
		cli.Fail(logger, err, true)
	}

	// env does not need package manager
	if parsed.Action == cli.ActionEnv {
		if err := env.Run(projectRoot, cfg, logger); err != nil {
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

	// --env flag: run env validation first, then strip from args before passing to command
	cfg, parsed = runEnvIfRequested(projectRoot, cfg, parsed, logger)

	// Route to command handlers
	switch parsed.Action {
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
		if err := scripting.RunOverride(parsed.ScriptName, parsed.ScriptArgs, projectRoot, cfg); err != nil {
			cli.Fail(logger, err, false)
		}
		return
	case cli.ActionScriptFolder:
		if err := scripting.ExecScriptFile(parsed.Command, parsed.ScriptArgs, originalWorkDir, cfg.Shell); err != nil {
			cli.Fail(logger, err, false)
		}
		return
	case cli.ActionScriptPackageJSON:
		if err := commands.Run(managerName, parsed.ScriptName, parsed.ScriptArgs, originalWorkDir); err != nil {
			cli.Fail(logger, err, false)
		}
		return
	case cli.ActionInstall:
		if err := commands.Install(managerName, originalWorkDir, cfg); err != nil {
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
	case cli.ActionMisox:
		if err := cli.RunMisox(managerName, parsed.PackageName, parsed.Args, originalWorkDir); err != nil {
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
		cli.Fail(logger, err, false)
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
