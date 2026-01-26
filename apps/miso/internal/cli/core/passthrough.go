package core

import (
	"fmt"
	"os"
)

// forward unknown commands to package manager
func RunPassthrough(managerName string, command string, args []string, workDir string) error {
	fmt.Fprintf(os.Stderr, "miso: command %q not found in scripts folder or package.json, forwarding to %s\n", command, managerName)
	spec := ExecSpec{
		Command: managerName,
		Args:    append([]string{command}, args...),
	}
	if err := Exec(spec, managerName, workDir); err != nil {
		return err
	}
	return nil
}
