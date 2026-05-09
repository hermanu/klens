package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Kubeconfig string `yaml:"kubeconfig"`
	Accent     string `yaml:"accent"`
	Namespace  string `yaml:"namespace"`
}

func defaults() Config {
	return Config{
		Accent:    "#e85a4f",
		Namespace: "",
	}
}

func Load(path string) (Config, error) {
	cfg := defaults()
	if path == "" {
		home, _ := os.UserHomeDir()
		path = home + "/.klens/config.yaml"
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	return cfg, yaml.Unmarshal(data, &cfg)
}
