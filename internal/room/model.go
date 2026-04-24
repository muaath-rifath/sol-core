package room

import "time"

type CursorResponse[T any] struct {
	Data       []T     `json:"data"`
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}

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

type ActivityLog struct {
	RoomID      string    `json:"room_id"`
	Timestamp   time.Time `json:"timestamp"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	BadgeText   string    `json:"badge_text"`
	BadgeColor  string    `json:"badge_color"`
}
