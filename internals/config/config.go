// Package config provides local configuration paths and settings for mgr.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

const (
	AppName           = "mgr"
	ConfigFileName    = "config.yaml"
	InventoryFileName = "inventory.yaml"
	STOREFILENAME     = ".mgr.json"
)

// Home is kept for older storage code that still references the previous
// config package surface.
var Home string

// FoostashConfig contains the values needed to construct the read-only
// Foostash SDK client.
type FoostashConfig struct {
	ServerURL    string `yaml:"server_url"`
	Project      string `yaml:"project"`
	Env          string `yaml:"env"`
	SSHKeyPath   string `yaml:"ssh_key_path"`
	MasterKey    string `yaml:"master_key,omitempty"`
	MasterKeyEnv string `yaml:"master_key_env,omitempty"`
}

// Config is the on-disk mgr configuration.
type Config struct {
	Version  int            `yaml:"version"`
	Foostash FoostashConfig `yaml:"foostash"`
}

// Paths groups the local files mgr owns.
type Paths struct {
	ConfigDir     string
	ConfigPath    string
	InventoryPath string
}

// DefaultPaths returns the OS-specific config paths for mgr.
func DefaultPaths() (Paths, error) {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return Paths{}, errors.Join(err, homeErr)
		}
		dir = filepath.Join(home, ".config")
	}
	cfgDir := filepath.Join(dir, AppName)
	return Paths{
		ConfigDir:     cfgDir,
		ConfigPath:    filepath.Join(cfgDir, ConfigFileName),
		InventoryPath: filepath.Join(cfgDir, InventoryFileName),
	}, nil
}

// DefaultConfig returns a valid empty configuration.
func DefaultConfig() Config {
	return Config{
		Version: 1,
		Foostash: FoostashConfig{
			Env:          "dev",
			MasterKeyEnv: "FOOSTASH_MASTER_KEY",
		},
	}
}

// Load reads config.yaml. Missing files are treated as an empty default config.
func Load(path string) (Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Foostash.MasterKeyEnv == "" {
		cfg.Foostash.MasterKeyEnv = "FOOSTASH_MASTER_KEY"
	}
	return cfg, nil
}

// Save writes config.yaml with user-only permissions.
func Save(path string, cfg Config) error {
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Foostash.MasterKeyEnv == "" {
		cfg.Foostash.MasterKeyEnv = "FOOSTASH_MASTER_KEY"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// InitConfig initializes compatibility globals for older packages.
func InitConfig() {
	home, err := os.UserHomeDir()
	if err == nil {
		Home = home
	}
}
