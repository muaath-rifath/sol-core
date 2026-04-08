package home

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Home CRUD

func (r *Repository) Create(ctx context.Context, h *Home) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO homes (id, name, owner_id, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)`,
		h.ID, h.Name, h.OwnerID, h.CreatedAt, h.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create home: %w", err)
	}
	return nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*Home, error) {
	var h Home
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, owner_id, created_at, updated_at FROM homes WHERE id = $1`, id,
	).Scan(&h.ID, &h.Name, &h.OwnerID, &h.CreatedAt, &h.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get home: %w", err)
	}
	return &h, nil
}

func (r *Repository) ListByUserID(ctx context.Context, userID string) ([]Home, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT h.id, h.name, h.owner_id, h.created_at, h.updated_at
		 FROM homes h
		 JOIN home_members m ON m.home_id = h.id
		 WHERE m.user_id = $1
		 ORDER BY h.created_at DESC`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list homes: %w", err)
	}
	defer rows.Close()

	var homes []Home
	for rows.Next() {
		var h Home
		if err := rows.Scan(&h.ID, &h.Name, &h.OwnerID, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan home: %w", err)
		}
		homes = append(homes, h)
	}
	return homes, nil
}

func (r *Repository) Update(ctx context.Context, h *Home) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE homes SET name=$2, updated_at=$3 WHERE id=$1`,
		h.ID, h.Name, h.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update home: %w", err)
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM homes WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete home: %w", err)
	}
	return nil
}

// Members

func (r *Repository) AddMember(ctx context.Context, m *Member) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO home_members (home_id, user_id, role, invited_by, joined_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (home_id, user_id) DO NOTHING`,
		m.HomeID, m.UserID, string(m.Role), m.InvitedBy, m.JoinedAt,
	)
	if err != nil {
		return fmt.Errorf("add member: %w", err)
	}
	return nil
}

func (r *Repository) GetMember(ctx context.Context, homeID, userID string) (*Member, error) {
	var m Member
	var role string
	err := r.pool.QueryRow(ctx,
		`SELECT home_id, user_id, role, invited_by, joined_at
		 FROM home_members WHERE home_id = $1 AND user_id = $2`,
		homeID, userID,
	).Scan(&m.HomeID, &m.UserID, &role, &m.InvitedBy, &m.JoinedAt)
	if err != nil {
		return nil, fmt.Errorf("get member: %w", err)
	}
	m.Role = MemberRole(role)
	return &m, nil
}

func (r *Repository) ListMembers(ctx context.Context, homeID string) ([]Member, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT m.home_id, m.user_id, m.role, m.invited_by, m.joined_at,
		        u.email, u.name
		 FROM home_members m
		 JOIN users u ON u.id = m.user_id
		 WHERE m.home_id = $1
		 ORDER BY m.joined_at ASC`, homeID,
	)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()

	var members []Member
	for rows.Next() {
		var m Member
		var role string
		if err := rows.Scan(&m.HomeID, &m.UserID, &role, &m.InvitedBy, &m.JoinedAt, &m.UserEmail, &m.UserName); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		m.Role = MemberRole(role)
		members = append(members, m)
	}
	return members, nil
}

func (r *Repository) UpdateMemberRole(ctx context.Context, homeID, userID string, role MemberRole) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE home_members SET role=$3 WHERE home_id=$1 AND user_id=$2`,
		homeID, userID, string(role),
	)
	if err != nil {
		return fmt.Errorf("update member role: %w", err)
	}
	return nil
}

func (r *Repository) RemoveMember(ctx context.Context, homeID, userID string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM home_members WHERE home_id=$1 AND user_id=$2`,
		homeID, userID,
	)
	if err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	return nil
}

// Invitations

func (r *Repository) CreateInvitation(ctx context.Context, inv *Invitation) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO home_invitations (id, home_id, inviter_id, invitee_email, token, status, expires_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		inv.ID, inv.HomeID, inv.InviterID, inv.InviteeEmail, inv.Token,
		string(inv.Status), inv.ExpiresAt, inv.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create invitation: %w", err)
	}
	return nil
}

func (r *Repository) GetByToken(ctx context.Context, token string) (*Invitation, error) {
	var inv Invitation
	var status string
	err := r.pool.QueryRow(ctx,
		`SELECT id, home_id, inviter_id, invitee_email, token, status, expires_at, created_at
		 FROM home_invitations WHERE token = $1`, token,
	).Scan(&inv.ID, &inv.HomeID, &inv.InviterID, &inv.InviteeEmail, &inv.Token,
		&status, &inv.ExpiresAt, &inv.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get invitation by token: %w", err)
	}
	inv.Status = InvitationStatus(status)
	return &inv, nil
}

func (r *Repository) GetInvitationByID(ctx context.Context, id string) (*Invitation, error) {
	var inv Invitation
	var status string
	err := r.pool.QueryRow(ctx,
		`SELECT id, home_id, inviter_id, invitee_email, token, status, expires_at, created_at
		 FROM home_invitations WHERE id = $1`, id,
	).Scan(&inv.ID, &inv.HomeID, &inv.InviterID, &inv.InviteeEmail, &inv.Token,
		&status, &inv.ExpiresAt, &inv.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get invitation by id: %w", err)
	}
	inv.Status = InvitationStatus(status)
	return &inv, nil
}

func (r *Repository) ListByHome(ctx context.Context, homeID string) ([]Invitation, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, home_id, inviter_id, invitee_email, token, status, expires_at, created_at
		 FROM home_invitations WHERE home_id = $1 ORDER BY created_at DESC`, homeID,
	)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	defer rows.Close()

	var invitations []Invitation
	for rows.Next() {
		var inv Invitation
		var status string
		if err := rows.Scan(&inv.ID, &inv.HomeID, &inv.InviterID, &inv.InviteeEmail, &inv.Token,
			&status, &inv.ExpiresAt, &inv.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan invitation: %w", err)
		}
		inv.Status = InvitationStatus(status)
		inv.Token = "" // don't expose token in list
		invitations = append(invitations, inv)
	}
	return invitations, nil
}

func (r *Repository) UpdateInvitationStatus(ctx context.Context, id string, status InvitationStatus) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE home_invitations SET status=$2 WHERE id=$1`,
		id, string(status),
	)
	if err != nil {
		return fmt.Errorf("update invitation status: %w", err)
	}
	return nil
}

// ExpireOldInvitations marks pending invitations as expired if past their expiry time
func (r *Repository) ExpireOldInvitations(ctx context.Context) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE home_invitations SET status='expired'
		 WHERE status='pending' AND expires_at < $1`, time.Now(),
	)
	return err
}
