package pm

import "github.com/ekkolyth/miso/internal/cli/core"

// miso upgrade
func Upgrade(local bool, args []string) error {
	var npmArgs []string
	if local {
		npmArgs = []string{"install", "@ekkolyth/miso"}
	} else {
		npmArgs = []string{"sudo npm install", "-g", "@ekkolyth/miso"}
	}
	npmArgs = append(npmArgs, args...)

	spec := core.ExecSpec{
		Command: "npm",
		Args:    npmArgs,
	}
	return core.Exec(spec, "npm", "")
}
