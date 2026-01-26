package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// return when miso.json not found
var ErrNotFound = errors.New("config: not found")

// default miso config filename
const FileName = "miso.json"

// persisted project metadata
type Config struct {
	PackageManager string            `json:"package-manager"`
	ProjectName    string            `json:"project-name"`
	Scripts        string            `json:"scripts"`
	Flags          map[string][]string `json:"flags,omitempty"`
}

// initialize optional maps
func (c *Config) EnsureDefaults() {
	if c.Scripts == "" {
		c.Scripts = "./scripts"
	}
	if c.Flags == nil {
		c.Flags = make(map[string][]string)
	}
}

// resolve config file path for project root
func Path(root string) string {
	return filepath.Join(root, FileName)
}

// read miso.json from disk
func Load(root string) (Config, error) {
	path := Path(root)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, ErrNotFound
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	cfg.EnsureDefaults()
	return cfg, nil
}

// save miso.json to disk, create parent dirs if needed
func Save(root string, cfg Config) error {
	cfg.EnsureDefaults()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	if err := os.WriteFile(Path(root), append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
