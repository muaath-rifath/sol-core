package device

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/muaathrifath/sol-core/internal/mqtt"
	"github.com/muaathrifath/sol-core/internal/room"
	"github.com/muaathrifath/sol-core/internal/ws"
)

var ErrValidation = errors.New("validation error")

const defaultLimit = 20
const maxLimit = 100

func normalizeLimit(limit int) int {
	if limit < 1 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

func encodeCursor(t time.Time, id string) string {
	raw := t.UTC().Format(time.RFC3339Nano) + "|" + id
	return base64.URLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(s string) (*time.Time, string, error) {
	if s == "" {
		return nil, "", nil
	}
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return nil, "", fmt.Errorf("%w: invalid cursor", ErrValidation)
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return nil, "", fmt.Errorf("%w: invalid cursor format", ErrValidation)
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, "", fmt.Errorf("%w: invalid cursor timestamp", ErrValidation)
	}
	return &t, parts[1], nil
}

func buildCursorResponse[T any](items []T, limit int, cursorFn func(T) (time.Time, string)) *CursorResponse[T] {
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	resp := &CursorResponse[T]{Data: items, HasMore: hasMore}
	if hasMore && len(items) > 0 {
		t, id := cursorFn(items[len(items)-1])
		encoded := encodeCursor(t, id)
		resp.NextCursor = &encoded
	}
	return resp
}

type Service struct {
	repo              *Repository
	otaRepo           *OTAAttemptRepository
	roomSvc           *room.Service
	mqtt              *mqtt.Client
	hub               *ws.Hub
	onlineFreshness   time.Duration
	otaAttemptTimeout time.Duration
}

func NewService(repo *Repository, otaRepo *OTAAttemptRepository, roomSvc *room.Service, mqttClient *mqtt.Client, hub *ws.Hub, onlineFreshness time.Duration, otaAttemptTimeout time.Duration) *Service {
	if onlineFreshness <= 0 {
		onlineFreshness = 45 * time.Second
	}
	if otaAttemptTimeout <= 0 {
		otaAttemptTimeout = 8 * time.Minute
	}
	return &Service{
		repo:              repo,
		otaRepo:           otaRepo,
		roomSvc:           roomSvc,
		mqtt:              mqttClient,
		hub:               hub,
		onlineFreshness:   onlineFreshness,
		otaAttemptTimeout: otaAttemptTimeout,
	}
}

func (s *Service) OnlineFreshness() time.Duration {
	if s.onlineFreshness <= 0 {
		return 45 * time.Second
	}
	return s.onlineFreshness
}

func (s *Service) Create(ctx context.Context, req CreateDeviceRequest) (*Device, error) {
	d := &Device{
		ID:        uuid.NewString(),
		Name:      req.Name,
		Type:      req.Type,
		RoomID:    req.RoomID,
		State:     make(map[string]any),
		Metadata:  req.Metadata,
		Online:    false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.Create(ctx, d); err != nil {
		return nil, fmt.Errorf("create device: %w", err)
	}
	return d, nil
}

func (s *Service) Get(ctx context.Context, id string) (*Device, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]Device, error) {
	return s.repo.List(ctx)
}

func (s *Service) ListPaginated(ctx context.Context, cursor string, limit int) (*CursorResponse[Device], error) {
	limit = normalizeLimit(limit)
	cursorTime, cursorID, err := decodeCursor(cursor)
	if err != nil {
		return nil, err
	}
	devices, err := s.repo.ListPaginated(ctx, cursorTime, cursorID, limit+1)
	if err != nil {
		return nil, err
	}
	return buildCursorResponse(devices, limit, func(d Device) (time.Time, string) {
		return d.CreatedAt, d.ID
	}), nil
}

func (s *Service) ListByRoom(ctx context.Context, roomID string) ([]Device, error) {
	return s.repo.ListByRoom(ctx, roomID)
}

func (s *Service) ListByRoomPaginated(ctx context.Context, roomID, cursor string, limit int) (*CursorResponse[Device], error) {
	limit = normalizeLimit(limit)
	cursorTime, cursorID, err := decodeCursor(cursor)
	if err != nil {
		return nil, err
	}
	devices, err := s.repo.ListByRoomPaginated(ctx, roomID, cursorTime, cursorID, limit+1)
	if err != nil {
		return nil, err
	}
	return buildCursorResponse(devices, limit, func(d Device) (time.Time, string) {
		return d.CreatedAt, d.ID
	}), nil
}

func (s *Service) Update(ctx context.Context, id string, req UpdateDeviceRequest) (*Device, error) {
	d, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		d.Name = *req.Name
	}
	if req.RoomID != nil {
		d.RoomID = *req.RoomID
	}
	if req.Metadata != nil {
		d.Metadata = *req.Metadata
	}
	d.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, d); err != nil {
		return nil, fmt.Errorf("update device: %w", err)
	}
	return d, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) SendCommand(ctx context.Context, cmd Command) error {
	topic := fmt.Sprintf("sol/devices/%s/cmd", cmd.DeviceID)
	return s.mqtt.Publish(topic, cmd)
}

