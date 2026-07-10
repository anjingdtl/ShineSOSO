// Package config exposes the resolved runtime configuration for the EasySearch binary.
//
// Defaults are tuned for a Windows desktop launch (single-user, 127.0.0.1,
// APPDATA-based data dir). All values can be overridden via environment
// variables or command-line flags; see ResolveFromEnv and cmd/easysearch.
package config

import (
    "os"
    "path/filepath"
)

// Config holds the resolved runtime configuration. Values are immutable after
// construction; copy the struct to derive variations.
type Config struct {
    // DataDir is the root directory for the SQLite database, logs, cache, and
    // per-user data. Created on first run if missing.
    DataDir string

    // BindHost is the address the HTTP server binds to. Must remain 127.0.0.1
    // (or equivalent loopback) for the personal-local-use threat model.
    BindHost string

    // ListenPort is the TCP port for the HTTP server. 0 picks a random free
    // port, which is the default and the only acceptable value for shipping.
    ListenPort int

    // LogLevel is one of debug, info, warn, error (case-insensitive).
    LogLevel string

    // OpenBrowser controls whether the launcher opens the user's default
    // browser to the WebUI on startup.
    OpenBrowser bool

    // DevMode enables Vite dev-server friendly behaviors: relax embed fallback,
    // allow CORS, skip auto-browser launch.
    DevMode bool

    // CatalogManifestURL is the URL the catalog updater polls on demand.
    // Empty disables online updates — the bundled embedded catalog is used.
    CatalogManifestURL string

    // CatalogSignaturePubKeyB64 is the base64-encoded Ed25519 public key
    // (32 bytes raw, 44 base64 chars) used to verify remote catalog
    // manifests. Empty disables signature verification — the SHA-256
    // gate alone is then the only integrity check.
    CatalogSignaturePubKeyB64 string
}

// Default returns the production config with environment overrides applied.
// See package doc for the precedence order.
func Default() Config {
    return Config{
        DataDir:           resolveDataDir(),
        BindHost:          "127.0.0.1",
        ListenPort:        0,
        LogLevel:          "info",
        OpenBrowser:       true,
        DevMode:           false,
        CatalogManifestURL: os.Getenv("EASYSEARCH_CATALOG_URL"),
        CatalogSignaturePubKeyB64: os.Getenv("EASYSEARCH_CATALOG_PUBKEY"),
    }
}

func resolveDataDir() string {
    if v := os.Getenv("EASYSEARCH_DATA_DIR"); v != "" {
        return v
    }
    if appdata := os.Getenv("APPDATA"); appdata != "" {
        return filepath.Join(appdata, "EasySearch", "data")
    }
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".easysearch", "data")
}
