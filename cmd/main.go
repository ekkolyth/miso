package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

	// if misox prepend "misox"
	if baseName := filepath.Base(os.Args[0]); baseName == "misox" {
		args = append([]string{"misox"}, args...)
	}

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

	// Handle "init" command early, before loading config or ensuring manager
	if len(args) > 0 && args[0] == "init" {
		// Check if miso.json already exists - if so, exit early
		_, err := config.Load(root)
		if err == nil {
			fail(logger, fmt.Errorf("miso.json already exists"), false)
		}
		if !errors.Is(err, config.ErrNotFound) {
			fail(logger, err, false)
		}

		managerList := registeredManagerNames()
		newCfg, err := onboarding.Init(root, managerList, styles, logger)
		if err != nil {
			fail(logger, err, false)
		}
		if err := config.Save(root, newCfg); err != nil {
			fail(logger, err, false)
		}

		// Check if package.json already exists - if so, skip running init
		packageJsonPath := filepath.Join(root, "package.json")
		if _, err := os.Stat(packageJsonPath); err == nil {
			logger.Info("package.json already exists, skipping manager init")
			return
		}

		// Run the package manager's init command
		if _, ok := driverRegistry[newCfg.PackageManager]; !ok {
			fail(logger, fmt.Errorf("unsupported manager: %s", newCfg.PackageManager), false)
		}
		spec := cli.ExecSpec{
			Command: newCfg.PackageManager,
			Args:    []string{"init"},
		}
		if err := cli.Exec(spec, newCfg.PackageManager); err != nil {
			fail(logger, err, false)
		}
		return
	}

	cfg, err := loadConfig(root)
	if err != nil {
		fail(logger, err, false)
	}

	parsed, err := cli.ParseCLI(args, cfg)
	if err != nil {
		fail(logger, err, true)
	}

	// Handle "version" command - can work without a manager
	if parsed.Action == cli.ActionVersion {
		misoVersion, err := getMisoVersion()
		if err != nil {
			// If we can't get version, show unknown but don't fail
			misoVersion = "unknown"
		}
		fmt.Fprintf(os.Stdout, "miso %s\n", misoVersion)

		// Try to get manager version if available, but don't trigger onboarding
		var managerName string
		if cfg.PackageManager != "" {
			managerName = cfg.PackageManager
		} else {
			// Try to detect manager from lockfile, but don't trigger onboarding
			detected, err := cli.DetectManager(root)
			if err == nil {
				managerName = detected
			}
		}

		// If we have a manager, try to run its version command
		if managerName != "" {
			driver, ok := driverRegistry[managerName]
			if ok {
				spec := driver.BuildVersion()
				// Run version command silently (no logging)
				cmd := exec.Command(spec.Command, spec.Args...)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				_ = cmd.Run() // Silently ignore errors
			}
		}
		return
	}

	managerName, cfg, err := ensureManager(root, cfg, styles, logger)
	if err != nil {
		fail(logger, err, false)
	}

	if os.Getenv("MISO_DEBUG") == "1" {
		logger.Debug("resolved manager", "manager", managerName)
	}

	switch parsed.Action {
	case cli.ActionScripts:
		if len(cfg.Scripts) == 0 {
			logger.Info("no scripts defined in miso.json")
			return
		}
		scriptNames := make([]string, 0, len(cfg.Scripts))
		for name := range cfg.Scripts {
			scriptNames = append(scriptNames, name)
		}
		sort.Strings(scriptNames)
		logger.Info(styles.Heading.Render("available scripts"))
		for _, name := range scriptNames {
			fmt.Fprintf(os.Stdout, "  %s: %s\n", name, cfg.Scripts[name])
		}
		return
	case cli.ActionRunMultiple:
		// For now, just run them sequentially
		// TODO: Add concurrent execution support
		driver, ok := driverRegistry[managerName]
		if !ok {
			fail(logger, fmt.Errorf("unsupported manager: %s", managerName), false)
		}
		for _, scriptName := range parsed.ScriptNames {
			spec := driver.BuildRun(scriptName, parsed.ScriptArgs)
			if err := cli.Exec(spec, managerName); err != nil {
				fail(logger, err, false)
			}
		}
		return
	}

	if parsed.Action == cli.ActionScriptOverride {
		command := cfg.Scripts[parsed.ScriptName]
		if command == "" {
			fail(logger, fmt.Errorf("script %q not defined in config", parsed.ScriptName), false)
		}
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
	case cli.ActionMisox:
		spec = driver.BuildMisox(parsed.PackageName, parsed.Args)
	case cli.ActionPassthrough:
		spec = cli.ExecSpec{
			Command: managerName,
			Args:    append([]string{parsed.Command}, parsed.Args...),
		}
	default:
		fail(logger, fmt.Errorf("unknown action"), true)
	}

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
	fmt.Fprintf(os.Stderr, "ERROR miso: %s\n", err.Error())
	if showUsage {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, usageText())
	}
	os.Exit(1)
}

func getMisoVersion() (string, error) {
	type packageInfo struct {
		Version string `json:"version"`
	}

	// Try multiple locations for package.json
	var packageJSONPaths []string

	// 1. Current directory (for npm installs)
	cwd, _ := os.Getwd()
	packageJSONPaths = append(packageJSONPaths, filepath.Join(cwd, "package.json"))

	// 2. node_modules/@ekkolyth/miso/package.json (for local npm installs)
	packageJSONPaths = append(packageJSONPaths, filepath.Join(cwd, "node_modules", "@ekkolyth", "miso", "package.json"))

	// 3. Relative to binary location (for Go installs)
	// Try to find the project root by looking for package.json
	// Start from the binary location and walk up
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		// Walk up to find package.json (max 10 levels)
		for i := 0; i < 10; i++ {
			pkgPath := filepath.Join(exeDir, "package.json")
			packageJSONPaths = append(packageJSONPaths, pkgPath)
			parent := filepath.Dir(exeDir)
			if parent == exeDir {
				break
			}
			exeDir = parent
		}
	}

	// 4. Try relative to source file location (for development)
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		dir := filepath.Dir(filename)
		// cmd/ -> project root
		projectRoot := filepath.Join(dir, "..")
		packageJSONPaths = append(packageJSONPaths, filepath.Join(projectRoot, "package.json"))
	}

	// Try each path
	for _, path := range packageJSONPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var info packageInfo
		if err := json.Unmarshal(data, &info); err != nil {
			continue
		}

		if info.Version != "" {
			return info.Version, nil
		}
	}

	return "", fmt.Errorf("could not find package.json with version")
}

func usageText() string {
	return strings.TrimSpace(`
Miso – the agnostic package manager

Usage:
  miso init
  miso version
  miso install
  miso add <pkg>
  miso remove <pkg>
  miso run <script> <args>
  miso dev <args>
  miso misox <package> [args...]
  miso <command> [args...]

  - Automatically detects bun, npm, pnpm, or yarn.
  - Generates a miso.json so you can override the manager or define scripts.
  - Unknown commands pass through to the detected package manager.
`)
}