func (s *Service) TriggerOTA(ctx context.Context, d *Device, homeID, roomID, firmwareVersionID string, requestedBy *string, idempotencyKey *string) (*OTAAttempt, error) {
	if d == nil {
		return nil, fmt.Errorf("device is required")
	}

	if idempotencyKey != nil && strings.TrimSpace(*idempotencyKey) != "" {
		existing, err := s.otaRepo.GetByIdempotencyKey(ctx, strings.TrimSpace(*idempotencyKey))
		if err == nil {
			return existing, nil
		}
	}

	hasActive, err := s.otaRepo.HasActiveForDevice(ctx, d.ID)
	if err != nil {
		return nil, err
	}
	if hasActive {
		return nil, fmt.Errorf("another ota update is already in progress")
	}

	now := time.Now()
	attempt := &OTAAttempt{
		ID:                uuid.NewString(),
		DeviceID:          d.ID,
		RoomID:            roomID,
		HomeID:            homeID,
		FirmwareVersionID: firmwareVersionID,
		RequestedBy:       requestedBy,
		IdempotencyKey:    idempotencyKey,
		RequestID:         uuid.NewString(),
		Status:            OTAAttemptStatusInitiated,
		ProgressPct:       0,
		Logs:              "OTA request accepted by server",
		StartedAt:         now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := s.otaRepo.Create(ctx, attempt); err != nil {
		return nil, err
	}

	if roomID != "" {
		_ = s.roomSvc.InsertActivityLog(ctx, &room.ActivityLog{
			RoomID:      roomID,
			Timestamp:   time.Now(),
			Title:       "OTA Flash Started",
			Description: fmt.Sprintf("Started OTA flashing for %s", d.Name),
			BadgeText:   "Started",
			BadgeColor:  "bg-tertiary-fixed text-on-tertiary-fixed",
		})
	}

	s.hub.Broadcast("ota.attempt.updated", attempt)
	return attempt, nil
}

func (s *Service) SendOTACommand(ctx context.Context, d *Device, attempt *OTAAttempt, url string) error {
	topic := fmt.Sprintf("sol/devices/%s/cmd", d.ID)
	if err := s.mqtt.Publish(topic, Command{
		DeviceID:  d.ID,
		RequestID: attempt.RequestID,
		Action:    "ota_update",
		Params: map[string]any{
			"url":        url,
			"request_id": attempt.RequestID,
		},
	}); err != nil {
		msg := err.Error()
		finishedAt := time.Now()
		_ = s.otaRepo.AppendLog(ctx, attempt.RequestID, "Failed to publish OTA command: "+msg)
		_ = s.otaRepo.UpdateProgress(ctx, attempt.RequestID, OTAAttemptStatusFailed, 0, &msg, &finishedAt)
		return err
	}
	return nil
}

func isOTATerminalStatus(status OTAAttemptStatus) bool {
	switch status {
	case OTAAttemptStatusUpdated, OTAAttemptStatusFailed, OTAAttemptStatusCancelled, OTAAttemptStatusTimedOut:
		return true
	default:
		return false
	}
}

func isOTACancellableStatus(status OTAAttemptStatus) bool {
	switch status {
	case OTAAttemptStatusInitiated, OTAAttemptStatusAcknowledged, OTAAttemptStatusDownloading, OTAAttemptStatusVerifying, OTAAttemptStatusUpdating:
		return true
	default:
		return false
	}
}

func (s *Service) CancelOTAAttempt(ctx context.Context, attempt *OTAAttempt) (*OTAAttempt, error) {
	if attempt == nil {
		return nil, fmt.Errorf("attempt is required")
	}
	if !isOTACancellableStatus(attempt.Status) {
		if isOTATerminalStatus(attempt.Status) {
			return attempt, nil
		}
		return nil, fmt.Errorf("attempt is not cancellable")
	}

	if err := s.otaRepo.AppendLog(ctx, attempt.RequestID, "Cancel requested by user"); err != nil {
		return nil, err
	}
	if err := s.otaRepo.UpdateProgress(ctx, attempt.RequestID, OTAAttemptStatusCancelling, attempt.ProgressPct, nil, nil); err != nil {
		return nil, err
	}

	topic := fmt.Sprintf("sol/devices/%s/cmd", attempt.DeviceID)
	if err := s.mqtt.Publish(topic, Command{
		DeviceID:  attempt.DeviceID,
		RequestID: attempt.RequestID,
		Action:    "ota_cancel",
		Params: map[string]any{
			"request_id": attempt.RequestID,
		},
	}); err != nil {
		_ = s.otaRepo.AppendLog(ctx, attempt.RequestID, "Failed to publish cancel command: "+err.Error())
		return nil, err
	}

	updated, err := s.otaRepo.GetByRequestID(ctx, attempt.RequestID)
	if err != nil {
		return nil, err
	}
	s.hub.Broadcast("ota.attempt.updated", updated)
	return updated, nil
}

func (s *Service) RunOTAAttemptWatchdog(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.beforeOTATriggerSweep(context.Background())
		}
	}
}

