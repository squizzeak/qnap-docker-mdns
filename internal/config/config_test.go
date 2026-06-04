package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.DomainSuffix != "local" {
		t.Errorf("expected local, got %s", cfg.DomainSuffix)
	}
	if cfg.ReverseProxy.ApacheListenPort != 80 {
		t.Errorf("expected 80, got %d", cfg.ReverseProxy.ApacheListenPort)
	}
	if cfg.Backups.MaxBackups != 100 {
		t.Errorf("expected 100, got %d", cfg.Backups.MaxBackups)
	}
	if cfg.Reconcile.Debounce.Duration != 500*time.Millisecond {
		t.Errorf("expected 500ms, got %v", cfg.Reconcile.Debounce.Duration)
	}
	if cfg.ProbeTimeout.Duration != 2*time.Second {
		t.Errorf("expected 2s, got %v", cfg.ProbeTimeout.Duration)
	}
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	content := []byte("domain_suffix: lan\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DomainSuffix != "lan" {
		t.Errorf("expected lan, got %s", cfg.DomainSuffix)
	}
}

func TestLoadFileMissing(t *testing.T) {
	_, err := LoadFile("/nonexistent/path.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestMerge(t *testing.T) {
	base := DefaultConfig()
	override := &Config{DomainSuffix: "lan"}
	result := Merge(base, override)
	if result.DomainSuffix != "lan" {
		t.Errorf("expected lan, got %s", result.DomainSuffix)
	}
	if result.ReverseProxy.ApacheListenPort != 80 {
		t.Errorf("expected default 80, got %d", result.ReverseProxy.ApacheListenPort)
	}
}

func TestMergeNilOverride(t *testing.T) {
	base := DefaultConfig()
	result := Merge(base, nil)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestLoadMerged(t *testing.T) {
	dir := t.TempDir()
	defPath := filepath.Join(dir, "default.yaml")
	overPath := filepath.Join(dir, "override.yaml")

	if err := os.WriteFile(defPath, []byte("domain_suffix: lan\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overPath, []byte("domain_suffix: home\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadMerged(defPath, overPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DomainSuffix != "home" {
		t.Errorf("expected home (override), got %s", cfg.DomainSuffix)
	}
}

func TestLoadMergedMissingOverride(t *testing.T) {
	dir := t.TempDir()
	defPath := filepath.Join(dir, "default.yaml")

	if err := os.WriteFile(defPath, []byte("domain_suffix: lan\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadMerged(defPath, "/nonexistent/override.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DomainSuffix != "lan" {
		t.Errorf("expected lan, got %s", cfg.DomainSuffix)
	}
}

func TestDurationUnmarshal(t *testing.T) {
	var d Duration
	if err := d.UnmarshalYAML(func(v interface{}) error {
		*v.(*string) = "3s"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if d.Duration != 3*time.Second {
		t.Errorf("expected 3s, got %v", d.Duration)
	}
}

func TestDurationUnmarshalInvalid(t *testing.T) {
	var d Duration
	err := d.UnmarshalYAML(func(v interface{}) error {
		*v.(*string) = "not-a-duration"
		return nil
	})
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
}
