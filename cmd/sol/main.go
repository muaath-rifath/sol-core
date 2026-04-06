package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/muaathrifath/sol-core/internal/ai"
	"github.com/muaathrifath/sol-core/internal/auth"
	"github.com/muaathrifath/sol-core/internal/automation"
	"github.com/muaathrifath/sol-core/internal/config"
	"github.com/muaathrifath/sol-core/internal/device"
	"github.com/muaathrifath/sol-core/internal/firmware"
	"github.com/muaathrifath/sol-core/internal/mqtt"
	"github.com/muaathrifath/sol-core/internal/platform"
	"github.com/muaathrifath/sol-core/internal/ws"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Platform clients
	pgPool, err := platform.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pgPool.Close()

	rdb := platform.NewRedis(cfg.RedisURL)
	defer rdb.Close()

	minioClient, err := platform.NewMinio(cfg.MinioEndpoint, cfg.MinioAccessKey, cfg.MinioSecretKey, cfg.MinioUseSSL)
	if err != nil {
		slog.Error("failed to connect to minio", "error", err)
		os.Exit(1)
	}

	// Auth
	oidcVerifier, err := auth.NewOIDCVerifier(ctx, cfg.OIDCIssuer, cfg.OIDCClientID)
	if err != nil {
		slog.Error("failed to init oidc verifier", "error", err)
		os.Exit(1)
	}
	authMiddleware := auth.NewMiddleware(oidcVerifier)

	// WebSocket hub
	hub := ws.NewHub(rdb)
	go hub.Run(ctx)

	// MQTT
	mqttClient, err := mqtt.NewClient(cfg.MQTTBrokerURL, cfg.MQTTClientID)
	if err != nil {
		slog.Error("failed to connect to mqtt broker", "error", err)
		os.Exit(1)
	}
	defer mqttClient.Disconnect()

	// AI client
	aiClient := ai.NewClient(cfg.AIServiceURL)

	// Domain services
	deviceRepo := device.NewRepository(pgPool)
	deviceSvc := device.NewService(deviceRepo, mqttClient, hub)
	deviceHandler := device.NewHandler(deviceSvc)

	automationRepo := automation.NewRepository(pgPool)
	automationSvc := automation.NewService(automationRepo, deviceSvc, aiClient)
	automationHandler := automation.NewHandler(automationSvc)

	firmwareStore := firmware.NewStore(minioClient, cfg.MinioBucket)
	firmwareHandler := firmware.NewHandler(firmwareStore)

	// MQTT message handler
	mqttHandler := mqtt.NewHandler(deviceSvc, hub)
	mqttClient.SetMessageHandler(mqttHandler.Handle)

	// Routes
	mux := http.NewServeMux()

	// Device routes
	mux.Handle("GET /api/v1/devices", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.List)))
	mux.Handle("POST /api/v1/devices", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.Create)))
	mux.Handle("GET /api/v1/devices/{id}", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.Get)))
	mux.Handle("PUT /api/v1/devices/{id}", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.Update)))
	mux.Handle("DELETE /api/v1/devices/{id}", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.Delete)))
	mux.Handle("POST /api/v1/devices/{id}/command", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.Command)))

	// Automation routes
	mux.Handle("GET /api/v1/automations", authMiddleware.Wrap(http.HandlerFunc(automationHandler.List)))
	mux.Handle("POST /api/v1/automations", authMiddleware.Wrap(http.HandlerFunc(automationHandler.Create)))
	mux.Handle("GET /api/v1/automations/{id}", authMiddleware.Wrap(http.HandlerFunc(automationHandler.Get)))
	mux.Handle("PUT /api/v1/automations/{id}", authMiddleware.Wrap(http.HandlerFunc(automationHandler.Update)))
	mux.Handle("DELETE /api/v1/automations/{id}", authMiddleware.Wrap(http.HandlerFunc(automationHandler.Delete)))

	// Firmware routes
	mux.Handle("GET /api/v1/firmware", authMiddleware.Wrap(http.HandlerFunc(firmwareHandler.List)))
	mux.Handle("POST /api/v1/firmware/upload", authMiddleware.Wrap(http.HandlerFunc(firmwareHandler.Upload)))
	mux.Handle("GET /api/v1/firmware/{id}/download", authMiddleware.Wrap(http.HandlerFunc(firmwareHandler.Download)))

	// WebSocket
	mux.Handle("/ws", authMiddleware.Wrap(http.HandlerFunc(hub.HandleWebSocket)))

	// Health
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		slog.Info("shutting down server")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown error", "error", err)
		}
		cancel()
	}()

	slog.Info("starting server", "port", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
