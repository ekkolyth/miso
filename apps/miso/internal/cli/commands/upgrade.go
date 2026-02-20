package commands

import "github.com/ekkolyth/miso/internal/manager"

// miso upgrade
func Upgrade(local bool, args []string) error {
	var npmArgs []string
	if local {
		npmArgs = []string{"install", "@ekkolyth/miso"}
	} else {
		npmArgs = []string{"sudo npm install", "-g", "@ekkolyth/miso"}
	}
	npmArgs = append(npmArgs, args...)

	spec := manager.ExecSpec{
		Command: "npm",
		Args:    npmArgs,
	}
	return manager.Exec(spec, "npm", "")
}
