package cli

import (
	"fmt"
	"os"

	"github.com/ekkolyth/miso/internal/manager"
)

// forward unknown commands to package manager
func RunPassthrough(managerName string, command string, args []string, workDir string) error {
	fmt.Fprintf(os.Stderr, "miso: command %q not found in scripts folder or package.json, forwarding to %s\n", command, managerName)
	spec := manager.ExecSpec{
		Command: managerName,
		Args:    append([]string{command}, args...),
	}
	return manager.Exec(spec, managerName, workDir)
}