func (s *Service) beforeOTATriggerSweep(ctx context.Context) {
	cutoff := time.Now().Add(-s.otaAttemptTimeout)
	stale, err := s.otaRepo.ListStaleActive(ctx, cutoff, 200)
	if err != nil {
		slog.Error("ota watchdog list stale", "error", err)
		return
	}

	for _, a := range stale {
		errorText := "OTA attempt timed out waiting for device progress"
		now := time.Now()
		_ = s.otaRepo.AppendLog(ctx, a.RequestID, "Watchdog timeout: no OTA progress within timeout window")
		_ = s.otaRepo.UpdateProgress(ctx, a.RequestID, OTAAttemptStatusTimedOut, a.ProgressPct, &errorText, &now)

		latest, err := s.otaRepo.GetByRequestID(ctx, a.RequestID)
		if err == nil {
			s.hub.Broadcast("ota.attempt.updated", latest)
		}

		_ = s.roomSvc.InsertActivityLog(ctx, &room.ActivityLog{
			RoomID:      a.RoomID,
			Timestamp:   now,
			Title:       "OTA Flash Timed Out",
			Description: errorText,
			BadgeText:   "Timeout",
			BadgeColor:  "bg-error-container text-on-error-container",
		})
	}
}

func (s *Service) ListOTAAttemptsByRoom(ctx context.Context, roomID string, limit int) ([]OTAAttempt, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.otaRepo.ListByRoom(ctx, roomID, limit)
}

func (s *Service) GetOTAAttemptByID(ctx context.Context, attemptID string) (*OTAAttempt, error) {
	return s.otaRepo.GetByID(ctx, attemptID)
}

