package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/ekkolyth/miso/internal/cli"
	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/managers"
	"github.com/ekkolyth/miso/internal/ui/onboarding"
	"github.com/ekkolyth/miso/internal/ui/theme"
)

var driverRegistry = map[string]cli.Manager{
	"bun":  managers.Bun{},
	"npm":  managers.Npm{},
	"pnpm": managers.Pnpm{},
	"yarn": managers.Yarn{},
}

func main() {
	args := os.Args[1:]

	styles := theme.Default()
	logger := log.NewWithOptions(os.Stderr, log.Options{
		Prefix:          "miso",
		ReportTimestamp: false,
	})
	if os.Getenv("MISO_DEBUG") == "1" {
		logger.SetLevel(log.DebugLevel)
	}

	root, err := os.Getwd()
	if err != nil {
		fail(logger, fmt.Errorf("determine working directory: %w", err), false)
	}

	cfg, err := loadConfig(root)
	if err != nil {
		fail(logger, err, false)
	}

	managerName, cfg, err := ensureManager(root, cfg, styles, logger)
	if err != nil {
		fail(logger, err, false)
	}

	parsed, err := cli.ParseCLI(args, cfg)
	if err != nil {
		fail(logger, err, true)
	}

	if os.Getenv("MISO_DEBUG") == "1" {
		logger.Debug("resolved manager", "manager", managerName)
	}

	if parsed.Action == cli.ActionScriptOverride {
		command := cfg.Scripts[parsed.ScriptName]
		if command == "" {
			fail(logger, fmt.Errorf("script %q not defined in config", parsed.ScriptName), false)
		}
		scriptCmd := commandWithArgs(command, parsed.ScriptArgs)
		logger.Info(styles.Heading.Render("running script"),
			"script", parsed.ScriptName,
			"command", scriptCmd)
		if err := cli.ExecScript(command, parsed.ScriptArgs); err != nil {
			fail(logger, err, false)
		}
		return
	}

	driver, ok := driverRegistry[managerName]
	if !ok {
		fail(logger, fmt.Errorf("unsupported manager: %s", managerName), false)
	}

	var spec cli.ExecSpec
	switch parsed.Action {
	case cli.ActionInstall:
		spec = driver.BuildInstall()
	case cli.ActionAdd:
		spec = driver.BuildAdd(parsed.PackageNames)
	case cli.ActionRemove:
		spec = driver.BuildRemove(parsed.PackageNames)
	case cli.ActionRun:
		spec = driver.BuildRun(parsed.ScriptName, parsed.ScriptArgs)
	case cli.ActionDev:
		spec = driver.BuildRun("dev", parsed.ScriptArgs)
	case cli.ActionPassthrough:
		spec = cli.ExecSpec{
			Command: managerName,
			Args:    append([]string{parsed.Command}, parsed.Args...),
		}
	default:
		fail(logger, fmt.Errorf("unknown action"), true)
	}

	logger.Info(styles.Heading.Render("running manager command"),
		"manager", managerName,
		"command", commandWithArgs(spec.Command, spec.Args))

	if err := cli.Exec(spec, managerName); err != nil {
		fail(logger, err, false)
	}
}

func loadConfig(root string) (config.Config, error) {
	cfg, err := config.Load(root)
	if errors.Is(err, config.ErrNotFound) {
		return config.Config{
			ProjectName: filepath.Base(root),
			Scripts:     map[string]string{},
		}, nil
	}
	if err != nil {
		return config.Config{}, err
	}
	cfg.EnsureDefaults()
	if cfg.ProjectName == "" {
		cfg.ProjectName = filepath.Base(root)
	}
	return cfg, nil
}

func ensureManager(root string, cfg config.Config, styles theme.Styles, logger *log.Logger) (string, config.Config, error) {
	if cfg.PackageManager != "" {
		return cfg.PackageManager, cfg, nil
	}

	detected, err := cli.DetectManager(root)
	if err == nil {
		cfg.PackageManager = detected
		logger.Info(styles.Heading.Render("detected manager"), "manager", detected)
		if err := config.Save(root, cfg); err != nil {
			return "", cfg, err
		}
		return detected, cfg, nil
	}

	if errors.Is(err, cli.ErrNoLockfile) {
		managerList := registeredManagerNames()
		newCfg, onboardingErr := onboarding.Run(root, managerList, styles, logger)
		if onboardingErr != nil {
			return "", cfg, onboardingErr
		}
		if err := config.Save(root, newCfg); err != nil {
			return "", cfg, err
		}
		return newCfg.PackageManager, newCfg, nil
	}

	return "", cfg, err
}

func registeredManagerNames() []string {
	names := make([]string, 0, len(driverRegistry))
	for name := range driverRegistry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func commandWithArgs(command string, args []string) string {
	if len(args) == 0 {
		return command
	}
	return fmt.Sprintf("%s %s", command, strings.Join(args, " "))
}

func fail(logger *log.Logger, err error, showUsage bool) {
	logger.Error(err.Error())
	if showUsage {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, usageText())
	}
	os.Exit(1)
}

func usageText() string {
	return strings.TrimSpace(`
Miso – the agnostic package manager

Usage:
  miso install
  miso add <pkg> 
  miso remove <pkg> 
  miso run <script> <args>
  miso dev <args>
  miso <command> [args...]

  - Automatically detects bun, npm, pnpm, or yarn.
  - Generates a miso.json so you can override the manager or define scripts.
  - Unknown commands pass through to the detected package manager.
`)
}
