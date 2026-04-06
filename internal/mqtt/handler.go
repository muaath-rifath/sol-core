package mqtt

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"
)

type DeviceService interface {
	HandleStateUpdate(ctx context.Context, deviceID string, state map[string]any) error
	HandleTelemetry(ctx context.Context, deviceID string, timestamp time.Time, data map[string]any) error
}

type Broadcaster interface {
	Broadcast(eventType string, data any)
}

type Handler struct {
	deviceSvc   DeviceService
	broadcaster Broadcaster
}

func NewHandler(deviceSvc DeviceService, broadcaster Broadcaster) *Handler {
	return &Handler{deviceSvc: deviceSvc, broadcaster: broadcaster}
}

func (h *Handler) Handle(topic string, payload []byte) {
	ctx := context.Background()
	parts := strings.Split(topic, "/")

	// Expected topics:
	// sol/devices/{id}/state     - device state updates
	// sol/devices/{id}/telemetry - telemetry data
	if len(parts) < 4 || parts[0] != "sol" || parts[1] != "devices" {
		slog.Debug("ignoring unknown topic", "topic", topic)
		return
	}

	deviceID := parts[2]
	msgType := parts[3]

	switch msgType {
	case "state":
		var state map[string]any
		if err := json.Unmarshal(payload, &state); err != nil {
			slog.Error("unmarshal state", "error", err, "topic", topic)
			return
		}
		if err := h.deviceSvc.HandleStateUpdate(ctx, deviceID, state); err != nil {
			slog.Error("handle state update", "error", err, "device_id", deviceID)
		}

	case "telemetry":
		var data map[string]any
		if err := json.Unmarshal(payload, &data); err != nil {
			slog.Error("unmarshal telemetry", "error", err, "topic", topic)
			return
		}
		if err := h.deviceSvc.HandleTelemetry(ctx, deviceID, time.Now(), data); err != nil {
			slog.Error("handle telemetry", "error", err, "device_id", deviceID)
		}

	default:
		slog.Debug("unhandled message type", "type", msgType, "topic", topic)
	}
}
