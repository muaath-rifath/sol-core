package home

import "time"

type MemberRole string

const (
	RoleOwner  MemberRole = "owner"
	RoleAdmin  MemberRole = "admin"
	RoleMember MemberRole = "member"
)

type InvitationStatus string

const (
	StatusPending  InvitationStatus = "pending"
	StatusAccepted InvitationStatus = "accepted"
	StatusDeclined InvitationStatus = "declined"
	StatusExpired  InvitationStatus = "expired"
)

type Home struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	OwnerID   string    `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Member struct {
	HomeID    string     `json:"home_id"`
	UserID    string     `json:"user_id"`
	UserEmail string     `json:"user_email,omitempty"`
	UserName  string     `json:"user_name,omitempty"`
	Role      MemberRole `json:"role"`
	InvitedBy *string    `json:"invited_by,omitempty"`
	JoinedAt  time.Time  `json:"joined_at"`
}

type Invitation struct {
	ID           string           `json:"id"`
	HomeID       string           `json:"home_id"`
	InviterID    string           `json:"inviter_id"`
	InviteeEmail string           `json:"invitee_email"`
	Token        string           `json:"token,omitempty"`
	Status       InvitationStatus `json:"status"`
	ExpiresAt    time.Time        `json:"expires_at"`
	CreatedAt    time.Time        `json:"created_at"`
}
