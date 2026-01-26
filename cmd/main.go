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

	originalWorkDir, err := os.Getwd()
	if err != nil {
		core.Fail(logger, fmt.Errorf("determine working directory: %w", err), false)
	}

	// Handle "init" command early, before loading config or ensuring manager
	if len(args) > 0 && args[0] == "init" {
		if err := core.RunInit(originalWorkDir, styles, logger); err != nil {
			core.Fail(logger, err, false)
		}
		return
	}

	// find project root
	projectRoot, err := core.FindProjectRoot(originalWorkDir)
	if err != nil {
		core.Fail(logger, err, false)
	}

	cfg, err := core.LoadConfig(projectRoot)
	if err != nil {
		core.Fail(logger, err, false)
	}

	parsed, err := core.ParseCLI(args, cfg, projectRoot)
	if err != nil {
		core.Fail(logger, err, true)
	}

	// Handle "version" command - can work without a manager
	if parsed.Action == core.ActionVersion {
		if err := core.RunVersion(projectRoot, cfg); err != nil {
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
	managerName, cfg, err := core.EnsureManager(projectRoot, cfg)
	if err != nil {
		core.Fail(logger, err, false)
	}

	if os.Getenv("MISO_DEBUG") == "1" {
		logger.Debug("resolved manager", "manager", managerName)
	}

	// Route to command handlers
	switch parsed.Action {
	case core.ActionScripts:
		if err := scripts.List(cfg, projectRoot, styles, logger); err != nil {
			core.Fail(logger, err, false)
		}
		return
	case core.ActionRunMultiple:
		if err := pm.RunMultiple(managerName, parsed.ScriptNames, parsed.ScriptArgs, originalWorkDir); err != nil {
			core.Fail(logger, err, false)
		}
		return
	case core.ActionScriptOverride:
		if err := scripts.RunOverride(parsed.ScriptName, parsed.ScriptArgs, projectRoot, cfg); err != nil {
			core.Fail(logger, err, false)
		}
		return
	case core.ActionScriptFolder:
		if err := scripts.ExecScriptFile(parsed.Command, parsed.ScriptArgs, originalWorkDir); err != nil {
			core.Fail(logger, err, false)
		}
		return
	case core.ActionScriptPackageJSON:
		if err := pm.Run(managerName, parsed.ScriptName, parsed.ScriptArgs, originalWorkDir); err != nil {
			core.Fail(logger, err, false)
		}
		return
	case core.ActionInstall:
		if err := pm.Install(managerName, originalWorkDir, cfg); err != nil {
			core.Fail(logger, err, false)
		}
		return
	case core.ActionAdd:
		if err := pm.Add(managerName, parsed.PackageNames, originalWorkDir, cfg); err != nil {
			core.Fail(logger, err, false)
		}
		return
	case core.ActionRemove:
		if err := pm.Remove(managerName, parsed.PackageNames, originalWorkDir, cfg); err != nil {
			core.Fail(logger, err, false)
		}
		return
	case core.ActionRun:
		if err := pm.Run(managerName, parsed.ScriptName, parsed.ScriptArgs, originalWorkDir); err != nil {
			core.Fail(logger, err, false)
		}
		return
	case core.ActionDev:
		if err := pm.Dev(managerName, parsed.ScriptArgs, originalWorkDir, cfg); err != nil {
			core.Fail(logger, err, false)
		}
		return
	case core.ActionMisox:
		if err := core.RunMisox(managerName, parsed.PackageName, parsed.Args, originalWorkDir); err != nil {
			core.Fail(logger, err, false)
		}
		return
	case core.ActionPassthrough:
		if err := core.RunPassthrough(managerName, parsed.Command, parsed.Args, originalWorkDir); err != nil {
			core.Fail(logger, err, false)
		}
		return
	default:
		core.Fail(logger, fmt.Errorf("unknown action"), true)
	}
}
