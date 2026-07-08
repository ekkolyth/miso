package manager_test

import (
	"reflect"
	"testing"

	"github.com/ekkolyth/miso/internal/manager"
	"github.com/ekkolyth/miso/internal/manager/bun"
	"github.com/ekkolyth/miso/internal/manager/npm"
	"github.com/ekkolyth/miso/internal/manager/pnpm"
	"github.com/ekkolyth/miso/internal/manager/yarn"
)

func TestBuildDlx(t *testing.T) {
	cases := []struct {
		name     string
		driver   manager.Manager
		expected manager.ExecSpec
	}{
		{"bun", bun.Bun{}, manager.ExecSpec{Command: "bunx", Args: []string{"skills", "add", "src"}}},
		{"npm", npm.Npm{}, manager.ExecSpec{Command: "npx", Args: []string{"skills", "add", "src"}}},
		{"pnpm", pnpm.Pnpm{}, manager.ExecSpec{Command: "pnpm", Args: []string{"dlx", "skills", "add", "src"}}},
		{"yarn", yarn.Yarn{}, manager.ExecSpec{Command: "yarn", Args: []string{"dlx", "skills", "add", "src"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.driver.BuildDlx("skills", []string{"add", "src"})
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("BuildDlx = %+v, want %+v", got, tc.expected)
			}
		})
	}
}
