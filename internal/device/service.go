package device

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/muaathrifath/sol-core/internal/mqtt"
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
	repo *Repository
	mqtt *mqtt.Client
	hub  *ws.Hub
}

func NewService(repo *Repository, mqttClient *mqtt.Client, hub *ws.Hub) *Service {
	return &Service{repo: repo, mqtt: mqttClient, hub: hub}
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
	topic := fmt.Sprintf("sol/devices/%s/command", cmd.DeviceID)
	return s.mqtt.Publish(topic, cmd)
}

func (s *Service) TriggerOTA(ctx context.Context, deviceID string, url string) error {
	topic := fmt.Sprintf("sol/devices/%s/command", deviceID)
	return s.mqtt.Publish(topic, Command{
		DeviceID: deviceID,
		Action:   "ota_update",
		Params:   map[string]any{"url": url},
	})
}

func (s *Service) HandleStateUpdate(ctx context.Context, deviceID string, state map[string]any) error {
	d, err := s.repo.GetByID(ctx, deviceID)
	if err != nil {
		return err
	}

	d.State = state
	d.Online = true
	d.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, d); err != nil {
		return err
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
