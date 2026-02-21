package commands

import "github.com/ekkolyth/miso/internal/manager"

// miso upgrade
func Upgrade(local bool, args []string) error {
	var spec manager.ExecSpec
	if local {
		npmArgs := append([]string{"install", "@ekkolyth/miso"}, args...)
		spec = manager.ExecSpec{Command: "npm", Args: npmArgs}
	} else {
		npmArgs := append([]string{"npm", "install", "-g", "@ekkolyth/miso"}, args...)
		spec = manager.ExecSpec{Command: "sudo", Args: npmArgs}
	}
	return manager.Exec(spec, "")
}
