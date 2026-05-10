package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds klens' persisted settings, read from ~/.klens/config.yaml.
type Config struct {
	Kubeconfig string `yaml:"kubeconfig"`
	Accent     string `yaml:"accent"`
	// Namespace is the last-opened scope (empty = all namespaces).
	Namespace string `yaml:"namespace"`
	// LastView is the resource view klens reopens to ("pods", "deployments",
	// "services", "secrets", "configmaps", "namespaces", "nodes", "pvcs").
	// Empty = pods (default landing).
	LastView string `yaml:"last_view"`
	// LogsSinceSeconds is the lookback window users last picked in the logs
	// view; carried across sessions so power users don't keep re-selecting.
	// 0 = use the built-in default (1800s = 30 min).
	LogsSinceSeconds int64 `yaml:"logs_since_seconds"`
}

func defaults() Config {
	return Config{
		Accent:    "#e85a4f",
		Namespace: "",
	}
}

// defaultPath resolves to ~/.klens/config.yaml.
func defaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".klens", "config.yaml")
}

// Load reads the config from path. An empty path defaults to ~/.klens/config.yaml.
// A missing file is not an error — the caller receives default values.
func Load(path string) (Config, error) {
	cfg := defaults()
	if path == "" {
		path = defaultPath()
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	err = yaml.Unmarshal(data, &cfg)
	return cfg, err
}

// Save writes the config to disk, creating ~/.klens/ if needed. Path "" uses
// the default path. Used to persist the last-opened namespace so klens
// re-opens to the user's most recent scope.
func Save(cfg Config, path string) error {
	if path == "" {
		path = defaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
