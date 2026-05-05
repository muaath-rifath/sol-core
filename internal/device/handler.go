package device

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/muaathrifath/sol-core/internal/certs"
	"github.com/muaathrifath/sol-core/internal/firmware"
	"github.com/muaathrifath/sol-core/internal/user"
)

type Handler struct {
	svc           *Service
	firmwareStore *firmware.Store
	versionRepo   *firmware.VersionRepository
	certsSvc      *certs.Service
	publicAPIURL  string
	otaAPIURL     string
}

func NewHandler(svc *Service, firmwareStore *firmware.Store, versionRepo *firmware.VersionRepository, certsSvc *certs.Service, publicAPIURL string, otaAPIURL string) *Handler {
	return &Handler{svc: svc, firmwareStore: firmwareStore, versionRepo: versionRepo, certsSvc: certsSvc, publicAPIURL: publicAPIURL, otaAPIURL: otaAPIURL}
}

func (h *Handler) Provision(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error":"device id is required"}`, http.StatusBadRequest)
		return
	}

	// Verify device exists
	_, err := h.svc.Get(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"device not found"}`, http.StatusNotFound)
		return
	}

	if h.certsSvc == nil {
		http.Error(w, `{"error":"mTLS service not configured"}`, http.StatusNotImplemented)
		return
	}

	bundle, err := h.certsSvc.GenerateDeviceCertificate(id)
	if err != nil {
		slog.Error("provision device certs", "error", err, "device_id", id)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, bundle)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	cursor := r.URL.Query().Get("cursor")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	resp, err := h.svc.ListPaginated(r.Context(), cursor, limit)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			http.Error(w, `{"error":"invalid cursor"}`, http.StatusUnprocessableEntity)
			return
		}
		slog.Error("list devices", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	if resp.Data == nil {
		resp.Data = []Device{}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		http.Error(w, `{"error":"missing request body"}`, http.StatusBadRequest)
		return
	}
	var req CreateDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	d, err := h.svc.Create(r.Context(), req)
	if err != nil {
		slog.Error("create device", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, d)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	d, err := h.svc.Get(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (h *Handler) GetTelemetry(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	points, err := h.svc.GetRecentTelemetry(r.Context(), id, limit)
	if err != nil {
		slog.Error("get telemetry", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	if points == nil {
		points = []TelemetryPoint{}
	}
	writeJSON(w, http.StatusOK, points)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if r.Body == nil {
		http.Error(w, `{"error":"missing request body"}`, http.StatusBadRequest)
		return
	}
	var req UpdateDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	d, err := h.svc.Update(r.Context(), id, req)
	if err != nil {
		slog.Error("update device", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		slog.Error("delete device", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Command(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	homeID := r.PathValue("homeId")
	roomID := r.PathValue("roomId")
	if roomID != "" {
		if homeID != "" {
			belongs, err := h.svc.repo.RoomBelongsToHome(r.Context(), roomID, homeID)
			if err != nil {
				slog.Error("check room membership", "error", err)
				http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
				return
			}
			if !belongs {
				http.Error(w, `{"error":"room not found"}`, http.StatusNotFound)
				return
			}
		}

		if _, err := h.svc.repo.GetByIDInRoom(r.Context(), id, roomID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				http.Error(w, `{"error":"device not found"}`, http.StatusNotFound)
				return
			}
			slog.Error("get device in room", "error", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
	}

	if r.Body == nil {
		http.Error(w, `{"error":"missing request body"}`, http.StatusBadRequest)
		return
	}
	var cmd Command
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	cmd.DeviceID = id

	if err := h.svc.GateCommand(r.Context(), homeID, id, cmd.Params); err != nil {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	if err := h.svc.SendCommand(r.Context(), cmd); err != nil {
		slog.Error("send command", "error", err)
		http.Error(w, `{"error":"failed to send command"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) ListByRoom(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	homeID := r.PathValue("homeId")
	roomID := r.PathValue("roomId")
	if homeID == "" || roomID == "" {
		http.Error(w, `{"error":"homeId and roomId are required"}`, http.StatusBadRequest)
		return
	}

	belongs, err := h.svc.repo.RoomBelongsToHome(r.Context(), roomID, homeID)
	if err != nil {
		slog.Error("check room membership", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	if !belongs {
		http.Error(w, `{"error":"room not found"}`, http.StatusNotFound)
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	resp, err := h.svc.ListByRoomPaginated(r.Context(), roomID, cursor, limit)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			http.Error(w, `{"error":"invalid cursor"}`, http.StatusUnprocessableEntity)
			return
		}
		slog.Error("list devices by room", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	if resp.Data == nil {
		resp.Data = []Device{}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) CreateInRoom(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	homeID := r.PathValue("homeId")
	roomID := r.PathValue("roomId")
	if homeID == "" || roomID == "" {
		http.Error(w, `{"error":"homeId and roomId are required"}`, http.StatusBadRequest)
		return
	}

	belongs, err := h.svc.repo.RoomBelongsToHome(r.Context(), roomID, homeID)
	if err != nil {
		slog.Error("check room membership", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	if !belongs {
		http.Error(w, `{"error":"room not found"}`, http.StatusNotFound)
		return
	}

	var req CreateDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusUnprocessableEntity)
		return
	}
	req.RoomID = roomID

	d, err := h.svc.Create(r.Context(), req)
	if err != nil {
		slog.Error("create device in room", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, d)
}

func (h *Handler) OTA(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	homeID := r.PathValue("homeId")
	roomID := r.PathValue("roomId")
	deviceID := r.PathValue("id")

	belongs, err := h.svc.repo.RoomBelongsToHome(r.Context(), roomID, homeID)
	if err != nil {
		slog.Error("check room membership", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	if !belongs {
		http.Error(w, `{"error":"room not found"}`, http.StatusNotFound)
		return
	}

	d, err := h.svc.repo.GetByIDInRoom(r.Context(), deviceID, roomID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, `{"error":"device not found"}`, http.StatusNotFound)
			return
		}
		slog.Error("get device in room", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	if !d.Online || time.Since(d.UpdatedAt) > h.svc.OnlineFreshness() {
		http.Error(w, `{"error":"device is offline; OTA requires an online device"}`, http.StatusConflict)
		return
	}

	var req OTARequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.FirmwareVersionID) == "" {
		http.Error(w, `{"error":"firmware_version_id is required"}`, http.StatusBadRequest)
		return
	}

	v, err := h.versionRepo.GetByID(r.Context(), req.FirmwareVersionID)
	if err != nil {
		http.Error(w, `{"error":"firmware version not found"}`, http.StatusNotFound)
		return
	}

	var requestedBy *string
	if u := user.FromContext(r.Context()); u != nil {
		requestedBy = &u.ID
	}

	var idempotencyKey *string
	if key := strings.TrimSpace(req.IdempotencyKey); key != "" {
		idempotencyKey = &key
	}

	// Will populate URL after creating attempt
	attempt, err := h.svc.TriggerOTA(r.Context(), d, homeID, roomID, v.ID, requestedBy, idempotencyKey)
	if err != nil {
		slog.Error("trigger ota", "error", err)
		if strings.Contains(strings.ToLower(err.Error()), "already in progress") {
			http.Error(w, `{"error":"an OTA update is already in progress for this device"}`, http.StatusConflict)
			return
		}
		http.Error(w, `{"error":"failed to send command"}`, http.StatusInternalServerError)
		return
	}

	url := fmt.Sprintf("%s/api/v1/ota/attempts/%s/firmware", h.otaAPIURL, attempt.ID)
	err = h.svc.SendOTACommand(r.Context(), d, attempt, url)
	if err != nil {
		slog.Error("trigger ota command", "error", err)
		http.Error(w, `{"error":"failed to send command"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"device_id":           d.ID,
		"firmware_version_id": v.ID,
		"attempt_id":          attempt.ID,
		"request_id":          attempt.RequestID,
		"status":              attempt.Status,
	})
}

func (h *Handler) ListOTAAttempts(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	homeID := r.PathValue("homeId")
	roomID := r.PathValue("roomId")
	if homeID == "" || roomID == "" {
		http.Error(w, `{"error":"homeId and roomId are required"}`, http.StatusBadRequest)
		return
	}

	belongs, err := h.svc.repo.RoomBelongsToHome(r.Context(), roomID, homeID)
	if err != nil {
		slog.Error("check room membership", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	if !belongs {
		http.Error(w, `{"error":"room not found"}`, http.StatusNotFound)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	attempts, err := h.svc.ListOTAAttemptsByRoom(r.Context(), roomID, limit)
	if err != nil {
		slog.Error("list ota attempts", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	if attempts == nil {
		attempts = []OTAAttempt{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": attempts})
}

func (h *Handler) RetryOTA(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	homeID := r.PathValue("homeId")
	roomID := r.PathValue("roomId")
	attemptID := r.PathValue("attemptId")
	if homeID == "" || roomID == "" || attemptID == "" {
		http.Error(w, `{"error":"homeId, roomId and attemptId are required"}`, http.StatusBadRequest)
		return
	}

	belongs, err := h.svc.repo.RoomBelongsToHome(r.Context(), roomID, homeID)
	if err != nil {
		slog.Error("check room membership", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	if !belongs {
		http.Error(w, `{"error":"room not found"}`, http.StatusNotFound)
		return
	}

	prev, err := h.svc.GetOTAAttemptByID(r.Context(), attemptID)
	if err != nil {
		http.Error(w, `{"error":"attempt not found"}`, http.StatusNotFound)
		return
	}
	if prev.RoomID != roomID || prev.HomeID != homeID {
		http.Error(w, `{"error":"attempt not found"}`, http.StatusNotFound)
		return
	}

	if prev.Status != OTAAttemptStatusFailed && prev.Status != OTAAttemptStatusUpdated && prev.Status != OTAAttemptStatusCancelled && prev.Status != OTAAttemptStatusTimedOut {
		http.Error(w, `{"error":"attempt is still in progress"}`, http.StatusConflict)
		return
	}

	d, err := h.svc.repo.GetByIDInRoom(r.Context(), prev.DeviceID, roomID)
	if err != nil {
		http.Error(w, `{"error":"device not found"}`, http.StatusNotFound)
		return
	}
	if !d.Online || time.Since(d.UpdatedAt) > h.svc.OnlineFreshness() {
		http.Error(w, `{"error":"device is offline; OTA requires an online device"}`, http.StatusConflict)
		return
	}

	v, err := h.versionRepo.GetByID(r.Context(), prev.FirmwareVersionID)
	if err != nil {
		http.Error(w, `{"error":"firmware version not found"}`, http.StatusNotFound)
		return
	}

	requestedBy := &u.ID
	attempt, err := h.svc.TriggerOTA(r.Context(), d, homeID, roomID, v.ID, requestedBy, nil)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already in progress") {
			http.Error(w, `{"error":"an OTA update is already in progress for this device"}`, http.StatusConflict)
			return
		}
		http.Error(w, `{"error":"failed to send command"}`, http.StatusInternalServerError)
		return
	}

	url := fmt.Sprintf("%s/api/v1/ota/attempts/%s/firmware", h.otaAPIURL, attempt.ID)
	err = h.svc.SendOTACommand(r.Context(), d, attempt, url)
	if err != nil {
		slog.Error("trigger ota command", "error", err)
		http.Error(w, `{"error":"failed to send command"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"attempt_id": attempt.ID,
		"request_id": attempt.RequestID,
		"status":     attempt.Status,
	})
}

func (h *Handler) CancelOTA(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	homeID := r.PathValue("homeId")
	roomID := r.PathValue("roomId")
	attemptID := r.PathValue("attemptId")
	if homeID == "" || roomID == "" || attemptID == "" {
		http.Error(w, `{"error":"homeId, roomId and attemptId are required"}`, http.StatusBadRequest)
		return
	}

	belongs, err := h.svc.repo.RoomBelongsToHome(r.Context(), roomID, homeID)
	if err != nil {
		slog.Error("check room membership", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	if !belongs {
		http.Error(w, `{"error":"room not found"}`, http.StatusNotFound)
		return
	}

	attempt, err := h.svc.GetOTAAttemptByID(r.Context(), attemptID)
	if err != nil {
		http.Error(w, `{"error":"attempt not found"}`, http.StatusNotFound)
		return
	}
	if attempt.RoomID != roomID || attempt.HomeID != homeID {
		http.Error(w, `{"error":"attempt not found"}`, http.StatusNotFound)
		return
	}

	updated, err := h.svc.CancelOTAAttempt(r.Context(), attempt)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not cancellable") {
			http.Error(w, `{"error":"attempt is not cancellable"}`, http.StatusConflict)
			return
		}
		http.Error(w, `{"error":"failed to cancel ota"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"attempt_id": updated.ID,
		"request_id": updated.RequestID,
		"status":     updated.Status,
	})
}

func (h *Handler) DownloadOTAFirmware(w http.ResponseWriter, r *http.Request) {
	attemptID := r.PathValue("attemptId")
	if attemptID == "" {
		http.Error(w, `{"error":"attempt_id is required"}`, http.StatusBadRequest)
		return
	}

	// 1. Get the certificate from the header (passed by Traefik mtls-header middleware)
	certHeader := r.Header.Get("X-Forwarded-Tls-Client-Cert")
	if certHeader == "" {
		http.Error(w, `{"error":"client certificate required"}`, http.StatusUnauthorized)
		return
	}

	// 2. Decode the PEM block.
	// The header might be passed as-is or URL-encoded by the proxy.
	// Some proxies pass the raw base64 (DER) without PEM headers.
	var block *pem.Block
	block, _ = pem.Decode([]byte(certHeader))
	if block == nil {
		// Try unescaping and decoding
		certPEM, err := url.QueryUnescape(certHeader)
		if err == nil {
			block, _ = pem.Decode([]byte(certPEM))
		}
	}

	// If still nil, check if it's raw base64 (DER)
	if block == nil {
		raw := certHeader
		if unescaped, err := url.QueryUnescape(certHeader); err == nil {
			raw = unescaped
		}
		
		// Common issue: + being unescaped to space
		raw = strings.ReplaceAll(raw, " ", "+")

		der, err := base64.StdEncoding.DecodeString(raw)
		if err == nil {
			block = &pem.Block{Type: "CERTIFICATE", Bytes: der}
		}
	}

	if block == nil {
		prefix := certHeader
		if len(prefix) > 100 {
			prefix = prefix[:100]
		}
		slog.Error("failed to decode client certificate from header",
			"header_len", len(certHeader),
			"header_prefix", prefix,
			"attempt_id", attemptID)
		http.Error(w, `{"error":"failed to decode client certificate"}`, http.StatusBadRequest)
		return
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		slog.Error("failed to parse client certificate", "error", err, "attempt_id", attemptID)
		http.Error(w, `{"error":"invalid client certificate"}`, http.StatusBadRequest)
		return
	}

	deviceID := cert.Subject.CommonName
	if deviceID == "" {
		http.Error(w, `{"error":"client certificate missing device identity"}`, http.StatusUnauthorized)
		return
	}

	// Verify attempt
	attempt, err := h.svc.GetOTAAttemptByID(r.Context(), attemptID)
	if err != nil {
		http.Error(w, `{"error":"attempt not found"}`, http.StatusNotFound)
		return
	}

	if attempt.DeviceID != deviceID {
		http.Error(w, `{"error":"unauthorized for this attempt"}`, http.StatusForbidden)
		return
	}

	if attempt.Status == OTAAttemptStatusCancelled || attempt.Status == OTAAttemptStatusTimedOut || attempt.Status == OTAAttemptStatusFailed {
		http.Error(w, `{"error":"attempt is no longer active"}`, http.StatusGone)
		return
	}

	// Get firmware
	v, err := h.versionRepo.GetByID(r.Context(), attempt.FirmwareVersionID)
	if err != nil {
		http.Error(w, `{"error":"firmware version not found"}`, http.StatusNotFound)
		return
	}

	reader, err := h.firmwareStore.Download(r.Context(), v.AppKey)
	if err != nil {
		http.Error(w, `{"error":"failed to download firmware image"}`, http.StatusInternalServerError)
		return
	}
	defer reader.Close()

	// Disable write deadline for firmware streaming — the 15s server WriteTimeout
	// will cut the connection mid-transfer on slow WiFi otherwise.
	http.NewResponseController(w).SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-%s-app.bin", v.TemplateID, v.Version))
	if v.SizeBytes != nil && *v.SizeBytes > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", *v.SizeBytes))
	}
	io.Copy(w, reader)
}

func (h *Handler) CreateAppliance(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if r.Body == nil {
		http.Error(w, `{"error":"missing request body"}`, http.StatusBadRequest)
		return
	}
	var req CreateApplianceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if gate := h.svc.PermGate(); gate != nil && req.DeviceID != "" {
		homeID, err := h.svc.HomeIDForDevice(r.Context(), req.DeviceID)
		if err != nil {
			http.Error(w, `{"error":"device not found"}`, http.StatusNotFound)
			return
		}
		role, err := gate.MemberRole(r.Context(), homeID, u.ID)
		if err != nil || (role != "owner" && role != "admin") {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
	}

	app, err := h.svc.CreateAppliance(r.Context(), req)
	if err != nil {
		slog.Error("create appliance", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, app)
}

func (h *Handler) GetAppliance(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	id := r.PathValue("applianceId")
	app, err := h.svc.GetAppliance(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if gate := h.svc.PermGate(); gate != nil {
		ok, err := gate.CheckAppliance(r.Context(), u.ID, id)
		if err != nil || !ok {
			// 404 rather than 403 to avoid leaking existence to non-members.
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
	}
	writeJSON(w, http.StatusOK, app)
}

func (h *Handler) UpdateAppliance(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	id := r.PathValue("applianceId")
	if r.Body == nil {
		http.Error(w, `{"error":"missing request body"}`, http.StatusBadRequest)
		return
	}
	var req UpdateApplianceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := h.requireApplianceManager(r.Context(), id, u.ID); err != nil {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	app, err := h.svc.UpdateAppliance(r.Context(), id, req)
	if err != nil {
		slog.Error("update appliance", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (h *Handler) DeleteAppliance(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	id := r.PathValue("applianceId")

	if err := h.requireApplianceManager(r.Context(), id, u.ID); err != nil {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	if err := h.svc.DeleteAppliance(r.Context(), id); err != nil {
		slog.Error("delete appliance", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// requireApplianceManager returns nil when the user is owner or admin of the
// home that owns the appliance. Returns a non-nil error otherwise (forbidden).
// When the permission gate is not wired, returns nil.
func (h *Handler) requireApplianceManager(ctx context.Context, applianceID, userID string) error {
	gate := h.svc.PermGate()
	if gate == nil {
		return nil
	}
	homeID, err := h.svc.HomeIDForAppliance(ctx, applianceID)
	if err != nil {
		return err
	}
	role, err := gate.MemberRole(ctx, homeID, userID)
	if err != nil {
		return err
	}
	if role != "owner" && role != "admin" {
		return errForbidden
	}
	return nil
}

var errForbidden = errors.New("forbidden")

func (h *Handler) ListAppliancesByRoom(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	homeID := r.PathValue("homeId")
	roomID := r.PathValue("roomId")
	apps, err := h.svc.ListAppliancesByRoom(r.Context(), roomID)
	if err != nil {
		slog.Error("list appliances by room", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	if apps == nil {
		apps = []Appliance{}
	}

	if gate := h.svc.PermGate(); gate != nil && homeID != "" {
		allowed, allAccess, err := gate.ListAccessibleApplianceIDs(r.Context(), homeID, u.ID)
		if err != nil {
			slog.Error("permission filter", "error", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		if !allAccess {
			set := map[string]struct{}{}
			for _, id := range allowed {
				set[id] = struct{}{}
			}
			filtered := apps[:0]
			for _, a := range apps {
				if _, ok := set[a.ID]; ok {
					filtered = append(filtered, a)
				}
			}
			apps = filtered
			if apps == nil {
				apps = []Appliance{}
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": apps})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
