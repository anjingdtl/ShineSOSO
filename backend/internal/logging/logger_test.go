package logging

import (
    "log/slog"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestNewLoggerWritesInfo(t *testing.T) {
    dir := t.TempDir()
    logPath := filepath.Join(dir, "test.log")
    logger, err := New(slog.LevelInfo, logPath)
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { _ = logger.Close() })

    logger.Info("hello", "k", "v")
    logger.Warn("careful")

    b, err := os.ReadFile(logPath)
    if err != nil {
        t.Fatal(err)
    }
    s := string(b)
    if !strings.Contains(s, "hello") {
        t.Fatalf("expected log line containing 'hello', got %q", s)
    }
    if !strings.Contains(s, "careful") {
        t.Fatalf("expected log line containing 'careful', got %q", s)
    }
    if strings.Contains(s, "debug-only") {
        t.Fatalf("info level should not include debug records")
    }
}

func TestNewLoggerCreatesParentDir(t *testing.T) {
    dir := t.TempDir()
    logPath := filepath.Join(dir, "nested", "deeper", "test.log")
    if _, err := os.Stat(filepath.Dir(logPath)); !os.IsNotExist(err) {
        t.Fatalf("precondition: nested dir should not exist yet")
    }
    logger, err := New(slog.LevelInfo, logPath)
    if err != nil {
        t.Fatalf("New should create nested dirs, got: %v", err)
    }
    t.Cleanup(func() { _ = logger.Close() })
    logger.Info("nested ok")
    if _, err := os.Stat(logPath); err != nil {
        t.Fatalf("expected log file to exist, got: %v", err)
    }
}

func TestLoggerCloseIdempotent(t *testing.T) {
    dir := t.TempDir()
    logPath := filepath.Join(dir, "test.log")
    logger, err := New(slog.LevelInfo, logPath)
    if err != nil {
        t.Fatal(err)
    }
    if err := logger.Close(); err != nil {
        t.Fatalf("first Close: %v", err)
    }
    if err := logger.Close(); err != nil {
        t.Fatalf("second Close should be nil, got %v", err)
    }
}

func TestNewLoggerRejectsEmptyPath(t *testing.T) {
    if _, err := New(slog.LevelInfo, ""); err == nil {
        t.Fatal("expected error for empty path")
    }
}

func TestParseLevel(t *testing.T) {
    cases := map[string]slog.Level{
        "debug": slog.LevelDebug,
        "info":  slog.LevelInfo,
        "warn":  slog.LevelWarn,
        "error": slog.LevelError,
        "weird": slog.LevelInfo, // default fallback
    }
    for in, want := range cases {
        if got := ParseLevel(in); got != want {
            t.Errorf("ParseLevel(%q) = %v, want %v", in, got, want)
        }
    }
}
