package commands_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/ekkolyth/miso/internal/cli/commands"
	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/manager"
	"github.com/ekkolyth/miso/internal/ui"
	"github.com/ekkolyth/miso/internal/workspace"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func detectAndWrite(t *testing.T, root string) ([]string, error) {
	t.Helper()
	apps, err := workspace.Detect(root)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	return commands.WriteConfigsForTest(root, apps)
}

func TestWriteConfigsAngularSharp(t *testing.T) {
	root := t.TempDir()
	webPackage := `{"name":"web","packageManager":"bun@1.4.2"}`
	writeTree(t, root, map[string]string{
		"web/package.json":  webPackage,
		"web/tsconfig.json": "{}",
		"api/api.csproj":    "<Project/>",
	})

	written, err := detectAndWrite(t, root)
	if err != nil {
		t.Fatalf("writeConfigs: %v", err)
	}
	want := []string{"api/miso.json", "web/miso.json", "miso.json"}
	if !slices.Equal(written, want) {
		t.Fatalf("written = %v, want %v", written, want)
	}

	rootCfg, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if !rootCfg.StandaloneMode() {
		t.Error("root StandaloneMode() = false, want true")
	}
	apiCfg, err := config.Load(filepath.Join(root, "api"))
	if err != nil {
		t.Fatal(err)
	}
	if !apiCfg.StandaloneMode() {
		t.Error("api StandaloneMode() = false, want true")
	}
	webCfg, err := config.Load(filepath.Join(root, "web"))
	if err != nil {
		t.Fatal(err)
	}
	if webCfg.PackageManager != nil {
		t.Errorf("web PackageManager = %v, want nil", *webCfg.PackageManager)
	}
	got, err := os.ReadFile(filepath.Join(root, "web", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != webPackage {
		t.Errorf("web/package.json changed: %q", got)
	}
}

func TestWriteConfigsWorkspaceJSAppGetsNoMisoJSON(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"package.json":          `{"workspaces":["apps/*"]}`,
		"apps/web/package.json": `{"name":"web"}`,
		"apps/api/go.mod":       "module api\n",
	})

	if _, err := detectAndWrite(t, root); err != nil {
		t.Fatalf("writeConfigs: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "apps", "web", "miso.json")); !os.IsNotExist(err) {
		t.Errorf("apps/web/miso.json should not exist, stat err = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "apps", "api", "miso.json"))
	if err != nil {
		t.Fatalf("apps/api/miso.json: %v", err)
	}
	if !strings.Contains(string(data), `"packageManager": false`) {
		t.Errorf("apps/api/miso.json lacks packageManager false: %s", data)
	}
	rootCfg, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if rootCfg.PackageManager != nil {
		t.Errorf("root PackageManager = %v, want nil", *rootCfg.PackageManager)
	}
}

func TestWriteConfigsNeverOverwritesMemberConfig(t *testing.T) {
	root := t.TempDir()
	existing := `{"shell":"bash"}`
	writeTree(t, root, map[string]string{
		"api/api.csproj": "<Project/>",
		"api/miso.json":  existing,
	})

	if _, err := detectAndWrite(t, root); err != nil {
		t.Fatalf("writeConfigs: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(root, "api", "miso.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte(existing)) {
		t.Errorf("api/miso.json changed: %q", got)
	}
}

func TestWriteConfigsDetectsTurbo(t *testing.T) {
	for marker, want := range map[string]string{"turbo.json": "turbo", "nx.json": "nx"} {
		t.Run(marker, func(t *testing.T) {
			root := t.TempDir()
			writeTree(t, root, map[string]string{
				"package.json": `{"name":"root"}`,
				marker:         "{}",
			})

			if _, err := detectAndWrite(t, root); err != nil {
				t.Fatalf("writeConfigs: %v", err)
			}
			cfg, err := config.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Repo != want {
				t.Errorf("Repo = %q, want %q", cfg.Repo, want)
			}
		})
	}
}

func TestWriteConfigsRootWrittenLast(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("read-only directories do not block root")
	}
	root := t.TempDir()
	writeTree(t, root, map[string]string{"api/api.csproj": "<Project/>"})
	api := filepath.Join(root, "api")
	if err := os.Chmod(api, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(api, 0o755) })

	written, err := detectAndWrite(t, root)
	if err == nil {
		t.Fatal("writeConfigs error = nil, want a write failure")
	}
	if slices.Contains(written, "miso.json") {
		t.Errorf("written = %v, should not contain the root", written)
	}
	if _, statErr := os.Stat(filepath.Join(root, "miso.json")); !os.IsNotExist(statErr) {
		t.Errorf("root miso.json should not exist, stat err = %v", statErr)
	}
}

func TestFallbackTitle(t *testing.T) {
	if got, want := commands.FallbackTitleForTest(true), "No apps selected. How do you want to set up miso?"; got != want {
		t.Errorf("FallbackTitle(true) = %q, want %q", got, want)
	}
	if got, want := commands.FallbackTitleForTest(false), "miso didn't find any apps here. How do you want to set up miso?"; got != want {
		t.Errorf("FallbackTitle(false) = %q, want %q", got, want)
	}
}

func TestAppLine(t *testing.T) {
	styles := ui.Default()

	badged := ansi.Strip(commands.AppLineForTest(
		workspace.DetectedApp{RelPath: "apps/web", Label: "Typescript", Badge: "bun"}, styles))
	if !strings.Contains(badged, "apps/web — Typescript") || !strings.Contains(badged, "bun") {
		t.Fatalf("badged line = %q", badged)
	}
	between := badged[strings.Index(badged, "Typescript"):strings.Index(badged, "bun")]
	if strings.ContainsAny(between, ",<[") {
		t.Errorf("badged line has punctuation between label and badge: %q", badged)
	}

	unmanaged := ansi.Strip(commands.AppLineForTest(
		workspace.DetectedApp{RelPath: "tools/legacy", Label: "Javascript", NoManager: true}, styles))
	if !strings.HasSuffix(unmanaged, "no manager found") {
		t.Errorf("unmanaged line = %q", unmanaged)
	}
}

func TestRepoFieldForOrchestration_Turbo(t *testing.T) {
	if got := commands.RepoFieldForOrchestration("turbo"); got != "turbo" {
		t.Errorf("RepoFieldForOrchestration(turbo) = %q, want %q", got, "turbo")
	}
}

func TestRepoFieldForOrchestration_Nx(t *testing.T) {
	if got := commands.RepoFieldForOrchestration("nx"); got != "nx" {
		t.Errorf("RepoFieldForOrchestration(nx) = %q, want %q", got, "nx")
	}
}

func TestRepoFieldForOrchestration_Miso(t *testing.T) {
	if got := commands.RepoFieldForOrchestration("miso"); got != "" {
		t.Errorf("RepoFieldForOrchestration(miso) = %q, want empty", got)
	}
}

func TestRepoFieldForOrchestration_Unset(t *testing.T) {
	if got := commands.RepoFieldForOrchestration(""); got != "" {
		t.Errorf("RepoFieldForOrchestration(\"\") = %q, want empty", got)
	}
}

func TestWriteConfigsWorkspaceLaravelAppGetsMemberConfig(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"package.json":                `{"workspaces":["apps/*"]}`,
		"apps/site/composer.json":     "{}",
		"apps/site/artisan":           "",
		"apps/site/package.json":      `{"name":"site"}`,
		"apps/site/package-lock.json": "{}",
	})

	if _, err := detectAndWrite(t, root); err != nil {
		t.Fatalf("writeConfigs: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "apps", "site", "miso.json"))
	if err != nil {
		t.Fatalf("apps/site/miso.json: %v", err)
	}
	if strings.Contains(string(data), "packageManager") {
		t.Errorf("apps/site/miso.json should not set packageManager: %s", data)
	}
}

func TestWriteConfigsTurboWinsOverNx(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"package.json": `{"name":"root"}`,
		"turbo.json":   "{}",
		"nx.json":      "{}",
	})

	if _, err := detectAndWrite(t, root); err != nil {
		t.Fatalf("writeConfigs: %v", err)
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Repo != "turbo" {
		t.Errorf("Repo = %q, want turbo", cfg.Repo)
	}
}

func TestFallbackOptions(t *testing.T) {
	values := func(hasPackageJSON bool) []string {
		var got []string
		for _, option := range commands.FallbackOptionsForTest(hasPackageJSON) {
			got = append(got, option.Value)
		}
		return got
	}

	if got, want := values(false), []string{"new", "standalone"}; !slices.Equal(got, want) {
		t.Errorf("without package.json = %v, want %v", got, want)
	}
	if got, want := values(true), []string{"standalone"}; !slices.Equal(got, want) {
		t.Errorf("with package.json = %v, want %v", got, want)
	}
}

func TestSelectedApps(t *testing.T) {
	apps := []workspace.DetectedApp{
		{RelPath: "apps/web", InWorkspace: true},
		{RelPath: "apps/api", InWorkspace: true},
		{RelPath: "tools/legacy"},
		{RelPath: "tools/learning"},
	}
	relPaths := func(picked []int) []string {
		var got []string
		for _, app := range commands.SelectedAppsForTest(apps, picked) {
			got = append(got, app.RelPath)
		}
		return got
	}

	if got, want := relPaths(nil), []string{"apps/web", "apps/api"}; !slices.Equal(got, want) {
		t.Errorf("nothing picked = %v, want only workspace apps %v", got, want)
	}
	if got, want := relPaths([]int{3}), []string{"apps/web", "apps/api", "tools/learning"}; !slices.Equal(got, want) {
		t.Errorf("one picked = %v, want %v", got, want)
	}
	if got, want := relPaths([]int{3, 2}), []string{"apps/web", "apps/api", "tools/legacy", "tools/learning"}; !slices.Equal(got, want) {
		t.Errorf("picked out of order = %v, want Detect order %v", got, want)
	}
}

func TestSelectedAppsNoWorkspaceNothingPicked(t *testing.T) {
	apps := []workspace.DetectedApp{{RelPath: "tools/legacy"}}
	if got := commands.SelectedAppsForTest(apps, nil); len(got) != 0 {
		t.Errorf("selected = %v, want none", got)
	}
}

func detectApps(t *testing.T, root string) []workspace.DetectedApp {
	t.Helper()
	apps, err := workspace.Detect(root)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	return apps
}

func TestWritePackageManagersSetsFieldAndKeepsOtherKeys(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"tools/legacy/package.json": `{"name":"legacy","version":"1.0.0","scripts":{"build":"tsc"}}`,
	})
	apps := detectApps(t, root)
	if len(apps) != 1 || !apps[0].NoManager {
		t.Fatalf("apps = %+v, want one NoManager app", apps)
	}

	written, err := commands.WritePackageManagersForTest(apps, map[string]string{"tools/legacy": "bun"})
	if err != nil {
		t.Fatalf("writePackageManagers: %v", err)
	}
	if want := []string{"tools/legacy/package.json"}; !slices.Equal(written, want) {
		t.Errorf("written = %v, want %v", written, want)
	}

	data, err := os.ReadFile(filepath.Join(root, "tools", "legacy", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pkg map[string]any
	if err := json.Unmarshal(data, &pkg); err != nil {
		t.Fatal(err)
	}
	if got, _ := pkg["packageManager"].(string); got != manager.ResolveManagerVersion("bun") {
		t.Errorf("packageManager = %q, want %q", got, manager.ResolveManagerVersion("bun"))
	}
	if pkg["name"] != "legacy" || pkg["version"] != "1.0.0" || pkg["scripts"] == nil {
		t.Errorf("other keys lost: %s", data)
	}
	if !strings.Contains(string(data), "\n  \"name\"") {
		t.Errorf("not 2-space indented: %s", data)
	}

	name, err := manager.Resolve(filepath.Join(root, "tools", "legacy"))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if name != "bun" {
		t.Errorf("Resolve = %q, want bun", name)
	}
}

func TestWritePackageManagersLeavesManagedAppAlone(t *testing.T) {
	root := t.TempDir()
	managed := `{"name":"web","packageManager":"bun@1.4.2"}`
	writeTree(t, root, map[string]string{"web/package.json": managed})
	apps := detectApps(t, root)

	written, err := commands.WritePackageManagersForTest(apps, map[string]string{"web": "npm"})
	if err != nil {
		t.Fatalf("writePackageManagers: %v", err)
	}
	if len(written) != 0 {
		t.Errorf("written = %v, want none", written)
	}
	got, err := os.ReadFile(filepath.Join(root, "web", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != managed {
		t.Errorf("web/package.json changed: %q", got)
	}
}

func TestWritePackageManagersThenRootConfigLast(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"tools/legacy/package.json": `{"name":"legacy"}`,
	})
	apps := detectApps(t, root)

	if _, err := commands.WritePackageManagersForTest(apps, map[string]string{"tools/legacy": "pnpm"}); err != nil {
		t.Fatalf("writePackageManagers: %v", err)
	}
	written, err := commands.WriteConfigsForTest(root, apps)
	if err != nil {
		t.Fatalf("writeConfigs: %v", err)
	}
	if got := written[len(written)-1]; got != "miso.json" {
		t.Errorf("last written = %q, want root miso.json", got)
	}
}
