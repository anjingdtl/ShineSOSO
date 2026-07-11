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
	"github.com/local/easysearch/backend/internal/catalog"
	"github.com/local/easysearch/backend/internal/config"
	"github.com/local/easysearch/backend/internal/indexer"
	"github.com/local/easysearch/backend/internal/launcher"
	"github.com/local/easysearch/backend/internal/logging"
	"github.com/local/easysearch/backend/internal/prowlarr"
	"github.com/local/easysearch/backend/internal/store"
)

const version = "0.4.0"

func main() {
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "--version" {
		fmt.Println(version)
		return
	}
	cfg := config.Default()
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

	// Phase 4: open SQLite, build the indexer repo, populate the catalog
	// with built-in definitions, and refresh.
	dbPath := filepath.Join(cfg.DataDir, "easysearch.db")
	st, err := store.Open(dbPath)
	if err != nil {
		logger.Error("open store failed", "err", err)
		os.Exit(1)
	}
	defer func() { _ = st.Close() }()

	repo := store.NewIndexerRepo(st)
	importedRepo := store.NewImportedDefinitionRepo(st)
	cat := catalog.New(repo)
	// Phase 6: register embedded catalog (manifest.json + definitions/*.yml)
	// instead of the Phase 4 hardcoded list. The hardcoded mock definitions
	// remain available as a fallback so older demo flows keep working.
	for _, d := range catalog.BuiltinDefinitions() {
		cat.RegisterDefinition(d)
	}
	if m, err := catalog.LoadBuiltin(); err != nil {
		logger.Warn("load builtin catalog", "err", err)
	} else {
		for _, d := range m {
			cat.RegisterDefinition(d)
		}
	}
	if err := cat.Refresh(); err != nil {
		logger.Warn("catalog refresh on boot", "err", err)
	}
	httpClient := indexer.NewClient()
	// The release package places Prowlarr next to EasySearch. Its data and
	// generated API key stay in EasySearch's own per-user data directory.
	self, selfErr := os.Executable()
	if selfErr != nil {
		logger.Warn("resolve executable path for Prowlarr runtime", "err", selfErr)
	}
	prowlarrManager := prowlarr.NewManager(prowlarr.Config{
		Executable: filepath.Join(filepath.Dir(self), "runtime", "prowlarr", "Prowlarr.exe"),
		DataDir:    filepath.Join(cfg.DataDir, "prowlarr"),
	})
	prowlarrManager.StartAsync(context.Background())
	defer prowlarrManager.Close()

	// Phase 6: catalog updater — boots with embedded catalog active, can
	// pull a remote manifest when the user hits POST /api/v1/indexer-catalog/update.
	updater := catalog.NewUpdater(cat, catalog.UpdaterConfig{
		ManifestURL: cfg.CatalogManifestURL,
		CacheDir:    filepath.Join(cfg.DataDir, "catalog-cache"),
		Logger:      logger.Logger,
		OnDefinitionActivated: func(definitionID, newVersion string) error {
			_, err := repo.BumpDefinitionVersion(definitionID, newVersion)
			return err
		},
	})
	if _, err := updater.ActivateEmbedded(); err != nil {
		logger.Warn("activate embedded catalog", "err", err)
	}
	logger.Info("catalog updater ready", "manifest_url", cfg.CatalogManifestURL, "active_version", updater.ActiveVersion())

	router := api.NewRouter(api.ServerDeps{
		Logger: logger.Logger,
		System: &api.SystemHandler{
			StartTime: startTime,
			Version:   version,
			Logger:    logger.Logger,
		},
		Search: api.NewSearchHandler(logger.Logger, cat, httpClient).WithProwlarr(prowlarrManager),
		Indexer: &api.IndexerHandler{
			Logger:     logger.Logger,
			Catalog:    cat,
			Repo:       repo,
			HTTPClient: httpClient,
		},
		Import: &api.ImportHandler{
			Logger:     logger.Logger,
			Repo:       importedRepo,
			Catalog:    cat,
			HTTPClient: httpClient,
		},
		Catalog: &api.CatalogUpdateHandler{
			Logger:  logger.Logger,
			Updater: updater,
		},
		Discovery: &api.DiscoveryHandler{Logger: logger.Logger, IndexerClient: httpClient},
		Prowlarr:  &api.ProwlarrHandler{Logger: logger.Logger, Manager: prowlarrManager},
		Diagnostics: &api.DiagnosticsHandler{
			StartTime: startTime,
			Version:   version,
			DataDir:   cfg.DataDir,
			LogDir:    api.DefaultLogDir(cfg.DataDir),
			DBPath:    dbPath,
			Store:     st,
			Repo:      repo,
			Catalog:   updater,
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
	if _, err := launcher.WritePortFile(cfg.DataDir, actualPort); err != nil {
		logger.Warn("write port file failed", "err", err)
	} else {
		logger.Info("port file written", "port", actualPort, "data_dir", cfg.DataDir)
	}
	logger.Info("http server listening", "addr", listener.Addr().String())

	server := &http.Server{
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      0, // SSE streams must not be cut off
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	url := fmt.Sprintf("http://%s", listener.Addr().String())
	fmt.Printf("easysearch listening on %s (PID %d)\n", url, os.Getpid())
	if cfg.OpenBrowser {
		if err := launcher.OpenURL(url); err != nil {
			logger.Warn("open browser failed", "err", err, "url", url)
			fmt.Println("please open the URL above in your browser")
		} else {
			logger.Info("browser launched", "url", url)
		}
	} else {
		fmt.Println("press Ctrl+C to stop")
	}

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		if err != nil {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}

	if err := launcher.RemovePortFile(cfg.DataDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.Warn("remove port file failed", "err", err)
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	logger.Info("easysearch stopped", "port", actualPort)
}
