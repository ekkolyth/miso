package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func Exec(spec ExecSpec, detectedManager string) error {
	if os.Getenv("MISO_DEBUG") == "1" {
		fmt.Printf("miso: detected manager: %s\n", detectedManager)
	}
	fmt.Printf("miso: running: %s %s\n", spec.Command, strings.Join(spec.Args, " "))

	if _, err := exec.LookPath(spec.Command); err != nil {
		return fmt.Errorf("%s is not on PATH", spec.Command)
	}

	cmd := exec.Command(spec.Command, spec.Args...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	return cmd.Run()
}