func (s *Service) HandleCommandAck(_ context.Context, deviceID string, ack map[string]any) error {
	slog.Info("received command ack from device", "device_id", deviceID, "ack", ack)
	requestID, _ := ack["requestId"].(string)
	if requestID == "" {
		requestID, _ = ack["request_id"].(string)
	}
	if requestID == "" {
		return nil
	}

	message, _ := ack["message"].(string)
	ok, _ := ack["ok"].(bool)

	if message != "" {
		_ = s.otaRepo.AppendLog(context.Background(), requestID, "ACK: "+message)
	}

	attempt, _ := s.otaRepo.GetByRequestID(context.Background(), requestID)

	if ok {
		nextStatus := OTAAttemptStatusAcknowledged
		nextProgress := 5
		if attempt != nil && attempt.Status == OTAAttemptStatusCancelling {
			nextStatus = OTAAttemptStatusCancelled
			nextProgress = attempt.ProgressPct
		}
		finishedAt := (*time.Time)(nil)
		if nextStatus == OTAAttemptStatusCancelled {
			now := time.Now()
			finishedAt = &now
		}
		if err := s.otaRepo.UpdateProgress(context.Background(), requestID, nextStatus, nextProgress, nil, finishedAt); err != nil {
			return err
		}
	} else {
		errorText := message
		if errorText == "" {
			errorText = "device rejected command"
		}
		finishedAt := time.Now()
		if err := s.otaRepo.UpdateProgress(context.Background(), requestID, OTAAttemptStatusFailed, 0, &errorText, &finishedAt); err != nil {
			return err
		}
	}

	attempt, err := s.otaRepo.GetByRequestID(context.Background(), requestID)
	if err == nil {
		s.hub.Broadcast("ota.attempt.updated", attempt)
		if !ok {
			_ = s.roomSvc.InsertActivityLog(context.Background(), &room.ActivityLog{
				RoomID:      attempt.RoomID,
				Timestamp:   time.Now(),
				Title:       "OTA Flash Failed",
				Description: message,
				BadgeText:   "Failed",
				BadgeColor:  "bg-error-container text-on-error-container",
			})
		}
		if ok && attempt.Status == OTAAttemptStatusCancelled {
			_ = s.otaRepo.AppendLog(context.Background(), requestID, "ACK: OTA cancellation confirmed")
			_ = s.roomSvc.InsertActivityLog(context.Background(), &room.ActivityLog{
				RoomID:      attempt.RoomID,
				Timestamp:   time.Now(),
				Title:       "OTA Flash Cancelled",
				Description: "OTA cancelled by user",
				BadgeText:   "Cancelled",
				BadgeColor:  "bg-secondary-container text-on-secondary-container",
			})
		}
	}

	_ = deviceID
	return nil
}

func mapOTAStatus(raw string) OTAAttemptStatus {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	if strings.Contains(s, "timed_out") || strings.Contains(s, "timeout") {
		return OTAAttemptStatusTimedOut
	}
	if strings.Contains(s, "cancelled") || strings.Contains(s, "canceled") {
		return OTAAttemptStatusCancelled
	}
	if strings.Contains(s, "cancelling") || strings.Contains(s, "canceling") {
		return OTAAttemptStatusCancelling
	}
	if strings.Contains(s, "download") {
		return OTAAttemptStatusDownloading
	}
	if strings.Contains(s, "verif") {
		return OTAAttemptStatusVerifying
	}
	if strings.Contains(s, "updat") || strings.Contains(s, "flash") {
		return OTAAttemptStatusUpdating
	}
	if strings.Contains(s, "done") || strings.Contains(s, "success") || strings.Contains(s, "updated") || strings.Contains(s, "reboot") {
		return OTAAttemptStatusUpdated
	}
	if strings.Contains(s, "fail") || strings.Contains(s, "error") {
		return OTAAttemptStatusFailed
	}
	return OTAAttemptStatusDownloading
}

