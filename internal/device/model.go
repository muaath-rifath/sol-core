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
	DeviceTypeSwitch DeviceType = "switch"
)

type Command struct {
	DeviceID  string         `json:"device_id"`
	RequestID string         `json:"requestId,omitempty"`
	Action    string         `json:"action"`
	Params    map[string]any `json:"params,omitempty"`
}

type OTARequest struct {
	FirmwareVersionID string `json:"firmware_version_id"`
	IdempotencyKey    string `json:"idempotency_key,omitempty"`
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

type Appliance struct {
	ID        string         `json:"id"`
	DeviceID  string         `json:"device_id"`
	RoomID    string         `json:"room_id,omitempty"`
	Name      string         `json:"name"`
	Type      DeviceType     `json:"type"`
	Channel   *int           `json:"channel,omitempty"`
	GPIOPin   *int           `json:"gpio_pin,omitempty"`
	ActiveLow bool           `json:"active_low"`
	State     map[string]any `json:"state"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type CreateApplianceRequest struct {
	DeviceID  string     `json:"device_id"`
	RoomID    string     `json:"room_id,omitempty"`
	Name      string     `json:"name"`
	Type      DeviceType `json:"type"`
	Channel   *int       `json:"channel,omitempty"`
	GPIOPin   *int       `json:"gpio_pin,omitempty"`
	ActiveLow bool       `json:"active_low"`
}

type UpdateApplianceRequest struct {
	Name      *string `json:"name,omitempty"`
	Channel   *int    `json:"channel,omitempty"`
	GPIOPin   *int    `json:"gpio_pin,omitempty"`
	ActiveLow *bool   `json:"active_low,omitempty"`
}
