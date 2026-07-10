package turbo

import (
	"testing"
)

func boolPtr(b bool) *bool {
	return &b
}

func TestParseLineColoredPrefix(t *testing.T) {
	// FORCE_COLOR / a color terminal makes turbo wrap the prefix in SGR:
	// "\x1b[35mmcp:dev: \x1b[0mtext". These must still parse (real ekko output).
	m := ParseLine("\x1b[35mmcp:dev: \x1b[0mcache bypass, force executing \x1b[2mabc123\x1b[0m")
	if m.Skip {
		t.Fatal("colored task line was dropped (Skip); every workspace's output would vanish")
	}
	if m.Label != "mcp:dev" {
		t.Errorf("Label = %q, want mcp:dev", m.Label)
	}

	scoped := ParseLine("\x1b[34m@ekko/ui:dev: \x1b[0mStorybook ready")
	if scoped.Skip || scoped.Label != "@ekko/ui:dev" {
		t.Errorf("scoped colored label = %q (skip=%v), want @ekko/ui:dev", scoped.Label, scoped.Skip)
	}
}

func TestParseLineColoredExitAndCache(t *testing.T) {
	ex := ParseLine("\x1b[36mapi:dev: \x1b[0m\x1b[31mexited with code 1\x1b[0m")
	if !ex.IsExit || ex.ExitCode != 1 {
		t.Errorf("colored exit line: IsExit=%v ExitCode=%d, want true/1", ex.IsExit, ex.ExitCode)
	}
	ch := ParseLine("\x1b[32mweb:build: \x1b[0m\x1b[2mcache hit, replaying logs\x1b[0m")
	if ch.CacheHit == nil || !*ch.CacheHit {
		t.Errorf("colored cache-hit not detected: %+v", ch)
	}
}

func TestParseLineWorkspaceOutput(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantLabel string
		wantText  string
		wantSkip  bool
	}{
		{
			name:      "simple workspace output",
			line:      "my-app:build: some build output",
			wantLabel: "my-app:build",
			wantText:  "some build output",
			wantSkip:  false,
		},
		{
			name:      "scoped package output",
			line:      "@org/pkg:test: running tests",
			wantLabel: "@org/pkg:test",
			wantText:  "running tests",
			wantSkip:  false,
		},
		{
			name:      "nested path package",
			line:      "packages/utils:lint: linting files",
			wantLabel: "packages/utils:lint",
			wantText:  "linting files",
			wantSkip:  false,
		},
		{
			name:     "boilerplate line no match skipped",
			line:     "  some random line without label",
			wantSkip: true,
		},
		{
			name:     "empty line skipped",
			line:     "",
			wantSkip: true,
		},
		{
			name:     "turbo header line skipped",
			line:     "• Packages in scope: my-app",
			wantSkip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseLine(tt.line)
			if got.Skip != tt.wantSkip {
				t.Errorf("ParseLine(%q).Skip = %v, want %v", tt.line, got.Skip, tt.wantSkip)
			}
			if !tt.wantSkip {
				if got.Label != tt.wantLabel {
					t.Errorf("ParseLine(%q).Label = %q, want %q", tt.line, got.Label, tt.wantLabel)
				}
				if got.Text != tt.wantText {
					t.Errorf("ParseLine(%q).Text = %q, want %q", tt.line, got.Text, tt.wantText)
				}
			}
		})
	}
}

func TestParseLineExitCode(t *testing.T) {
	tests := []struct {
		name         string
		line         string
		wantIsExit   bool
		wantExitCode int
	}{
		{
			name:         "exit code 1",
			line:         "my-app:build: exited with code 1",
			wantIsExit:   true,
			wantExitCode: 1,
		},
		{
			name:         "exit code 0",
			line:         "my-app:test: exited with code 0",
			wantIsExit:   true,
			wantExitCode: 0,
		},
		{
			name:         "exit code 127",
			line:         "packages/cli:dev: exited with code 127",
			wantIsExit:   true,
			wantExitCode: 127,
		},
		{
			name:       "normal output not exit",
			line:       "my-app:build: compiling...",
			wantIsExit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseLine(tt.line)
			if got.IsExit != tt.wantIsExit {
				t.Errorf("ParseLine(%q).IsExit = %v, want %v", tt.line, got.IsExit, tt.wantIsExit)
			}
			if tt.wantIsExit && got.ExitCode != tt.wantExitCode {
				t.Errorf("ParseLine(%q).ExitCode = %d, want %d", tt.line, got.ExitCode, tt.wantExitCode)
			}
		})
	}
}

func TestParseLineCacheStatus(t *testing.T) {
	tests := []struct {
		name         string
		line         string
		wantCacheHit *bool
	}{
		{
			name:         "cache hit",
			line:         "my-app:build: cache hit, replaying logs",
			wantCacheHit: boolPtr(true),
		},
		{
			name:         "cache miss",
			line:         "my-app:build: cache miss, executing",
			wantCacheHit: boolPtr(false),
		},
		{
			name:         "normal output no cache status",
			line:         "my-app:build: compiling source files",
			wantCacheHit: nil,
		},
		{
			name:         "exit code line no cache status",
			line:         "my-app:build: exited with code 1",
			wantCacheHit: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseLine(tt.line)
			if tt.wantCacheHit == nil {
				if got.CacheHit != nil {
					t.Errorf("ParseLine(%q).CacheHit = %v, want nil", tt.line, *got.CacheHit)
				}
			} else {
				if got.CacheHit == nil {
					t.Errorf("ParseLine(%q).CacheHit = nil, want %v", tt.line, *tt.wantCacheHit)
				} else if *got.CacheHit != *tt.wantCacheHit {
					t.Errorf("ParseLine(%q).CacheHit = %v, want %v", tt.line, *got.CacheHit, *tt.wantCacheHit)
				}
			}
		})
	}
}
