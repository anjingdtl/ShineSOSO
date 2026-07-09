// Package logging configures a rotating slog.Logger that writes JSON to disk
// and a human-friendly stream to stderr.
//
// Privacy: callers MUST scrub magnets, raw keywords, and user paths from
// attribute values. The logger does not enforce this; it only formats.
package logging

import (
    "fmt"
    "io"
    "log/slog"
    "os"
    "path/filepath"

    "gopkg.in/natefinch/lumberjack.v2"
)

// Rotation defaults (spec §25.2).
const (
    MaxSizeMB  = 10
    MaxBackups = 5
    MaxAgeDays = 30
)

// Logger is a slog.Logger that owns a closable file handle. Callers MUST
// call Close on shutdown to release the file handle; on Windows, leaving
// the handle open prevents log rotation and TempDir cleanup.
type Logger struct {
    *slog.Logger
    file io.Closer
}

// Close releases the underlying log file handle. Safe to call multiple
// times (subsequent calls return nil). The Logger remains usable after
// Close, but no further records will reach the file.
func (l *Logger) Close() error {
    if l.file == nil {
        return nil
    }
    err := l.file.Close()
    l.file = nil
    return err
}

// New returns a Logger that writes JSON to filePath (rotated by lumberjack)
// and mirrors to stderr. It also ensures the parent directory exists.
func New(level slog.Level, filePath string) (*Logger, error) {
    if filePath == "" {
        return nil, fmt.Errorf("logging: empty filePath")
    }
    if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
        return nil, fmt.Errorf("create log dir: %w", err)
    }
    w := &lumberjack.Logger{
        Filename:   filePath,
        MaxSize:    MaxSizeMB,
        MaxBackups: MaxBackups,
        MaxAge:     MaxAgeDays,
        Compress:   true,
    }
    handler := slog.NewJSONHandler(io.MultiWriter(w, os.Stderr), &slog.HandlerOptions{
        Level: level,
    })
    return &Logger{
        Logger: slog.New(handler),
        file:   w,
    }, nil
}

// ParseLevel maps a string to a slog.Level. Unknown values fall back to
// info-level; the caller may override via flag/env.
func ParseLevel(s string) slog.Level {
    switch s {
    case "debug":
        return slog.LevelDebug
    case "warn", "warning":
        return slog.LevelWarn
    case "error":
        return slog.LevelError
    default:
        return slog.LevelInfo
    }
}
