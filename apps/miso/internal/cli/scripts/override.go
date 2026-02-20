package scripts

import (
	"fmt"

	"github.com/ekkolyth/miso/internal/config"
)

// run custom script (legacy - should use resolution system instead)
func RunOverride(scriptName string, scriptArgs []string, root string, cfg config.Config) error {
	resolved, err := ResolveScript(scriptName, root, cfg)
	if err != nil {
		return err
	}
	if resolved.Source == ScriptSourceNone {
		return fmt.Errorf("script %q not found", scriptName)
	}
	if resolved.Source == ScriptSourceFolder {
		return ExecScriptFile(resolved.Path, scriptArgs, "", cfg.Shell)
	}
	if resolved.Source == ScriptSourcePackageJSON {
		// this shouldn't happen in RunOverride, but handle it anyway
		return fmt.Errorf("script %q found in package.json, use package manager directly", scriptName)
	}
	return fmt.Errorf("unknown script source")
}
