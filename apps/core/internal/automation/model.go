package automation

import "time"

type Rule struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Enabled     bool      `json:"enabled"`
	Trigger     Trigger   `json:"trigger"`
	Conditions  []Cond    `json:"conditions,omitempty"`
	Actions     []Action  `json:"actions"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Trigger struct {
	Type   string         `json:"type"` // "device_state", "schedule", "event"
	Config map[string]any `json:"config"`
}

type Cond struct {
	Type   string         `json:"type"` // "device_state", "time_range", "ai_condition"
	Config map[string]any `json:"config"`
}

type Action struct {
	Type   string         `json:"type"` // "device_command", "notification", "ai_action"
	Config map[string]any `json:"config"`
}

type CreateRuleRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Trigger     Trigger  `json:"trigger"`
	Conditions  []Cond   `json:"conditions,omitempty"`
	Actions     []Action `json:"actions"`
}

type UpdateRuleRequest struct {
	Name        *string  `json:"name,omitempty"`
	Description *string  `json:"description,omitempty"`
	Enabled     *bool    `json:"enabled,omitempty"`
	Trigger     *Trigger `json:"trigger,omitempty"`
	Conditions  []Cond   `json:"conditions,omitempty"`
	Actions     []Action `json:"actions,omitempty"`
}
