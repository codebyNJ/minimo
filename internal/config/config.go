package config

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DebounceMS       int               `yaml:"debounce_ms"`
	PollIntervalSec  int               `yaml:"poll_interval_seconds"`
	EnabledProviders []string          `yaml:"enabled_providers"`
	ProviderPaths    map[string]string `yaml:"provider_paths"`
}

func Default() Config {
	return Config{
		DebounceMS:      500,
		PollIntervalSec: 2,
	}
}

func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return filepath.Join(home, ".ctx", "config.yaml")
}

func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c Config) Debounce() time.Duration {
	return time.Duration(c.DebounceMS) * time.Millisecond
}

func (c Config) PollInterval() time.Duration {
	return time.Duration(c.PollIntervalSec) * time.Second
}
