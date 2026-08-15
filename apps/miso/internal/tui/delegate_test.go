package tui

import (
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/ekkolyth/miso/internal/turbo"
)

func TestForceColorEnv(t *testing.T) {
	t.Run("forces FORCE_COLOR when unset", func(t *testing.T) {
		got := forceColorEnv([]string{"PATH=/bin", "TERM=xterm-256color"})
		if !slices.Contains(got, "FORCE_COLOR=1") {
			t.Errorf("expected FORCE_COLOR=1 to be appended, got %v", got)
		}
	})

	t.Run("respects explicit NO_COLOR", func(t *testing.T) {
		got := forceColorEnv([]string{"PATH=/bin", "NO_COLOR=1"})
		if slices.Contains(got, "FORCE_COLOR=1") {
			t.Errorf("must not force color when NO_COLOR is set, got %v", got)
		}
	})

	t.Run("leaves a pre-set FORCE_COLOR untouched", func(t *testing.T) {
		base := []string{"PATH=/bin", "FORCE_COLOR=3"}
		got := forceColorEnv(base)
		if slices.Contains(got, "FORCE_COLOR=1") {
			t.Errorf("must not override caller's FORCE_COLOR, got %v", got)
		}
	})
}

func TestDelegateFilterArgs(t *testing.T) {
	turbo := delegateFilterArgs("turbo", []string{"@ekko/web", "@ekko/api"})
	if len(turbo) != 2 || turbo[0] != "--filter=@ekko/web" || turbo[1] != "--filter=@ekko/api" {
		t.Errorf("turbo filters = %v", turbo)
	}
	nx := delegateFilterArgs("nx", []string{"web", "api"})
	if len(nx) != 1 || nx[0] != "--projects=web,api" {
		t.Errorf("nx filters = %v", nx)
	}
	if len(delegateFilterArgs("turbo", nil)) != 0 {
		t.Error("no filters -> no args")
	}
}

func TestDelegateRunPlain_Succeeds(t *testing.T) {
	dir := t.TempDir()
	ran, err := delegateRunPlain("sh", "sh", []string{"-c", "exit 0"}, dir, os.Environ())
	if !ran {
		t.Error("expected ran = true")
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDelegateRunPlain_FailingCommandErrors(t *testing.T) {
	dir := t.TempDir()
	ran, err := delegateRunPlain("sh", "sh", []string{"-c", "exit 3"}, dir, os.Environ())
	if !ran {
		t.Error("expected ran = true")
	}
	if err == nil {
		t.Error("expected error from failing command")
	}
}

func TestParseNxHeader(t *testing.T) {
	tests := []struct {
		line  string
		isHdr bool
		label string
	}{
		{"> nx run web:build", true, "web:build"},
		{"> nx run shared:test", true, "shared:test"},
		{"regular output", false, ""},
		{"> NX some other message", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			label, isHdr := parseNxHeader(tt.line)
			if isHdr != tt.isHdr {
				t.Errorf("isHeader = %v, want %v", isHdr, tt.isHdr)
			}
			if label != tt.label {
				t.Errorf("label = %q, want %q", label, tt.label)
			}
		})
	}
}

// a partially-failed turbo run reports turbo's own exit, not a count taken from
// the panes: the pane set carries the synthetic "turbo" meta tab, so counting it
// contradicts the summary turbo printed directly above.
func TestDelegateRoutingReportsDelegateExitNotPaneCount(t *testing.T) {
	transcript := []string{
		"@scope/pkg-a:build: cache miss, executing abc123",
		"@scope/pkg-a:build: Build complete",
		"@scope/pkg-a:build: exited with code 0",
		"@scope/pkg-b:build: compiling",
		"@scope/pkg-b:build: ERROR boom",
		"@scope/pkg-b:build: exited with code 1",
		" Tasks:    1 successful, 2 total",
		"Failed:    @scope/pkg-b#build",
		" ERROR  run failed: command exited (1)",
	}

	pm := NewProcessManager()
	router := &delegateRouter{pm: pm, scriptName: "build", root: t.TempDir()}
	for _, line := range transcript {
		router.routeTurbo(turbo.ParseLine(line), line)
	}

	// turbo's non-task lines land in the meta tab, which never reports an exit
	// code of its own, so the post-Wait fallback stamps the run's code on it
	for _, process := range pm.Processes {
		if process.State != StateExited {
			process.State = StateExited
			process.ExitCode = 1
		}
	}

	if got := len(pm.Processes); got != 3 {
		t.Fatalf("panes = %d, want 3 (two tasks plus the meta tab)", got)
	}
	if pm.findProc(metaTabLabel) == nil {
		t.Fatalf("expected a %q meta tab", metaTabLabel)
	}
	if got := pm.FailedCount(); got != 2 {
		t.Fatalf("FailedCount = %d, want 2 — the meta tab counts as failed, which is why it can't drive the summary", got)
	}

	delegateExit := &exec.ExitError{ProcessState: &os.ProcessState{}}
	_, err := delegateResult(nil, delegateExit)
	if err != delegateExit {
		t.Fatalf("err = %v, want the delegate's own exit error", err)
	}
	if err != nil && strings.Contains(err.Error(), "tasks failed") {
		t.Errorf("err %q reports a pane count; turbo already printed its own summary", err.Error())
	}
}

func TestDelegateResultPrefersProgramError(t *testing.T) {
	programErr := errors.New("tui crashed")
	if _, err := delegateResult(programErr, errors.New("exit status 1")); err != programErr {
		t.Errorf("err = %v, want the program error", err)
	}
	if _, err := delegateResult(nil, nil); err != nil {
		t.Errorf("err = %v, want nil on a clean run", err)
	}
}
