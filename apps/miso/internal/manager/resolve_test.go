package manager_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ekkolyth/miso/internal/manager"
	"github.com/ekkolyth/miso/internal/manager/drivers"
)

func init() { drivers.Register() }

func TestResolve(t *testing.T) {
	cases := []struct {
		name      string
		files     map[string]string
		want      string
		wantErr   string
		wantIsErr error
	}{
		{
			name: "field wins over two lockfiles",
			files: map[string]string{
				"package.json":      `{"packageManager":"bun@1.4.2"}`,
				"bun.lock":          "",
				"package-lock.json": "",
			},
			want: "bun",
		},
		{
			name:  "lockfile without package.json",
			files: map[string]string{"pnpm-lock.yaml": ""},
			want:  "pnpm",
		},
		{
			name: "package.json without field falls back to lockfile",
			files: map[string]string{
				"package.json": `{}`,
				"yarn.lock":    "",
			},
			want: "yarn",
		},
		{
			name:    "unsupported manager",
			files:   map[string]string{"package.json": `{"packageManager":"deno@2"}`},
			wantErr: `unsupported package manager "deno"`,
		},
		{
			name:      "empty directory",
			files:     map[string]string{},
			wantIsErr: manager.ErrNoLockfile,
		},
		{
			name:    "invalid package.json",
			files:   map[string]string{"package.json": `{nope`},
			wantErr: "parse package.json",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, body := range tc.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			got, err := manager.Resolve(dir)

			switch {
			case tc.wantIsErr != nil:
				if !errors.Is(err, tc.wantIsErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantIsErr)
				}
			case tc.wantErr != "":
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
				}
			default:
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tc.want {
					t.Fatalf("got %q, want %q", got, tc.want)
				}
			}
		})
	}
}
