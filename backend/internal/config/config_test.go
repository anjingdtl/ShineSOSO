package config

import (
    "os"
    "path/filepath"
    "testing"
)

func TestDefaultConfig(t *testing.T) {
    cfg := Default()
    if cfg.BindHost != "127.0.0.1" {
        t.Fatalf("BindHost want 127.0.0.1, got %q", cfg.BindHost)
    }
    if cfg.LogLevel != "info" {
        t.Fatalf("LogLevel want info, got %q", cfg.LogLevel)
    }
    if !cfg.OpenBrowser {
        t.Fatal("OpenBrowser should default true")
    }
}

func TestResolveDataDirFromEnvOverride(t *testing.T) {
    custom := filepath.Join(os.TempDir(), "EasySearchEnvTest")
    t.Setenv("EASYSEARCH_DATA_DIR", custom)
    cfg := Default()
    if cfg.DataDir != custom {
        t.Fatalf("DataDir want %q, got %q", custom, cfg.DataDir)
    }
}

func TestResolveDataDirAbsolute(t *testing.T) {
    cfg := Default()
    if !filepath.IsAbs(cfg.DataDir) {
        t.Fatalf("DataDir should be absolute, got %q", cfg.DataDir)
    }
}

func TestResolveDataDirFallsBackToHome(t *testing.T) {
    t.Setenv("APPDATA", "")
    home, _ := os.UserHomeDir()
    cfg := Default()
    want := filepath.Join(home, ".easysearch", "data")
    if cfg.DataDir != want {
        t.Fatalf("DataDir want %q, got %q", want, cfg.DataDir)
    }
}
