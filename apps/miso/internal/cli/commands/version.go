package commands

import (
	"fmt"
	"os"
)

// Version is set at build time via -ldflags "-X github.com/ekkolyth/miso/internal/cli/commands.Version=x.y.z"
var Version = "unknown"

// miso version — prints the bare version ("dev" for local builds), never
// "miso <v>" (which reads like the `miso dev` command and isn't parseable).
func RunVersion() error {
	_, _ = fmt.Fprintf(os.Stdout, "%s\n", Version)
	return nil
}
