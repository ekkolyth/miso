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

func TestInvokedAsMisox(t *testing.T) {
	cases := []struct {
		argv0 string
		want  bool
	}{
		{"/usr/local/bin/misox", true},
		{"misox", true},
		{"misox-darwin-arm64", true},
		{"/home/user/.local/bin/miso", false},
		{"miso", false},
		{"misoxx", false},
		{"notmisox", false},
	}
	for _, tc := range cases {
		if got := cli.InvokedAsMisox(tc.argv0); got != tc.want {
			t.Errorf("InvokedAsMisox(%q) = %v, want %v", tc.argv0, got, tc.want)
		}
	}
}
