package workspace

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ekkolyth/miso/internal/manager/drivers"
)

func init() { drivers.Register() }

func TestDetect(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  []DetectedApp
	}{
		{
			name: "angular-sharp",
			files: map[string]string{
				"web/package.json":      `{"packageManager":"bun@1.4.2"}`,
				"web/tsconfig.json":     `{}`,
				"web/bun.lock":          ``,
				"web/package-lock.json": `{}`,
				"api/api.csproj":        ``,
			},
			want: []DetectedApp{
				{RelPath: "api", Label: "C#"},
				{RelPath: "web", Label: "Typescript", Badge: "bun", HasPackageJSON: true},
			},
		},
		{
			name: "monorepo",
			files: map[string]string{
				"package.json":                  `{"workspaces":["apps/*"]}`,
				"apps/web/package.json":         `{"packageManager":"pnpm@9.0.0"}`,
				"apps/web/tsconfig.json":        `{}`,
				"apps/api/go.mod":               ``,
				"apps/site/composer.json":       `{}`,
				"apps/site/artisan":             ``,
				"apps/site/package.json":        `{}`,
				"apps/site/package-lock.json":   `{}`,
				"apps/cli/Cargo.toml":           `[package]`,
				"tools/legacy/package.json":     `{}`,
				"tools/learning/pyproject.toml": ``,
				"tools/learning/uv.lock":        ``,
			},
			want: []DetectedApp{
				{RelPath: "apps/api", Label: "Go", InWorkspace: true},
				{RelPath: "apps/cli", Label: "Rust", InWorkspace: true},
				{RelPath: "apps/site", Label: "Laravel", Badge: "npm", HasPackageJSON: true, InWorkspace: true},
				{RelPath: "apps/web", Label: "Typescript", Badge: "pnpm", HasPackageJSON: true, InWorkspace: true},
				{RelPath: "tools/learning", Label: "Python", Badge: "uv"},
				{RelPath: "tools/legacy", Label: "Javascript", NoManager: true, HasPackageJSON: true},
			},
		},
		{
			name:  "plain single app",
			files: map[string]string{"package.json": `{}`, "bun.lock": ``},
			want:  []DetectedApp{{RelPath: ".", Label: "Javascript", Badge: "bun", HasPackageJSON: true}},
		},
		{
			name: "go.work",
			files: map[string]string{
				"go.work":      `go 1.22`,
				"go.mod":       `module x`,
				"svc/a/go.mod": ``,
				"svc/b/go.mod": ``,
			},
			want: []DetectedApp{
				{RelPath: "svc/a", Label: "Go"},
				{RelPath: "svc/b", Label: "Go"},
			},
		},
		{
			name: "cargo workspace",
			files: map[string]string{
				"Cargo.toml":          "[workspace]\nmembers = [\"crates/*\"]\n",
				"crates/x/Cargo.toml": `[package]`,
			},
			want: []DetectedApp{{RelPath: ".", Label: "Rust"}},
		},
		{
			name: "sln",
			files: map[string]string{
				"App.sln":            ``,
				"src/Api/Api.csproj": ``,
				"src/Lib/Lib.fsproj": ``,
			},
			want: []DetectedApp{
				{RelPath: "src/Api", Label: "C#"},
				{RelPath: "src/Lib", Label: "F#"},
			},
		},
		{
			name: "java and python poetry",
			files: map[string]string{
				"svc/pom.xml":       ``,
				"ml/pyproject.toml": ``,
				"ml/poetry.lock":    ``,
			},
			want: []DetectedApp{
				{RelPath: "ml", Label: "Python", Badge: "poetry"},
				{RelPath: "svc", Label: "Java/Kotlin"},
			},
		},
		{
			name: "skips",
			files: map[string]string{
				"node_modules/x/package.json": `{}`,
				".venv/pyproject.toml":        ``,
			},
			want: nil,
		},
		{
			name: "has config",
			files: map[string]string{
				"api/api.csproj": ``,
				"api/miso.json":  `{}`,
			},
			want: []DetectedApp{{RelPath: "api", Label: "C#", HasConfig: true}},
		},
		{
			name: "workspace exclusion wins",
			files: map[string]string{
				"package.json":          `{"workspaces":["apps/*","!apps/legacy"]}`,
				"apps/legacy/go.mod":    ``,
				"apps/legacy/miso.json": `{}`,
				"apps/api/go.mod":       ``,
			},
			want: []DetectedApp{{RelPath: "apps/api", Label: "Go", InWorkspace: true}},
		},
		{
			name: "glob member deeper than the scan",
			files: map[string]string{
				"package.json":              `{"workspaces":["packages/*/*"]}`,
				"packages/a/b/package.json": `{}`,
				"packages/a/b/go.mod":       ``,
			},
			want: []DetectedApp{{RelPath: "packages/a/b", Label: "Go", HasPackageJSON: true, NoManager: true, InWorkspace: true}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			for rel, content := range tt.files {
				writeTreeFile(t, filepath.Join(root, filepath.FromSlash(rel)), content)
			}
			got, err := Detect(root)
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			for i := range got {
				if want := filepath.Join(root, filepath.FromSlash(got[i].RelPath)); got[i].Dir != want && got[i].RelPath != "." {
					t.Errorf("Dir = %q, want %q", got[i].Dir, want)
				}
				got[i].Dir = ""
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v\nwant %+v", got, tt.want)
			}
		})
	}
}

func TestDetectMultipleLockfilesErrors(t *testing.T) {
	root := t.TempDir()
	writeTreeFile(t, filepath.Join(root, "web", "package.json"), `{}`)
	writeTreeFile(t, filepath.Join(root, "web", "bun.lock"), ``)
	writeTreeFile(t, filepath.Join(root, "web", "yarn.lock"), ``)

	_, err := Detect(root)
	if err == nil || !strings.Contains(err.Error(), "web") {
		t.Fatalf("err = %v, want error naming web", err)
	}
}

func writeTreeFile(t *testing.T, path, body string) {
	t.Helper()
	mkdirAll(t, filepath.Dir(path))
	writeFile(t, path, body)
}
