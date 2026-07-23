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

// TestEffectiveConfig_NoConfigMemberDropsRootTasks fences the fan-out
// broadcast bug for the no-miso.json path: a member with no config of its own
// must not inherit root's task list either, else a root-scope concurrent
// broadcasts to every fan-out member regardless of whether it has a miso.json.
func TestEffectiveConfig_NoConfigMemberDropsRootTasks(t *testing.T) {
	base := config.Config{
		Shell:   "bash",
		Scripts: "./scripts",
		Tasks:   map[string]config.TaskConfig{"dev": {Concurrent: []string{"services"}}},
	}
	member := Member{Name: "web", Dir: t.TempDir(), ConfigPath: ""}

	got := EffectiveConfig(base, member)
	if got.Tasks != nil {
		t.Errorf("Tasks = %v, want nil (no-config member must not inherit root's)", got.Tasks)
	}
	if got.Shell != "bash" || got.Scripts != "./scripts" {
		t.Errorf("Shell/Scripts = %q/%q, want unchanged root (bash/./scripts)", got.Shell, got.Scripts)
	}
}

// TestEffectiveConfig_MalformedMemberConfigDropsRootTasks fences the fan-out
// broadcast bug for the parse-error path: a member whose miso.json fails to
// parse must not inherit root's task list either, else a root-scope
// concurrent broadcasts to every fan-out member regardless of whether its
// own config is readable.
func TestEffectiveConfig_MalformedMemberConfigDropsRootTasks(t *testing.T) {
	root := t.TempDir()
	memberDir := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(memberDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memberDir, "miso.json"),
		[]byte(`{not valid json`), 0o644); err != nil {
		t.Fatal(err)
	}
	base := config.Config{
		Shell:   "bash",
		Scripts: "./scripts",
		Tasks:   map[string]config.TaskConfig{"dev": {Concurrent: []string{"services"}}},
	}
	member := Member{Name: "web", Dir: memberDir, ConfigPath: filepath.Join(memberDir, "miso.json")}

	got := EffectiveConfig(base, member)
	if got.Tasks != nil {
		t.Errorf("Tasks = %v, want nil (malformed member config must not inherit root's)", got.Tasks)
	}
	if got.Shell != "bash" || got.Scripts != "./scripts" {
		t.Errorf("Shell/Scripts = %q/%q, want unchanged root (bash/./scripts)", got.Shell, got.Scripts)
	}
}

// TestEffectiveConfig_MemberWithoutTasksDropsRootTasks fences the fan-out
// broadcast bug: a member declaring no tasks of its own must not inherit root's
// task list, else a root-scope concurrent fans out to every member.
func TestEffectiveConfig_MemberWithoutTasksDropsRootTasks(t *testing.T) {
	root := t.TempDir()
	memberDir := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(memberDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memberDir, "miso.json"),
		[]byte(`{"shell":"zsh"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	base := config.Config{
		Shell:   "bash",
		Scripts: "./scripts",
		Tasks:   map[string]config.TaskConfig{"dev": {Concurrent: []string{"services"}}},
	}
	member := Member{Name: "web", Dir: memberDir, ConfigPath: filepath.Join(memberDir, "miso.json")}

	got := EffectiveConfig(base, member)
	if got.Tasks != nil {
		t.Errorf("Tasks = %v, want nil (member owns none; must not inherit root's)", got.Tasks)
	}
}

// TestEffectiveConfig_MemberTasksReplaceRoot verifies a member's own tasks fully
// replace root's (member concurrent is member-owned), not merge with them.
func TestEffectiveConfig_MemberTasksReplaceRoot(t *testing.T) {
	root := t.TempDir()
	memberDir := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(memberDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memberDir, "miso.json"),
		[]byte(`{"repo":{"tasks":{"dev":{"concurrent":["convex"]}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	base := config.Config{
		Shell:   "bash",
		Scripts: "./scripts",
		Tasks:   map[string]config.TaskConfig{"dev": {Concurrent: []string{"services"}}},
	}
	member := Member{Name: "web", Dir: memberDir, ConfigPath: filepath.Join(memberDir, "miso.json")}

	got := EffectiveConfig(base, member)
	if conc := got.TaskConcurrent("dev"); len(conc) != 1 || conc[0] != "convex" {
		t.Errorf("TaskConcurrent(dev) = %v, want [convex] (member owns tasks)", conc)
	}
}
