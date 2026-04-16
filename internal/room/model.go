package room

import "time"

type Room struct {
	ID        string         `json:"id"`
	HomeID    string         `json:"home_id"`
	Name      string         `json:"name"`
	Floor     *int           `json:"floor,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type CreateRoomRequest struct {
	Name  string `json:"name"`
	Floor *int   `json:"floor,omitempty"`
}

type UpdateRoomRequest struct {
	Name  *string `json:"name,omitempty"`
	Floor *int    `json:"floor,omitempty"`
}
