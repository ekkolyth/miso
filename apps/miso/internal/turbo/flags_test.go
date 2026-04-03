package turbo

import (
	"reflect"
	"testing"
)

func TestSplitFlags(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		tuiActive bool
		wantMiso  []string
		wantTurbo []string
	}{
		{
			name:      "all passthrough",
			args:      []string{"--filter=web", "--concurrency=2"},
			tuiActive: false,
			wantMiso:  []string{},
			wantTurbo: []string{"--filter=web", "--concurrency=2"},
		},
		{
			name:      "miso --env stripped",
			args:      []string{"--env", "--filter=web"},
			tuiActive: false,
			wantMiso:  []string{"--env"},
			wantTurbo: []string{"--filter=web"},
		},
		{
			name:      "--log-order=value stripped with TUI active",
			args:      []string{"--log-order=grouped", "--filter=web"},
			tuiActive: true,
			wantMiso:  []string{},
			wantTurbo: []string{"--filter=web"},
		},
		{
			name:      "--log-order value space-separated stripped with TUI active",
			args:      []string{"--log-order", "grouped", "--filter=web"},
			tuiActive: true,
			wantMiso:  []string{},
			wantTurbo: []string{"--filter=web"},
		},
		{
			name:      "--log-order=value kept without TUI",
			args:      []string{"--log-order=grouped", "--filter=web"},
			tuiActive: false,
			wantMiso:  []string{},
			wantTurbo: []string{"--log-order=grouped", "--filter=web"},
		},
		{
			name:      "--log-order value space-separated kept without TUI",
			args:      []string{"--log-order", "grouped", "--filter=web"},
			tuiActive: false,
			wantMiso:  []string{},
			wantTurbo: []string{"--log-order", "grouped", "--filter=web"},
		},
		{
			name:      "empty args",
			args:      []string{},
			tuiActive: false,
			wantMiso:  []string{},
			wantTurbo: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMiso, gotTurbo := SplitFlags(tt.args, tt.tuiActive)

			if gotMiso == nil {
				gotMiso = []string{}
			}
			if gotTurbo == nil {
				gotTurbo = []string{}
			}

			if !reflect.DeepEqual(gotMiso, tt.wantMiso) {
				t.Errorf("SplitFlags() miso = %v, want %v", gotMiso, tt.wantMiso)
			}
			if !reflect.DeepEqual(gotTurbo, tt.wantTurbo) {
				t.Errorf("SplitFlags() turbo = %v, want %v", gotTurbo, tt.wantTurbo)
			}
		})
	}
}
