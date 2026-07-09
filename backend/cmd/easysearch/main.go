package main

import (
    "context"
    "errors"
    "fmt"
    "log/slog"
    "net"
    "net/http"
    "os"
    "os/signal"
    "path/filepath"
    "syscall"
    "time"

    "github.com/local/easysearch/backend/internal/api"
    "github.com/local/easysearch/backend/internal/config"
    "github.com/local/easysearch/backend/internal/logging"
)

const version = "0.1.0"

func main() {
    args := os.Args[1:]
    if len(args) > 0 && args[0] == "--version" {
        fmt.Println(version)
        return
    }
    cfg := config.Default()
    // Smoke / scripted runs may force a port; production uses 0 (random).
    for i, a := range args {
        if a == "--port" && i+1 < len(args) {
            var p int
            if _, err := fmt.Sscanf(args[i+1], "%d", &p); err == nil {
                cfg.ListenPort = p
            }
        }
        if a == "--no-browser" {
            cfg.OpenBrowser = false
        }
    }
    logPath := filepath.Join(cfg.DataDir, "logs", "easysearch.log")
    logger, err := logging.New(logging.ParseLevel(cfg.LogLevel), logPath)
    if err != nil {
        fmt.Fprintf(os.Stderr, "init logger: %v\n", err)
        os.Exit(1)
    }
    defer func() { _ = logger.Close() }()
    slog.SetDefault(logger.Logger)

    startTime := time.Now()
    logger.Info("easysearch boot",
        "data_dir", cfg.DataDir,
        "bind_host", cfg.BindHost,
        "version", version,
    )

    router := api.NewRouter(api.ServerDeps{
        Logger: logger.Logger,
        System: &api.SystemHandler{
            StartTime: startTime,
            Version:   version,
            Logger:    logger.Logger,
        },
    })

    addr := fmt.Sprintf("%s:%d", cfg.BindHost, cfg.ListenPort)
    listener, err := net.Listen("tcp", addr)
    if err != nil {
        logger.Error("listen failed", "addr", addr, "err", err)
        os.Exit(1)
    }
    actualPort := listener.Addr().(*net.TCPAddr).Port
    logger.Info("http server listening", "addr", listener.Addr().String())

    server := &http.Server{
        Handler:           router,
        ReadHeaderTimeout: 5 * time.Second,
        // WriteTimeout is intentionally generous; SSE streams in Phase 2+
        // // can hold the connection open for the full search duration.
        WriteTimeout: 0,
        IdleTimeout:  60 * time.Second,
    }

    // Graceful shutdown on Ctrl+C / SIGTERM.
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    errCh := make(chan error, 1)
    go func() {
        if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
            errCh <- err
        }
        close(errCh)
    }()

    fmt.Printf("easysearch listening on http://%s (PID %d)\n", listener.Addr(), os.Getpid())
    fmt.Printf("press Ctrl+C to stop\n")

    select {
    case <-ctx.Done():
        logger.Info("shutdown signal received")
    case err := <-errCh:
        if err != nil {
            logger.Error("server error", "err", err)
            os.Exit(1)
        }
    }

    shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := server.Shutdown(shutdownCtx); err != nil {
        logger.Error("graceful shutdown failed", "err", err)
        os.Exit(1)
    }
    logger.Info("easysearch stopped", "port", actualPort)
}
