package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"docklite-agent/internal/api"
	"docklite-agent/internal/backup"
	"docklite-agent/internal/config"
	"docklite-agent/internal/docker"
	"docklite-agent/internal/handlers"
	"docklite-agent/internal/store"
)

func main() {
	cfg := config.Load()

	dockerClient, err := docker.NewClient(cfg.DockerSocketPath)
	if err != nil {
		log.Fatalf("failed to create docker client: %v", err)
	}
	defer dockerClient.Close()

	sqliteStore, err := store.NewSQLiteStore(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer sqliteStore.Close()

	// Initialize tables needed by the agent
	if err := sqliteStore.InitializeAgentTables(); err != nil {
		log.Fatalf("failed to initialize agent tables: %v", err)
	}

	if err := handlers.EnsureBootstrapToken(sqliteStore, cfg.Token); err != nil {
		log.Fatalf("failed to ensure bootstrap token: %v", err)
	}

	handlers := handlers.New(dockerClient, sqliteStore, cfg.Token, cfg.BackupBaseDir, cfg.ListenAddr, cfg.NextjsURL)
	router := api.NewRouter(handlers, cfg.NextjsURL)

	backup.StartScheduler(sqliteStore, dockerClient, cfg.BackupBaseDir)

	// Pull site images in the background so they are cached before the first
	// container creation request. This prevents proxy timeouts on first use.
	dockerClient.PrewarmImages(context.Background())

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()

	if cfg.NextjsURL != "" {
		log.Printf("docklite-agent listening on %s (proxying to Next.js at %s)", cfg.ListenAddr, cfg.NextjsURL)
	} else {
		log.Printf("docklite-agent listening on %s (headless mode, no Next.js proxy)", cfg.ListenAddr)
	}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
