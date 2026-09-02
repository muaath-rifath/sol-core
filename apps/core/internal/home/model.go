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

// Home is the base home record.
type Home struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	OwnerID     string     `json:"owner_id"`
	MyRole      MemberRole `json:"my_role,omitempty"`   // populated in user-scoped queries
	MemberCount int        `json:"member_count,omitempty"` // populated in list/get queries
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// CursorResponse is the response envelope for all infinite-scroll list endpoints.
// NextCursor is nil when there are no more items.
type CursorResponse[T any] struct {
	Data       []T     `json:"data"`
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
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
	ID            string           `json:"id"`
	HomeID        string           `json:"home_id"`
	InviterID     string           `json:"inviter_id"`
	InviteeEmail  string           `json:"invitee_email"`
	InviteeIsUser bool             `json:"invitee_is_user"`
	Token         string           `json:"token,omitempty"`
	Status        InvitationStatus `json:"status"`
	ExpiresAt     time.Time        `json:"expires_at"`
	CreatedAt     time.Time        `json:"created_at"`
}

// InvitationDetail is the public view of an invitation (no auth required).
// Returned by GET /api/v1/invitations/{token}.
type InvitationDetail struct {
	ID            string           `json:"id"`
	HomeID        string           `json:"home_id"`
	HomeName      string           `json:"home_name"`
	InviterID     string           `json:"inviter_id"`
	InviterName   string           `json:"inviter_name"`
	InviteeEmail  string           `json:"invitee_email"`
	InviteeIsUser bool             `json:"invitee_is_user"`
	Status        InvitationStatus `json:"status"`
	ExpiresAt     time.Time        `json:"expires_at"`
	CreatedAt     time.Time        `json:"created_at"`
}
