package permission

import (
	"errors"
	"time"
)

type ScopeType string

const (
	ScopeRoom      ScopeType = "room"
	ScopeDevice    ScopeType = "device"
	ScopeAppliance ScopeType = "appliance"
)

func (s ScopeType) Valid() bool {
	return s == ScopeRoom || s == ScopeDevice || s == ScopeAppliance
}

type Grant struct {
	ID        string
	HomeID    string
	UserID    string
	ScopeType ScopeType
	ScopeID   string
	GrantedAt time.Time
	GrantedBy *string
}

// ScopeRef is the wire format for both the PUT request body and individual
// rows produced by the GET tree response when serialized. It is also the
// canonical input shape for SetGrants.
type ScopeRef struct {
	Type ScopeType `json:"type"`
	ID   string    `json:"id"`
}

// PermissionTree is the GET response. AllAccess is true for owner/admin
// targets — UI uses it to render a banner instead of the editor.
type PermissionTree struct {
	HomeID    string     `json:"home_id"`
	UserID    string     `json:"user_id"`
	Role      string     `json:"role"`
	AllAccess bool       `json:"all_access"`
	Rooms     []TreeRoom `json:"rooms"`
}

type TreeRoom struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	GrantedDirectly  bool         `json:"granted_directly"`
	CanManageDevices bool         `json:"can_manage_devices"`
	Devices          []TreeDevice `json:"devices"`
}

type TreeDevice struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	GrantedDirectly bool            `json:"granted_directly"`
	Appliances      []TreeAppliance `json:"appliances"`
}

type TreeAppliance struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	Channel         *int   `json:"channel,omitempty"`
	GrantedDirectly bool   `json:"granted_directly"`
}

var (
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
	ErrValidation = errors.New("validation error")
)
