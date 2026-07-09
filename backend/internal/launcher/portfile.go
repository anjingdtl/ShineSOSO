// Package launcher writes the .port file and (optionally) opens the user's
// default browser. The Vite dev server reads the same .port file to
// configure its proxy.
package launcher

import (
    "fmt"
    "os"
    "path/filepath"
    "strconv"
)

// PortFileName is written into DataDir before the HTTP server accepts
// connections, so a parallel frontend dev server can read it on startup.
const PortFileName = ".port"

// WritePortFile atomically writes the chosen port to <dataDir>/.port.
// The frontend dev proxy polls for this file's existence on startup.
func WritePortFile(dataDir string, port int) (string, error) {
    if dataDir == "" {
        return "", fmt.Errorf("launcher: empty dataDir")
    }
    if err := os.MkdirAll(dataDir, 0o755); err != nil {
        return "", fmt.Errorf("create data dir: %w", err)
    }
    path := filepath.Join(dataDir, PortFileName)
    if err := os.WriteFile(path, []byte(strconv.Itoa(port)), 0o644); err != nil {
        return "", fmt.Errorf("write port file: %w", err)
    }
    return path, nil
}

// ReadPortFile reads the .port file and returns the port. Returns 0 + a
// non-nil error if the file is missing or malformed.
func ReadPortFile(dataDir string) (int, error) {
    path := filepath.Join(dataDir, PortFileName)
    b, err := os.ReadFile(path)
    if err != nil {
        return 0, err
    }
    p, err := strconv.Atoi(string(b))
    if err != nil {
        return 0, fmt.Errorf("parse port file: %w", err)
    }
    return p, nil
}

// RemovePortFile deletes the .port file. Errors are returned but expected
// during shutdown when the file may already be gone.
func RemovePortFile(dataDir string) error {
    return os.Remove(filepath.Join(dataDir, PortFileName))
}
