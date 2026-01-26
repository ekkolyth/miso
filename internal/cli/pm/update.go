package pm

import "github.com/ekkolyth/miso/internal/cli/core"

// miso update
func Update(local bool, args []string) error {
	var npmArgs []string
	if local {
		// Local install: npm install @ekkolyth/miso
		npmArgs = []string{"install", "@ekkolyth/miso"}
	} else {
		// Global install: npm install -g @ekkolyth/miso
		npmArgs = []string{"install", "-g", "@ekkolyth/miso"}
	}
	// Append any additional args passed to update command
	npmArgs = append(npmArgs, args...)

	spec := core.ExecSpec{
		Command: "npm",
		Args:    npmArgs,
	}
	if err := core.Exec(spec, "npm"); err != nil {
		return err
	}
	return nil
}
