package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ekkolyth/miso/internal/config"
)

func TestEffectiveConfig_MemberOverridesShell(t *testing.T) {
	root := t.TempDir()
	memberDir := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(memberDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memberDir, "miso.json"),
		[]byte(`{"shell":"zsh","scripts":"./tasks"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	base := config.Config{Shell: "bash", Scripts: "./scripts", Repo: "miso"}
	member := Member{Name: "web", Dir: memberDir, ConfigPath: filepath.Join(memberDir, "miso.json")}

	got := EffectiveConfig(base, member)
	if got.Shell != "zsh" {
		t.Errorf("Shell = %q, want zsh (member override)", got.Shell)
	}
	if got.Scripts != "./tasks" {
		t.Errorf("Scripts = %q, want ./tasks (member override)", got.Scripts)
	}
}

func TestEffectiveConfig_InheritsRootWhenAbsent(t *testing.T) {
	root := t.TempDir()
	memberDir := filepath.Join(root, "apps", "api")
	if err := os.MkdirAll(memberDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memberDir, "miso.json"),
		[]byte(`{"scripts":"./tasks"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	base := config.Config{Shell: "bash", Scripts: "./scripts"}
	member := Member{Name: "api", Dir: memberDir, ConfigPath: filepath.Join(memberDir, "miso.json")}

	got := EffectiveConfig(base, member)
	if got.Shell != "bash" {
		t.Errorf("Shell = %q, want bash (inherited)", got.Shell)
	}
}

func TestEffectiveConfig_NoMemberConfig_ReturnsRoot(t *testing.T) {
	base := config.Config{Shell: "bash", Scripts: "./scripts"}
	member := Member{Name: "web", Dir: t.TempDir(), ConfigPath: ""}
	got := EffectiveConfig(base, member)
	if got.Shell != "bash" || got.Scripts != "./scripts" {
		t.Errorf("got %+v, want unchanged root", got)
	}
}
