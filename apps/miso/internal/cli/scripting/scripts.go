package scripting

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/charmbracelet/log"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/turbo"
	"github.com/ekkolyth/miso/internal/ui"
)

// miso scripts
func List(cfg config.Config, root string, styles ui.Styles, logger *log.Logger) error {
	scriptsPath := cfg.Scripts
	if scriptsPath == "" {
		scriptsPath = "./scripts"
	}

	// resolve scripts path relative to root
	if !filepath.IsAbs(scriptsPath) {
		scriptsPath = filepath.Join(root, scriptsPath)
	}

	// discover scripts from folder
	folderScripts, err := DiscoverScripts(scriptsPath)
	if err != nil {
		return fmt.Errorf("discover scripts: %w", err)
	}

	// read package.json scripts
	pkgScripts, err := ReadPackageJSONScripts(root)
	if err != nil {
		return fmt.Errorf("read package.json scripts: %w", err)
	}

	// check if any scripts exist
	hasFolderScripts := len(folderScripts) > 0
	hasPkgScripts := len(pkgScripts) > 0

	hasTurboTasks := false
	if cfg.IsDelegated() && cfg.RepoMode() == "turbo" {
		turboCfg, turboErr := turbo.LoadConfig(root)
		if turboErr == nil && len(turboCfg.Tasks) > 0 {
			hasTurboTasks = true
		}
	}

	if !hasFolderScripts && !hasPkgScripts && !hasTurboTasks {
		logger.Info("no scripts found in scripts folder, package.json, or turbo.json")
		return nil
	}

	logger.Info(styles.Heading.Render("available scripts"))

	// show scripts from folder
	if hasFolderScripts {
		_, _ = fmt.Fprintf(os.Stdout, "\nscripts folder:\n")
		var folderNames []string
		for name := range folderScripts {
			folderNames = append(folderNames, name)
		}
		sort.Strings(folderNames)
		for _, name := range folderNames {
			scripts := folderScripts[name]
			_, _ = fmt.Fprintf(os.Stdout, "  %s (%s)\n", name, scripts[0].RelativePath)
		}
	}

	// show scripts from package.json
	if hasPkgScripts {
		if hasFolderScripts {
			_, _ = fmt.Fprintf(os.Stdout, "\npackage.json:\n")
		} else {
			_, _ = fmt.Fprintf(os.Stdout, "package.json:\n")
		}
		var pkgNames []string
		for name := range pkgScripts {
			pkgNames = append(pkgNames, name)
		}
		sort.Strings(pkgNames)
		for _, name := range pkgNames {
			_, _ = fmt.Fprintf(os.Stdout, "  %s: %s\n", name, pkgScripts[name])
		}
	}

	// show turbo.json tasks (only in turbo mode, deduplicated)
	if cfg.IsDelegated() && cfg.RepoMode() == "turbo" {
		turboCfg, turboErr := turbo.LoadConfig(root)
		if turboErr == nil && len(turboCfg.Tasks) > 0 {
			existingNames := make(map[string]bool)
			for name := range folderScripts {
				existingNames[name] = true
			}
			for name := range pkgScripts {
				existingNames[name] = true
			}

			var turboNames []string
			for _, name := range turboCfg.TaskNames() {
				if !existingNames[name] {
					turboNames = append(turboNames, name)
				}
			}

			if len(turboNames) > 0 {
				_, _ = fmt.Fprintf(os.Stdout, "\nturbo.json:\n")
				for _, name := range turboNames {
					_, _ = fmt.Fprintf(os.Stdout, "  %s\n", name)
				}
			}
		}
	}

	return nil
}

// sorted, deduped by resolution order (scripts folder over package.json)
func ListNames(root string, cfg config.Config) ([]string, error) {
	scriptsPath := cfg.Scripts
	if scriptsPath == "" {
		scriptsPath = "./scripts"
	}
	if !filepath.IsAbs(scriptsPath) {
		scriptsPath = filepath.Join(root, scriptsPath)
	}

	folderScripts, err := DiscoverScripts(scriptsPath)
	if err != nil {
		return nil, fmt.Errorf("discover scripts: %w", err)
	}

	pkgScripts, err := ReadPackageJSONScripts(root)
	if err != nil {
		return nil, fmt.Errorf("read package.json scripts: %w", err)
	}

	seen := make(map[string]bool)
	var names []string

	for name := range folderScripts {
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	for name := range pkgScripts {
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}

	if cfg.IsDelegated() && cfg.RepoMode() == "turbo" {
		turboCfg, turboErr := turbo.LoadConfig(root)
		if turboErr == nil {
			for _, name := range turboCfg.TaskNames() {
				if !seen[name] {
					seen[name] = true
					names = append(names, name)
				}
			}
		}
	}

	sort.Strings(names)
	return names, nil
}
