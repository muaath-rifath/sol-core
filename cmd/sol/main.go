package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/muaathrifath/sol-core/internal/ai"
	"github.com/muaathrifath/sol-core/internal/auth"
	"github.com/muaathrifath/sol-core/internal/automation"
	"github.com/muaathrifath/sol-core/internal/certs"
	"github.com/muaathrifath/sol-core/internal/config"
	"github.com/muaathrifath/sol-core/internal/device"
	"github.com/muaathrifath/sol-core/internal/firmware"
	"github.com/muaathrifath/sol-core/internal/home"
	"github.com/muaathrifath/sol-core/internal/mcp"
	"github.com/muaathrifath/sol-core/internal/mqtt"
	"github.com/muaathrifath/sol-core/internal/permission"
	"github.com/muaathrifath/sol-core/internal/platform"
	"github.com/muaathrifath/sol-core/internal/room"
	"github.com/muaathrifath/sol-core/internal/user"
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
	// defer rdb.Close()
	if rdb == nil {
		slog.Error("redis client is nil")
		os.Exit(1)
	}

	minioClient, err := platform.NewMinio(cfg.MinioEndpoint, cfg.MinioAccessKey, cfg.MinioSecretKey, cfg.MinioUseSSL)
	if err != nil {
		slog.Error("failed to connect to minio", "error", err)
		os.Exit(1)
	}

	if err := platform.EnsureBucket(minioClient, cfg.MinioBucket); err != nil {
		slog.Error("failed to ensure minio bucket", "bucket", cfg.MinioBucket, "error", err)
		os.Exit(1)
	}

	// Brevo email client (nil when API key not configured — email silently disabled)
	var brevoClient *platform.BrevoClient
	if cfg.BrevoAPIKey != "" {
		brevoClient = platform.NewBrevoClient(cfg.BrevoAPIKey, cfg.BrevoSenderEmail, cfg.BrevoSenderName)
		slog.Info("brevo email enabled", "sender", cfg.BrevoSenderEmail)
	} else {
		slog.Info("brevo api key not set — invitation emails disabled")
	}

	// User repository (needed by auth middleware)
	userRepo := user.NewRepository(pgPool)
	userSvc := user.NewService(userRepo)
	userHandler := user.NewHandler(userSvc)

	// Home service
	homeRepo := home.NewRepository(pgPool)
	homeSvc := home.NewService(homeRepo, userRepo, rdb, brevoClient, cfg.FrontendURL)
	homeHandler := home.NewHandler(homeSvc)

	// Auth
	oidcVerifier := auth.NewOIDCVerifier()
	// No implicit home creation on first login.
	authMiddleware := auth.NewMiddleware(oidcVerifier, userRepo, nil)

	// WebSocket hub
	hub := ws.NewHub(rdb)
	go hub.Run(ctx)

	// MQTT
	mqttClient, err := mqtt.NewClient(cfg.MQTTBrokerURL, cfg.MQTTClientID, cfg.MQTTUsername, cfg.MQTTPassword, cfg.CACertPath, cfg.ClientCertPath, cfg.ClientKeyPath)
	if err != nil {
		slog.Error("failed to connect to mqtt broker", "error", err)
		os.Exit(1)
	}
	defer mqttClient.Disconnect()

	// AI client
	aiClient := ai.NewClient(cfg.AIServiceURL)

	// Domain services
	roomRepo := room.NewRepository(pgPool)
	roomSvc := room.NewService(roomRepo)
	roomHandler := room.NewHandler(roomSvc)

	deviceRepo := device.NewRepository(pgPool)
	otaAttemptRepo := device.NewOTAAttemptRepository(pgPool)
	deviceSvc := device.NewService(
		deviceRepo,
		otaAttemptRepo,
		roomSvc,
		mqttClient,
		hub,
		time.Duration(cfg.OTAOnlineFreshnessSec)*time.Second,
		time.Duration(cfg.OTAAttemptTimeoutSec)*time.Second,
	)
	go deviceSvc.RunOTAAttemptWatchdog(ctx)

	// Certs Service
	certsSvc, err := certs.NewService(cfg.CACertPath, cfg.CAKeyPath)
	if err != nil {
		slog.Warn("certs service disabled (mTLS will not be available)", "error", err)
	}

	firmwareStore := firmware.NewStore(minioClient, cfg.MinioBucket)
	firmwareVersionRepo := firmware.NewVersionRepository(pgPool)
	firmwareBuildRepo := firmware.NewBuildRepository(pgPool)

	firmwareBuilder := firmware.NewBuilder(rdb, firmwareBuildRepo)
	firmwareHandler := firmware.NewHandler(firmwareStore, firmwareVersionRepo, firmwareBuildRepo, firmwareBuilder)
	deviceHandler := device.NewHandler(deviceSvc, firmwareStore, firmwareVersionRepo, certsSvc, cfg.PublicAPIURL, cfg.OTAAPIURL)

	automationRepo := automation.NewRepository(pgPool)
	automationSvc := automation.NewService(automationRepo, deviceSvc, aiClient)
	automationHandler := automation.NewHandler(automationSvc)

	// Permission service — must be wired before MCP / device gating.
	permRepo := permission.NewRepository(pgPool)
	permSvc := permission.NewService(permRepo)
	permHandler := permission.NewHandler(permSvc, roomSvc, deviceSvc)
	deviceSvc.SetPermissionGate(permSvc)
	roomSvc.SetPermissionGate(permSvc)

	// MCP Server
	mcpServer := mcp.NewServer(deviceSvc, roomSvc)

	// MQTT message handler
	mqttHandler := mqtt.NewHandler(deviceSvc, hub)
	mqttClient.SetMessageHandler(mqttHandler.Handle)

	// WS command handler — routes device.command messages from browser clients.
	// Inject the authenticated user into the context so the device gate can
	// look it up via user.FromContext (matches the HTTP auth middleware).
	hub.SetCommandHandler(func(ctx context.Context, u *user.User, msg ws.ClientMessage) error {
		if msg.Type != "device.command" {
			return nil
		}
		var req device.WSCommandRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			return fmt.Errorf("invalid command payload")
		}
		if u != nil {
			ctx = user.WithContext(ctx, u)
		}
		return deviceSvc.SendCommandForUser(ctx, req)
	})

	// Subscribe to device topics
	if err := mqttClient.Subscribe("sol/devices/+/state", 1); err != nil {
		slog.Error("failed to subscribe to device state", "error", err)
	}
	if err := mqttClient.Subscribe("sol/devices/+/telemetry", 1); err != nil {
		slog.Error("failed to subscribe to device telemetry", "error", err)
	}
	if err := mqttClient.Subscribe("sol/devices/+/ack", 1); err != nil {
		slog.Error("failed to subscribe to device ack", "error", err)
	}
	if err := mqttClient.Subscribe("sol/devices/+/ota", 1); err != nil {
		slog.Error("failed to subscribe to device ota status", "error", err)
	}

	// Routes
	mux := http.NewServeMux()

	// User routes
	mux.Handle("GET /api/v1/me", authMiddleware.Wrap(http.HandlerFunc(userHandler.Me)))

	// Home routes
	mux.Handle("POST /api/v1/homes", authMiddleware.Wrap(http.HandlerFunc(homeHandler.CreateHome)))
	mux.Handle("GET /api/v1/homes", authMiddleware.Wrap(http.HandlerFunc(homeHandler.ListHomes)))
	mux.Handle("GET /api/v1/homes/{id}", authMiddleware.Wrap(http.HandlerFunc(homeHandler.GetHome)))
	mux.Handle("PUT /api/v1/homes/{id}", authMiddleware.Wrap(http.HandlerFunc(homeHandler.UpdateHome)))
	mux.Handle("DELETE /api/v1/homes/{id}", authMiddleware.Wrap(http.HandlerFunc(homeHandler.DeleteHome)))
	mux.Handle("POST /api/v1/homes/{id}/transfer-ownership", authMiddleware.Wrap(http.HandlerFunc(homeHandler.TransferOwnership)))
	mux.Handle("GET /api/v1/homes/{id}/members", authMiddleware.Wrap(http.HandlerFunc(homeHandler.ListMembers)))
	mux.Handle("POST /api/v1/homes/{id}/members", authMiddleware.Wrap(http.HandlerFunc(homeHandler.AddMember)))
	mux.Handle("PATCH /api/v1/homes/{id}/members/{userId}/role", authMiddleware.Wrap(http.HandlerFunc(homeHandler.UpdateMemberRole)))
	mux.Handle("DELETE /api/v1/homes/{id}/members/{userId}", authMiddleware.Wrap(http.HandlerFunc(homeHandler.RemoveMember)))
	mux.Handle("GET /api/v1/homes/{id}/members/{userId}/permissions", authMiddleware.Wrap(http.HandlerFunc(permHandler.GetPermissions)))
	mux.Handle("PUT /api/v1/homes/{id}/members/{userId}/permissions", authMiddleware.Wrap(http.HandlerFunc(permHandler.PutPermissions)))
	mux.Handle("POST /api/v1/homes/{id}/invitations", authMiddleware.Wrap(http.HandlerFunc(homeHandler.InviteByEmail)))
	mux.Handle("GET /api/v1/homes/{id}/invitations", authMiddleware.Wrap(http.HandlerFunc(homeHandler.ListInvitations)))
	mux.Handle("DELETE /api/v1/homes/{id}/invitations/{invId}", authMiddleware.Wrap(http.HandlerFunc(homeHandler.CancelInvitation)))
	mux.Handle("POST /api/v1/invitations/{token}/accept", authMiddleware.Wrap(http.HandlerFunc(homeHandler.AcceptInvitation)))
	// Public invite endpoints — token is the secret, no account required
	mux.HandleFunc("GET /api/v1/invitations/{token}", homeHandler.GetInvitation)
	mux.HandleFunc("POST /api/v1/invitations/{token}/decline", homeHandler.DeclineInvitation)

	// Device routes
	mux.Handle("GET /api/v1/devices", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.List)))
	mux.Handle("POST /api/v1/devices", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.Create)))
	mux.Handle("GET /api/v1/devices/{id}", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.Get)))
	mux.Handle("GET /api/v1/devices/{id}/provision", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.Provision)))
	mux.Handle("GET /api/v1/devices/{id}/telemetry", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.GetTelemetry)))
	mux.Handle("PUT /api/v1/devices/{id}", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.Update)))
	mux.Handle("DELETE /api/v1/devices/{id}", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.Delete)))
	mux.Handle("POST /api/v1/devices/{id}/command", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.Command)))
	mux.Handle("GET /api/v1/homes/{homeId}/rooms/{roomId}/devices", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.ListByRoom)))
	mux.Handle("POST /api/v1/homes/{homeId}/rooms/{roomId}/devices", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.CreateInRoom)))
	mux.Handle("POST /api/v1/homes/{homeId}/rooms/{roomId}/devices/{id}/command", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.Command)))
	mux.Handle("POST /api/v1/homes/{homeId}/rooms/{roomId}/devices/{id}/ota", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.OTA)))
	mux.Handle("GET /api/v1/homes/{homeId}/rooms/{roomId}/ota-attempts", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.ListOTAAttempts)))
	mux.Handle("POST /api/v1/homes/{homeId}/rooms/{roomId}/ota-attempts/{attemptId}/retry", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.RetryOTA)))
	mux.Handle("POST /api/v1/homes/{homeId}/rooms/{roomId}/ota-attempts/{attemptId}/cancel", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.CancelOTA)))

	// Appliance routes
	mux.Handle("POST /api/v1/appliances", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.CreateAppliance)))
	mux.Handle("GET /api/v1/appliances/{applianceId}", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.GetAppliance)))
	mux.Handle("PUT /api/v1/appliances/{applianceId}", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.UpdateAppliance)))
	mux.Handle("DELETE /api/v1/appliances/{applianceId}", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.DeleteAppliance)))
	mux.Handle("GET /api/v1/homes/{homeId}/rooms/{roomId}/appliances", authMiddleware.Wrap(http.HandlerFunc(deviceHandler.ListAppliancesByRoom)))

	// Room routes
	mux.Handle("GET /api/v1/homes/{homeId}/rooms", authMiddleware.Wrap(http.HandlerFunc(roomHandler.List)))
	mux.Handle("POST /api/v1/homes/{homeId}/rooms", authMiddleware.Wrap(http.HandlerFunc(roomHandler.Create)))
	mux.Handle("GET /api/v1/homes/{homeId}/rooms/{roomId}", authMiddleware.Wrap(http.HandlerFunc(roomHandler.Get)))
	mux.Handle("PUT /api/v1/homes/{homeId}/rooms/{roomId}", authMiddleware.Wrap(http.HandlerFunc(roomHandler.Update)))
	mux.Handle("DELETE /api/v1/homes/{homeId}/rooms/{roomId}", authMiddleware.Wrap(http.HandlerFunc(roomHandler.Delete)))
	mux.Handle("GET /api/v1/homes/{homeId}/rooms/{roomId}/activity", authMiddleware.Wrap(http.HandlerFunc(roomHandler.ListActivity)))

	// Automation routes
	mux.Handle("GET /api/v1/automations", authMiddleware.Wrap(http.HandlerFunc(automationHandler.List)))
	mux.Handle("POST /api/v1/automations", authMiddleware.Wrap(http.HandlerFunc(automationHandler.Create)))
	mux.Handle("GET /api/v1/automations/{id}", authMiddleware.Wrap(http.HandlerFunc(automationHandler.Get)))
	mux.Handle("PUT /api/v1/automations/{id}", authMiddleware.Wrap(http.HandlerFunc(automationHandler.Update)))
	mux.Handle("DELETE /api/v1/automations/{id}", authMiddleware.Wrap(http.HandlerFunc(automationHandler.Delete)))

	// Firmware routes
	mux.Handle("GET /api/v1/firmware", authMiddleware.Wrap(http.HandlerFunc(firmwareHandler.List)))
	mux.Handle("POST /api/v1/firmware/upload", authMiddleware.Wrap(http.HandlerFunc(firmwareHandler.Upload)))
	mux.Handle("POST /api/v1/firmware/build", authMiddleware.Wrap(http.HandlerFunc(firmwareHandler.Build)))
	mux.Handle("GET /api/v1/firmware/builds/{id}", authMiddleware.Wrap(http.HandlerFunc(firmwareHandler.GetBuild)))
	mux.Handle("GET /api/v1/firmware/targets", authMiddleware.Wrap(http.HandlerFunc(firmwareHandler.ListTargets)))
	mux.Handle("GET /api/v1/firmware/versions/{id}/download", authMiddleware.Wrap(http.HandlerFunc(firmwareHandler.DownloadByVersionID)))
	mux.Handle("GET /api/v1/firmware/versions/{id}/presigned-url", authMiddleware.Wrap(http.HandlerFunc(firmwareHandler.PresignedURL)))

	// Public OTA firmware download (no auth)
	mux.HandleFunc("GET /api/v1/ota/firmware/{id}", firmwareHandler.DownloadByVersionID)

	// Device OTA firmware download (mTLS protected)
	mux.HandleFunc("GET /api/v1/ota/attempts/{attemptId}/firmware", deviceHandler.DownloadOTAFirmware)


	// Internal build routes (for the worker)
	mux.HandleFunc("PATCH /api/internal/firmware/builds/{id}", firmwareHandler.UpdateBuildStatus)
	mux.HandleFunc("POST /api/internal/firmware/builds/{id}/logs", firmwareHandler.AppendBuildLogs)

	// WebSocket
	mux.Handle("/ws", authMiddleware.Wrap(http.HandlerFunc(hub.HandleWebSocket)))

	mux.Handle("GET /api/v1/mcp/sse", mcpServer.Handler())
	mux.Handle("POST /api/v1/mcp/sse", mcpServer.Handler())

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
