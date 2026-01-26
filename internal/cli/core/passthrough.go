package core

// forward unknown commands to package manager
func RunPassthrough(managerName string, command string, args []string) error {
	spec := ExecSpec{
		Command: managerName,
		Args:    append([]string{command}, args...),
	}
	if err := Exec(spec, managerName); err != nil {
		return err
	}
	return nil
}
