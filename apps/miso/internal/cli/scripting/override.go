package scripting

import (
	"fmt"

	"github.com/ekkolyth/miso/internal/config"
)

// run custom script (legacy - should use resolution system instead)
func RunOverride(scriptName string, scriptArgs []string, root string, cfg config.Config, managerName string, environ []string) error {
	resolved, err := ResolveScript(scriptName, root, cfg)
	if err != nil {
		return err
	}
	if resolved.Source == ScriptSourceNone {
		return fmt.Errorf("script %q not found", scriptName)
	}
	if resolved.Source == ScriptSourceFolder {
		return ExecScriptFile(resolved.Path, scriptArgs, root, cfg.Shell, managerName, environ)
	}
	if resolved.Source == ScriptSourcePackageJSON {
		return fmt.Errorf("script %q found in package.json, use package manager directly", scriptName)
	}
	return fmt.Errorf("unknown script source")
}
