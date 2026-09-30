// Package config reads and writes the app's YAML settings file, which lives
// in the OS per-user config directory so it is found no matter where the
// binary is launched from.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

const (
	appDirName     = "cents"
	configFileName = "config.yml"
	dbFileName     = "cents.db"
)

// ErrNotFound is returned by Load when there is no config file yet.
var ErrNotFound = errors.New("config file not found")

type Config struct {
	DBPath string `yaml:"db_path"`
}

// Dir is the app's directory inside the OS config directory, e.g.
// ~/Library/Application Support/cents on macOS or ~/.config/cents on Linux.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to locate user config directory: %w", err)
	}

	return filepath.Join(base, appDirName), nil
}

func DefaultPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, configFileName), nil
}

// DefaultDBPath is where a new database is suggested when the user has not
// picked one yet.
func DefaultDBPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, dbFileName), nil
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, ErrNotFound
	}

	if err != nil {
		return Config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	return cfg, nil
}

func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
