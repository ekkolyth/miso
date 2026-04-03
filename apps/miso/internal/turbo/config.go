package turbo

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type TurboTask struct {
	DependsOn  []string `json:"dependsOn"`
	Cache      *bool    `json:"cache"`
	Persistent bool     `json:"persistent"`
	Outputs    []string `json:"outputs"`
	Env        []string `json:"env"`
}

// TurboConfig holds the parsed configuration from turbo.json.
// Version 1 uses the "pipeline" key; version 2 uses the "tasks" key.
type TurboConfig struct {
	Version int
	Tasks   map[string]TurboTask
}

// sorted
func (c TurboConfig) TaskNames() []string {
	names := make([]string, 0, len(c.Tasks))
	for name := range c.Tasks {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// rawTurboConfig is used for JSON unmarshalling to detect v1 vs v2 format.
type rawTurboConfig struct {
	Tasks    map[string]TurboTask `json:"tasks"`
	Pipeline map[string]TurboTask `json:"pipeline"`
}

// Behaviour:
//   - File not found → empty TurboConfig with initialized Tasks map, nil error.
//   - Malformed JSON → error.
//   - Neither "tasks" nor "pipeline" key present → error.
//   - "tasks" key present → Version 2.
//   - "pipeline" key present → Version 1.
func LoadConfig(root string) (TurboConfig, error) {
	path := filepath.Join(root, "turbo.json")

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return TurboConfig{Tasks: make(map[string]TurboTask)}, nil
		}
		return TurboConfig{}, fmt.Errorf("reading turbo.json: %w", err)
	}

	var raw rawTurboConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return TurboConfig{}, fmt.Errorf("parsing turbo.json: %w", err)
	}

	if raw.Tasks != nil {
		return TurboConfig{
			Version: 2,
			Tasks:   raw.Tasks,
		}, nil
	}

	if raw.Pipeline != nil {
		return TurboConfig{
			Version: 1,
			Tasks:   raw.Pipeline,
		}, nil
	}

	return TurboConfig{}, fmt.Errorf("turbo.json: neither \"tasks\" nor \"pipeline\" key found")
}
