package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"docker-external-dns/internal/config"
	"docker-external-dns/internal/models"
	"docker-external-dns/internal/providers"
	"docker-external-dns/internal/providers/cloudflare"
	"docker-external-dns/internal/providers/pihole"
	"docker-external-dns/internal/sources"
	"docker-external-dns/internal/sources/docker"
	"docker-external-dns/internal/sources/traefik"
	"docker-external-dns/internal/syncer"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Initialize logging
	var logLevel slog.Level
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn", "warning":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)
	slog.Info("Loaded configuration", "provider", cfg.Provider, "source", cfg.Source, "interval", cfg.IntervalSeconds)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Source
	var src sources.Source
	switch cfg.Source {
	case "docker":
		src = docker.NewDockerSource(cfg.DockerHosts, cfg.Identifier)
	case "traefik":
		src = traefik.NewTraefikSource(cfg.TraefikConfigs)
	default:
		slog.Error("Unknown source", "source", cfg.Source)
		os.Exit(1)
	}

	if err := src.Initialize(ctx); err != nil {
		slog.Error("Failed to initialize source", "error", err)
		os.Exit(1)
	}

	// Initialize Provider
	var prov providers.Provider
	switch cfg.Provider {
	case "cloudflare":
		prov = cloudflare.NewCloudflareProvider(cfg.CFApiToken, cfg.Identifier)
	case "pihole":
		prov = pihole.NewPiholeProvider(cfg.PiholeURL, cfg.PiholeApiToken, cfg.PiholeApiVersion, cfg.PiholePassword, cfg.PiholeSkipVerify)
	default:
		slog.Error("Unknown provider", "provider", cfg.Provider)
		os.Exit(1)
	}

	if err := prov.Initialize(ctx); err != nil {
		slog.Error("Failed to initialize provider", "error", err)
		os.Exit(1)
	}

	var registryType models.RecordType
	var proxySupported bool
	if cfg.Provider == "cloudflare" {
		registryType = models.TypeTXT
		proxySupported = true
	} else {
		registryType = models.TypeCNAME
		proxySupported = false
	}

	// Initialize Syncer
	interval := time.Duration(cfg.IntervalSeconds) * time.Second
	syncService := syncer.NewSyncer(src, prov, interval, cfg.DomainFilters, cfg.Identifier, registryType, proxySupported)

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		slog.Info("Shutting down...")
		cancel()
	}()

	// Start Syncer
	syncService.Start(ctx)
}
