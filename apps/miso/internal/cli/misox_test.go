package cli_test

import (
	"testing"

	"github.com/ekkolyth/miso/internal/cli"
)

func TestRunMisox_unknownManager(t *testing.T) {
	err := cli.RunMisox("unknown-manager", "some-pkg", nil, "/tmp")
	if err == nil {
		t.Fatal("expected error for unknown manager, got nil")
	}
}
