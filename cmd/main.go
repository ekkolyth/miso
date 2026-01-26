package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/ekkolyth/miso/internal/cli/core"
	"github.com/ekkolyth/miso/internal/cli/pm"
	"github.com/ekkolyth/miso/internal/cli/pm/managers"
	"github.com/ekkolyth/miso/internal/cli/scripts"
	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/ui"
)

func init() {
	// Register manager drivers
	core.RegisterManager("bun", managers.Bun{})
	core.RegisterManager("npm", managers.Npm{})
	core.RegisterManager("pnpm", managers.Pnpm{})
	core.RegisterManager("yarn", managers.Yarn{})
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
	if os.Getenv("MISO_DEBUG") == "1" {
		logger.SetLevel(log.DebugLevel)
	}

	root, err := os.Getwd()
	if err != nil {
		core.Fail(logger, fmt.Errorf("determine working directory: %w", err), false)
	}

	// Handle "init" command early, before loading config or ensuring manager
	if len(args) > 0 && args[0] == "init" {
		if err := core.RunInit(root, styles, logger); err != nil {
			core.Fail(logger, err, false)
		}
		return
	}

	cfg, err := core.LoadConfig(root)
	if err != nil {
		core.Fail(logger, err, false)
	}

	parsed, err := core.ParseCLI(args, cfg)
	if err != nil {
		core.Fail(logger, err, true)
	}

	// Handle "version" command - can work without a manager
	if parsed.Action == core.ActionVersion {
		if err := core.RunVersion(root, cfg); err != nil {
			core.Fail(logger, err, false)
		}
		return
	}

	// Handle "update" command - can work without a manager
	if parsed.Action == core.ActionUpdate {
		if err := pm.Update(parsed.Local, parsed.Args); err != nil {
			core.Fail(logger, err, false)
		}
		return
	}

	// Ensure manager is configured before proceeding
	managerName, cfg, err := core.EnsureManager(root, cfg, func(root string, managerList []string) (config.Config, error) {
		newCfg, err := core.RunOnboarding(root, managerList, styles, logger)
		if err != nil {
			return config.Config{}, err
		}
		logger.Info(styles.Heading.Render("detected manager"), "manager", newCfg.PackageManager)
		return newCfg, nil
	})
	if err != nil {
		core.Fail(logger, err, false)
	}

	if os.Getenv("MISO_DEBUG") == "1" {
		logger.Debug("resolved manager", "manager", managerName)
	}

	// Route to command handlers
	switch parsed.Action {
	case core.ActionScripts:
		if err := scripts.List(cfg, styles, logger); err != nil {
			core.Fail(logger, err, false)
		}
		return
	case core.ActionRunMultiple:
		if err := scripts.RunMultiple(managerName, parsed.ScriptNames, parsed.ScriptArgs); err != nil {
			core.Fail(logger, err, false)
		}
		return
	case core.ActionScriptOverride:
		if err := scripts.RunOverride(parsed.ScriptName, parsed.ScriptArgs, cfg); err != nil {
			core.Fail(logger, err, false)
		}
		return
	case core.ActionInstall:
		if err := pm.Install(managerName); err != nil {
			core.Fail(logger, err, false)
		}
		return
	case core.ActionAdd:
		if err := pm.Add(managerName, parsed.PackageNames); err != nil {
			core.Fail(logger, err, false)
		}
		return
	case core.ActionRemove:
		if err := pm.Remove(managerName, parsed.PackageNames); err != nil {
			core.Fail(logger, err, false)
		}
		return
	case core.ActionRun:
		if err := scripts.Run(managerName, parsed.ScriptName, parsed.ScriptArgs); err != nil {
			core.Fail(logger, err, false)
		}
		return
	case core.ActionDev:
		if err := scripts.Dev(managerName, parsed.ScriptArgs); err != nil {
			core.Fail(logger, err, false)
		}
		return
	case core.ActionMisox:
		if err := core.RunMisox(managerName, parsed.PackageName, parsed.Args); err != nil {
			core.Fail(logger, err, false)
		}
		return
	case core.ActionPassthrough:
		if err := core.RunPassthrough(managerName, parsed.Command, parsed.Args); err != nil {
			core.Fail(logger, err, false)
		}
		return
	default:
		core.Fail(logger, fmt.Errorf("unknown action"), true)
	}
}
