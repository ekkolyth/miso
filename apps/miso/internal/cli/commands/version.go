package commands

import (
	"fmt"
	"os"
)

// Version is set at build time via -ldflags "-X github.com/ekkolyth/miso/internal/cli/commands.Version=x.y.z"
var Version = "unknown"

// miso version
func RunVersion() error {
	_, _ = fmt.Fprintf(os.Stdout, "miso %s\n", Version)
	return nil
}
