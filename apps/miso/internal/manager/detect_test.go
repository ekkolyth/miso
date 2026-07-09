package manager

import (
	"errors"
	"testing"

	"github.com/ekkolyth/miso/internal/testutil"
)

func TestDetectManager(t *testing.T) {
	tests := []struct {
		name      string
		lockfiles []string
		want      string
		wantErr   string // substring; "" means expect success
	}{
		{"bun lockb", []string{"bun.lockb"}, "bun", ""},
		{"bun lock text", []string{"bun.lock"}, "bun", ""},
		{"npm", []string{"package-lock.json"}, "npm", ""},
		{"pnpm", []string{"pnpm-lock.yaml"}, "pnpm", ""},
		{"yarn", []string{"yarn.lock"}, "yarn", ""},
		{"none", nil, "", "no lockfile"},
		{"conflict", []string{"bun.lockb", "yarn.lock"}, "", "multiple lockfiles"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			testutil.WriteFiles(t, dir, tt.lockfiles...)

			got, err := DetectManager(dir)
			if tt.wantErr == "" {
				testutil.NoError(t, err)
				testutil.Equal(t, got, tt.want)
			} else {
				testutil.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestDetectManager_NoneReturnsSentinel(t *testing.T) {
	_, err := DetectManager(t.TempDir())
	if !errors.Is(err, ErrNoLockfile) {
		t.Fatalf("want ErrNoLockfile, got %v", err)
	}
}
