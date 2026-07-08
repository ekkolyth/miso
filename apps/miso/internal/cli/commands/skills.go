package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ekkolyth/miso/internal/manager"
)

const (
	skillsSource = "https://github.com/ekkolyth/miso/tree/main/packages/skills"
	skillName    = "miso"
)

var errNpmOverrides = errors.New(`npm's package installer (arborist) crashes on projects with an "overrides" field (npm bug). install bun or pnpm, or remove "overrides" to use miso skills`)

// buildSkillsArgs assembles the skills-tool argv for an add/remove targeting the
// given harness agent ids.
func buildSkillsArgs(op, name string, harnesses []string) []string {
	var args []string
	switch op {
	case "add":
		args = []string{"add", skillsSource}
	case "remove":
		args = []string{"remove", name}
	}
	for _, harness := range harnesses {
		args = append(args, "-a", harness)
	}
	return append(args, "-y")
}

// hasNpmOverridesConflict reports whether managerName is npm and workDir's
// package.json declares an "overrides" field — the combination that crashes npm's
// arborist.
func hasNpmOverridesConflict(managerName, workDir string) (bool, error) {
	if managerName != "npm" {
		return false, nil
	}
	data, err := os.ReadFile(filepath.Join(workDir, "package.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	var pkg struct {
		Overrides json.RawMessage `json:"overrides"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false, nil // malformed json — let the runner surface its own error
	}
	return len(pkg.Overrides) > 0, nil
}

func guardNpmOverrides(managerName, workDir string) error {
	conflict, err := hasNpmOverridesConflict(managerName, workDir)
	if err != nil {
		return err
	}
	if conflict {
		return errNpmOverrides
	}
	return nil
}

// ParseSkillsFlags scans args for --add, --rm, and --yes/-y. Does not modify args.
func ParseSkillsFlags(args []string) (add, rm, yes bool) {
	for _, arg := range args {
		switch arg {
		case "--add":
			add = true
		case "--rm":
			rm = true
		case "--yes", "-y":
			yes = true
		}
	}
	return
}

// RunSkillsAdd installs the miso skill for the given harnesses via the detected
// manager's dlx runner.
func RunSkillsAdd(managerName, workDir string, harnesses []string) error {
	return runSkills("add", managerName, workDir, harnesses)
}

// RunSkillsRemove uninstalls the miso skill from the given harnesses.
func RunSkillsRemove(managerName, workDir string, harnesses []string) error {
	return runSkills("remove", managerName, workDir, harnesses)
}

func runSkills(op, managerName, workDir string, harnesses []string) error {
	if err := guardNpmOverrides(managerName, workDir); err != nil {
		return err
	}
	driver, ok := manager.GetManager(managerName)
	if !ok {
		return fmt.Errorf("unsupported manager: %s", managerName)
	}
	spec := driver.BuildDlx("skills", buildSkillsArgs(op, skillName, harnesses))
	return manager.Exec(spec, workDir)
}
