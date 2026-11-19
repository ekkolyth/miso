package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

var contractArgs = []string{
	"error",
	"Building the Debian VM image requires Lima 2.x, but limactl was not found.",
	"install=brew install lima",
	"then=miso dev",
}

func TestRunLogEmitsWireLineUnderMiso(t *testing.T) {
	t.Setenv("MISO", "1")
	sample, err := os.ReadFile(filepath.Join("..", "..", "scriptlog", "testdata", "contracts", "log-line.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := RunLog(contractArgs, &stdout, &stderr); err != nil {
		t.Fatalf("RunLog() error = %v", err)
	}
	if want := string(bytes.TrimRight(sample, "\n")) + "\n"; stdout.String() != want {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunLogRendersWithoutMiso(t *testing.T) {
	t.Setenv("MISO", "")
	var stdout, stderr bytes.Buffer
	if err := RunLog(contractArgs, &stdout, &stderr); err != nil {
		t.Fatalf("RunLog() error = %v", err)
	}
	want := `ERROR Building the Debian VM image requires Lima 2.x, but limactl was not found. install="brew install lima" then="miso dev"` + "\n"
	if got := ansi.Strip(stderr.String()); got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunLogSplitsOnFirstEquals(t *testing.T) {
	t.Setenv("MISO", "1")
	var stdout, stderr bytes.Buffer
	if err := RunLog([]string{"info", "t", "url=a=b"}, &stdout, &stderr); err != nil {
		t.Fatalf("RunLog() error = %v", err)
	}
	want := `::miso::{"level":"info","text":"t","fields":[["url","a=b"]]}` + "\n"
	if stdout.String() != want {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
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
			t.Setenv("MISO", "1")
			var stdout, stderr bytes.Buffer
			err := RunLog(tt.args, &stdout, &stderr)
			if err == nil || err.Error() != tt.want {
				t.Fatalf("RunLog() error = %v, want %q", err, tt.want)
			}
			if stdout.Len() != 0 || stderr.Len() != 0 {
				t.Errorf("writers not empty: stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}
