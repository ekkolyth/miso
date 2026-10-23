package turbo

import (
	"reflect"
	"testing"
)

func TestSplitFlags(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		chromeActive bool
		wantTurbo    []string
	}{
		{
			name:         "all passthrough",
			args:         []string{"--filter=web", "--concurrency=2"},
			chromeActive: false,
			wantTurbo:    []string{"--filter=web", "--concurrency=2"},
		},
		{
			name:         "--env after the command goes to turbo",
			args:         []string{"--env", "--filter=web"},
			chromeActive: false,
			wantTurbo:    []string{"--env", "--filter=web"},
		},
		{
			name:         "--log-order=value stripped with TUI active",
			args:         []string{"--log-order=grouped", "--filter=web"},
			chromeActive: true,
			wantTurbo:    []string{"--filter=web"},
		},
		{
			name:         "--log-order value space-separated stripped with TUI active",
			args:         []string{"--log-order", "grouped", "--filter=web"},
			chromeActive: true,
			wantTurbo:    []string{"--filter=web"},
		},
		{
			name:         "--log-order=value kept without TUI",
			args:         []string{"--log-order=grouped", "--filter=web"},
			chromeActive: false,
			wantTurbo:    []string{"--log-order=grouped", "--filter=web"},
		},
		{
			name:         "--log-order value space-separated kept without TUI",
			args:         []string{"--log-order", "grouped", "--filter=web"},
			chromeActive: false,
			wantTurbo:    []string{"--log-order", "grouped", "--filter=web"},
		},
		{
			name:         "empty args",
			args:         []string{},
			chromeActive: false,
			wantTurbo:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTurbo := SplitFlags(tt.args, tt.chromeActive)

			if gotTurbo == nil {
				gotTurbo = []string{}
			}

			if !reflect.DeepEqual(gotTurbo, tt.wantTurbo) {
				t.Errorf("SplitFlags() turbo = %v, want %v", gotTurbo, tt.wantTurbo)
			}
		})
	}
}
