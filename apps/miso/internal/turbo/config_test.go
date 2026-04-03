package turbo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigV2(t *testing.T) {
	dir := t.TempDir()
	content := `{
		"$schema": "https://turbo.build/schema.json",
		"tasks": {
			"build": {
				"dependsOn": ["^build"],
				"outputs": ["dist/**"],
				"cache": true
			},
			"test": {
				"dependsOn": ["build"],
				"cache": false
			},
			"dev": {
				"persistent": true,
				"cache": false
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(dir, "turbo.json"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write turbo.json: %v", err)
	}

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}

	if cfg.Version != 2 {
		t.Errorf("Version = %d, want 2", cfg.Version)
	}

	if len(cfg.Tasks) != 3 {
		t.Errorf("len(Tasks) = %d, want 3", len(cfg.Tasks))
	}

	build, ok := cfg.Tasks["build"]
	if !ok {
		t.Fatal("Tasks[\"build\"] not found")
	}
	if len(build.DependsOn) != 1 || build.DependsOn[0] != "^build" {
		t.Errorf("build.DependsOn = %v, want [^build]", build.DependsOn)
	}
	if len(build.Outputs) != 1 || build.Outputs[0] != "dist/**" {
		t.Errorf("build.Outputs = %v, want [dist/**]", build.Outputs)
	}
	if build.Cache == nil || *build.Cache != true {
		t.Errorf("build.Cache = %v, want true", build.Cache)
	}

	testTask, ok := cfg.Tasks["test"]
	if !ok {
		t.Fatal("Tasks[\"test\"] not found")
	}
	if len(testTask.DependsOn) != 1 || testTask.DependsOn[0] != "build" {
		t.Errorf("test.DependsOn = %v, want [build]", testTask.DependsOn)
	}
	if testTask.Cache == nil || *testTask.Cache != false {
		t.Errorf("test.Cache = %v, want false", testTask.Cache)
	}

	dev, ok := cfg.Tasks["dev"]
	if !ok {
		t.Fatal("Tasks[\"dev\"] not found")
	}
	if !dev.Persistent {
		t.Errorf("dev.Persistent = false, want true")
	}

	names := cfg.TaskNames()
	if len(names) != 3 {
		t.Fatalf("TaskNames() len = %d, want 3", len(names))
	}
	expected := []string{"build", "dev", "test"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("TaskNames()[%d] = %q, want %q", i, name, expected[i])
		}
	}
}

func TestLoadConfigV1(t *testing.T) {
	dir := t.TempDir()
	content := `{
		"$schema": "https://turbo.build/schema.v1.json",
		"pipeline": {
			"build": {
				"dependsOn": ["^build"],
				"outputs": ["dist/**"]
			},
			"test": {
				"dependsOn": ["build"]
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(dir, "turbo.json"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write turbo.json: %v", err)
	}

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}

	if cfg.Version != 1 {
		t.Errorf("Version = %d, want 1", cfg.Version)
	}

	if len(cfg.Tasks) != 2 {
		t.Errorf("len(Tasks) = %d, want 2", len(cfg.Tasks))
	}

	build, ok := cfg.Tasks["build"]
	if !ok {
		t.Fatal("Tasks[\"build\"] not found")
	}
	if len(build.DependsOn) != 1 || build.DependsOn[0] != "^build" {
		t.Errorf("build.DependsOn = %v, want [^build]", build.DependsOn)
	}
}

func TestLoadConfigNotFound(t *testing.T) {
	dir := t.TempDir()

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}

	if cfg.Version != 0 {
		t.Errorf("Version = %d, want 0", cfg.Version)
	}

	if cfg.Tasks == nil {
		t.Error("Tasks = nil, want initialized map")
	}

	if len(cfg.Tasks) != 0 {
		t.Errorf("len(Tasks) = %d, want 0", len(cfg.Tasks))
	}
}

func TestLoadConfigMalformed(t *testing.T) {
	dir := t.TempDir()
	content := `{ this is not valid json }`
	if err := os.WriteFile(filepath.Join(dir, "turbo.json"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write turbo.json: %v", err)
	}

	_, err := LoadConfig(dir)
	if err == nil {
		t.Error("LoadConfig() error = nil, want error for malformed JSON")
	}
}

func TestLoadConfigNoTasksOrPipeline(t *testing.T) {
	dir := t.TempDir()
	content := `{
		"$schema": "https://turbo.build/schema.json",
		"globalEnv": ["NODE_ENV"]
	}`
	if err := os.WriteFile(filepath.Join(dir, "turbo.json"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write turbo.json: %v", err)
	}

	_, err := LoadConfig(dir)
	if err == nil {
		t.Error("LoadConfig() error = nil, want error when neither tasks nor pipeline key present")
	}
}
