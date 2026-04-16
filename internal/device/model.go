package device

import "time"

type CursorResponse[T any] struct {
	Data       []T     `json:"data"`
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}

type Device struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Type       DeviceType        `json:"type"`
	RoomID     string            `json:"room_id,omitempty"`
	State      map[string]any    `json:"state"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	FirmwareID string            `json:"firmware_id,omitempty"`
	Online     bool              `json:"online"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

type DeviceType string

const (
	DeviceTypeLight  DeviceType = "light"
	DeviceTypeSwitch DeviceType = "switch"
	DeviceTypeSensor DeviceType = "sensor"
	DeviceTypeLock   DeviceType = "lock"
	DeviceTypeFan    DeviceType = "fan"
	DeviceTypeCustom DeviceType = "custom"
)

type Command struct {
	DeviceID string         `json:"device_id"`
	Action   string         `json:"action"`
	Params   map[string]any `json:"params,omitempty"`
}

type OTARequest struct {
	FirmwareVersionID string `json:"firmware_version_id"`
	URL               string `json:"url,omitempty"`
}

type TelemetryPoint struct {
	DeviceID  string         `json:"device_id"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data"`
}

type CreateDeviceRequest struct {
	Name     string            `json:"name"`
	Type     DeviceType        `json:"type"`
	RoomID   string            `json:"room_id,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type UpdateDeviceRequest struct {
	Name     *string            `json:"name,omitempty"`
	RoomID   *string            `json:"room_id,omitempty"`
	Metadata *map[string]string `json:"metadata,omitempty"`
}
