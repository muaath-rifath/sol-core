package device

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/muaathrifath/sol-core/internal/mqtt"
	"github.com/muaathrifath/sol-core/internal/ws"
)

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

func (s *Service) ListByRoom(ctx context.Context, roomID string) ([]Device, error) {
	return s.repo.ListByRoom(ctx, roomID)
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
