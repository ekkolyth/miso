package commands

import (
	"bytes"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

var exampleArgs = []string{
	"error",
	"Building the Debian VM image requires Lima 2.x, but limactl was not found.",
	"install=brew install lima",
	"then=miso dev",
}

func TestRunLogWritesRenderedLine(t *testing.T) {
	var stdout bytes.Buffer
	if err := RunLog(exampleArgs, &stdout); err != nil {
		t.Fatalf("RunLog() error = %v", err)
	}
	want := `ERROR Building the Debian VM image requires Lima 2.x, but limactl was not found. install="brew install lima" then="miso dev"` + "\n"
	if got := ansi.Strip(stdout.String()); got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestRunLogSplitsOnFirstEquals(t *testing.T) {
	var stdout bytes.Buffer
	if err := RunLog([]string{"info", "t", "url=a=b"}, &stdout); err != nil {
		t.Fatalf("RunLog() error = %v", err)
	}
	if got, want := ansi.Strip(stdout.String()), "INFO t url=a=b\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestRunLogErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no args", []string{}, "missing level (use debug, info, warn, or error)"},
		{"unknown level", []string{"fatal", "x"}, `unknown level "fatal" (use debug, info, warn, or error)`},
		{"no text", []string{"error"}, "missing message text"},
		{"empty text", []string{"error", ""}, "missing message text"},
		{"field without equals", []string{"error", "disk", "full"}, `field "full" is not key=value`},
		{"field with empty key", []string{"error", "disk", "=v"}, `field "=v" has an empty key`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			err := RunLog(tt.args, &stdout)
			if err == nil || err.Error() != tt.want {
				t.Fatalf("RunLog() error = %v, want %q", err, tt.want)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}