func (s *Service) HandleOTAStatus(_ context.Context, deviceID string, payload map[string]any) error {
	slog.Info("received ota status from device", "device_id", deviceID, "payload", payload)
	requestID, _ := payload["requestId"].(string)
	if requestID == "" {
		return nil
	}

	statusRaw, _ := payload["status"].(string)
	status := mapOTAStatus(statusRaw)

	progress := 0
	switch v := payload["progress"].(type) {
	case float64:
		progress = int(v)
	case int:
		progress = v
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}

	var errorText *string
	if v, ok := payload["error"].(string); ok && strings.TrimSpace(v) != "" {
		t := strings.TrimSpace(v)
		errorText = &t
	}

	finishedAt := (*time.Time)(nil)
	if isOTATerminalStatus(status) {
		now := time.Now()
		finishedAt = &now
	}

	line := ""
	if v, ok := payload["log"].(string); ok {
		line = strings.TrimSpace(v)
	}
	if line == "" {
		if v, ok := payload["message"].(string); ok {
			line = strings.TrimSpace(v)
		}
	}
	if line != "" {
		_ = s.otaRepo.AppendLog(context.Background(), requestID, line)
	}

	if err := s.otaRepo.UpdateProgress(context.Background(), requestID, status, progress, errorText, finishedAt); err != nil {
		return err
	}

	attempt, err := s.otaRepo.GetByRequestID(context.Background(), requestID)
	if err == nil {
		s.hub.Broadcast("ota.attempt.updated", attempt)
		if status == OTAAttemptStatusUpdated {
			_ = s.roomSvc.InsertActivityLog(context.Background(), &room.ActivityLog{
				RoomID:      attempt.RoomID,
				Timestamp:   time.Now(),
				Title:       "OTA Flash Complete",
				Description: "Device updated and rebooted successfully",
				BadgeText:   "Success",
				BadgeColor:  "bg-tertiary-fixed text-on-tertiary-fixed",
			})
		}
		if status == OTAAttemptStatusFailed {
			detail := "OTA update failed"
			if errorText != nil {
				detail = *errorText
			}
			_ = s.roomSvc.InsertActivityLog(context.Background(), &room.ActivityLog{
				RoomID:      attempt.RoomID,
				Timestamp:   time.Now(),
				Title:       "OTA Flash Failed",
				Description: detail,
				BadgeText:   "Failed",
				BadgeColor:  "bg-error-container text-on-error-container",
			})
		}
		if status == OTAAttemptStatusCancelled {
			_ = s.roomSvc.InsertActivityLog(context.Background(), &room.ActivityLog{
				RoomID:      attempt.RoomID,
				Timestamp:   time.Now(),
				Title:       "OTA Flash Cancelled",
				Description: "OTA cancelled by user",
				BadgeText:   "Cancelled",
				BadgeColor:  "bg-secondary-container text-on-secondary-container",
			})
		}
		if status == OTAAttemptStatusTimedOut {
			_ = s.roomSvc.InsertActivityLog(context.Background(), &room.ActivityLog{
				RoomID:      attempt.RoomID,
				Timestamp:   time.Now(),
				Title:       "OTA Flash Timed Out",
				Description: "No OTA progress before timeout window",
				BadgeText:   "Timeout",
				BadgeColor:  "bg-error-container text-on-error-container",
			})
		}
	}

	return nil
}

