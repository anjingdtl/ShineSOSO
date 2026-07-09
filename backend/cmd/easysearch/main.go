package main

import (
    "fmt"
    "log/slog"
    "os"
    "path/filepath"

    "github.com/local/easysearch/backend/internal/config"
    "github.com/local/easysearch/backend/internal/logging"
)

const version = "0.1.0"

func main() {
    if len(os.Args) > 1 && os.Args[1] == "--version" {
        fmt.Println(version)
        return
    }

    cfg := config.Default()
    logPath := filepath.Join(cfg.DataDir, "logs", "easysearch.log")
    logger, err := logging.New(logging.ParseLevel(cfg.LogLevel), logPath)
    if err != nil {
        fmt.Fprintf(os.Stderr, "init logger: %v\n", err)
        os.Exit(1)
    }
    defer func() { _ = logger.Close() }()
    slog.SetDefault(logger.Logger)

    logger.Info("easysearch boot",
        "data_dir", cfg.DataDir,
        "bind_host", cfg.BindHost,
        "version", version,
    )
    fmt.Println("easysearch: skeleton boot ok (config + logger wired)")
}
