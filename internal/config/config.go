package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNotFound indicates that no miso.json file exists in the target directory.
var ErrNotFound = errors.New("config: not found")

// FileName is the default name for the miso config file.
const FileName = "miso.json"

// Config captures persisted metadata about the current project.
type Config struct {
	PackageManager string            `json:"package-manager"`
	ProjectName    string            `json:"project-name"`
	Scripts        map[string]string `json:"scripts"`
}

// EnsureDefaults makes sure optional maps are initialized.
func (c *Config) EnsureDefaults() {
	if c.Scripts == nil {
		c.Scripts = map[string]string{}
	}
}

// Path resolves the config file path for a given project root.
func Path(root string) string {
	return filepath.Join(root, FileName)
}

// Load reads miso.json from disk.
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

// Save persists miso.json to disk, ensuring parent directories exist.
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