func (s *Service) HandleStateUpdate(ctx context.Context, deviceID string, state map[string]any) error {
	d, err := s.repo.GetByID(ctx, deviceID)
	if err != nil {
		return err
	}

	onlineStr, ok := state["online"].(bool)
	isOnline := false
	if ok && onlineStr {
		isOnline = true
	}

	if d.Online != isOnline && d.RoomID != "" {
		title := "Device lost connection"
		description := fmt.Sprintf("%s recovered automatically", d.Name)
		badge := "Recovered"
		badgeColor := "bg-error-container text-on-error-container"

		if isOnline {
			title = "Device connected"
			description = fmt.Sprintf("%s was discovered online", d.Name)
			badge = "Online"
			badgeColor = "bg-tertiary-fixed text-on-tertiary-fixed"
		} else {
			title = "Device lost connection"
			description = fmt.Sprintf("%s went offline", d.Name)
			badge = "Offline"
			badgeColor = "bg-error-container text-on-error-container"
		}

		_ = s.roomSvc.InsertActivityLog(ctx, &room.ActivityLog{
			RoomID:      d.RoomID,
			Timestamp:   time.Now(),
			Title:       title,
			Description: description,
			BadgeText:   badge,
			BadgeColor:  badgeColor,
		})
	}

	d.State = state
	d.Online = isOnline
	d.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, d); err != nil {
		return err
	}

	if relaysAny, ok := state["relays"]; ok {
		if relaysArray, ok := relaysAny.([]any); ok {
			appliances, err := s.repo.ListAppliancesByDevice(ctx, deviceID)
			if err == nil {
				for _, app := range appliances {
					if app.Channel != nil && *app.Channel < len(relaysArray) && *app.Channel >= 0 {
						if isItOn, isValid := relaysArray[*app.Channel].(bool); isValid {
							if app.State == nil {
								app.State = make(map[string]any)
							}
							if currentOn, exists := app.State["isOn"].(bool); !exists || currentOn != isItOn {
								app.State["isOn"] = isItOn
								app.UpdatedAt = time.Now()
								_ = s.repo.UpdateAppliance(ctx, &app)
								s.hub.Broadcast("appliance.state", map[string]any{
									"appliance_id": app.ID,
									"state":        app.State,
								})
							}
						}
					}
				}
			}
		}
	}

	s.hub.Broadcast("device.state", map[string]any{
		"device_id": deviceID,
		"state":     state,
	})

	return nil
}

func (s *Service) HandleTelemetry(ctx context.Context, deviceID string, timestamp time.Time, data map[string]any) error {
	tp := &TelemetryPoint{
		DeviceID:  deviceID,
		Timestamp: timestamp,
		Data:      data,
	}
	return s.repo.InsertTelemetry(ctx, tp)
}

func (s *Service) GetRecentTelemetry(ctx context.Context, deviceID string, limit int) ([]TelemetryPoint, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	return s.repo.GetRecentTelemetry(ctx, deviceID, limit)
}

func (s *Service) CreateAppliance(ctx context.Context, req CreateApplianceRequest) (*Appliance, error) {
	a := &Appliance{
		ID:        uuid.NewString(),
		DeviceID:  req.DeviceID,
		RoomID:    req.RoomID,
		Name:      req.Name,
		Type:      req.Type,
		Channel:   req.Channel,
		GPIOPin:   req.GPIOPin,
		ActiveLow: req.ActiveLow,
		State:     make(map[string]any),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.CreateAppliance(ctx, a); err != nil {
		return nil, fmt.Errorf("create appliance: %w", err)
	}
	return a, nil
}

func (s *Service) GetAppliance(ctx context.Context, id string) (*Appliance, error) {
	return s.repo.GetApplianceByID(ctx, id)
}

func (s *Service) ListAppliancesByRoom(ctx context.Context, roomID string) ([]Appliance, error) {
	return s.repo.ListAppliancesByRoom(ctx, roomID)
}

func (s *Service) ListAppliancesByDevice(ctx context.Context, deviceID string) ([]Appliance, error) {
	return s.repo.ListAppliancesByDevice(ctx, deviceID)
}

func (s *Service) ListAllAppliances(ctx context.Context) ([]Appliance, error) {
	return s.repo.ListAllAppliances(ctx)
}

func (s *Service) UpdateAppliance(ctx context.Context, id string, req UpdateApplianceRequest) (*Appliance, error) {
	a, err := s.repo.GetApplianceByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		a.Name = *req.Name
	}
	if req.Channel != nil {
		a.Channel = req.Channel
	}
	if req.GPIOPin != nil {
		a.GPIOPin = req.GPIOPin
	}
	if req.ActiveLow != nil {
		a.ActiveLow = *req.ActiveLow
	}
	a.UpdatedAt = time.Now()

	if err := s.repo.UpdateAppliance(ctx, a); err != nil {
		return nil, fmt.Errorf("update appliance: %w", err)
	}
	return a, nil
}

func (s *Service) DeleteAppliance(ctx context.Context, id string) error {
	return s.repo.DeleteAppliance(ctx, id)
}
