package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/manu/klens/config"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Accent != "#e85a4f" {
		t.Errorf("want accent #e85a4f, got %s", cfg.Accent)
	}
}

func TestLoad_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte("accent: \"#f0a830\"\n"), 0644)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Accent != "#f0a830" {
		t.Errorf("want accent #f0a830, got %s", cfg.Accent)
	}
}
