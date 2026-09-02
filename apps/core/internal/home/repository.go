package home

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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

// GetByID fetches a home by ID without any user-scoped enrichment.
// Used internally (e.g. after accepting an invitation).
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

// GetByIDForUser fetches a home enriched with the requesting user's role and the
// total member count. Returns an error if the user is not a member.
func (r *Repository) GetByIDForUser(ctx context.Context, homeID, userID string) (*Home, error) {
	var h Home
	var role string
	err := r.pool.QueryRow(ctx,
		`SELECT h.id, h.name, h.owner_id, h.created_at, h.updated_at,
		        m.role,
		        (SELECT COUNT(*) FROM home_members WHERE home_id = h.id) AS member_count
		 FROM homes h
		 JOIN home_members m ON m.home_id = h.id AND m.user_id = $2
		 WHERE h.id = $1`, homeID, userID,
	).Scan(&h.ID, &h.Name, &h.OwnerID, &h.CreatedAt, &h.UpdatedAt, &role, &h.MemberCount)
	if err != nil {
		return nil, fmt.Errorf("get home for user: %w", err)
	}
	h.MyRole = MemberRole(role)
	return &h, nil
}

// ListByUserID returns homes the user is a member of, ordered by created_at DESC.
// Pass nil cursorTime for the first page. Fetches limit+1 rows so the caller
// can detect whether more pages exist without a separate COUNT query.
func (r *Repository) ListByUserID(ctx context.Context, userID string, cursorTime *time.Time, cursorID string, limit int) ([]Home, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT h.id, h.name, h.owner_id, h.created_at, h.updated_at,
		        m.role,
		        (SELECT COUNT(*) FROM home_members hm WHERE hm.home_id = h.id) AS member_count
		 FROM homes h
		 JOIN home_members m ON m.home_id = h.id AND m.user_id = $1
		 WHERE ($2::timestamptz IS NULL
		        OR h.created_at < $2
		        OR (h.created_at = $2 AND h.id::text < $3))
		 ORDER BY h.created_at DESC, h.id::text DESC
		 LIMIT $4`, userID, cursorTime, cursorID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list homes: %w", err)
	}
	defer rows.Close()

	var homes []Home
	for rows.Next() {
		var h Home
		var role string
		if err := rows.Scan(&h.ID, &h.Name, &h.OwnerID, &h.CreatedAt, &h.UpdatedAt, &role, &h.MemberCount); err != nil {
			return nil, fmt.Errorf("scan home: %w", err)
		}
		h.MyRole = MemberRole(role)
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

// ListMembers returns members ordered by joined_at ASC (oldest first — stable for
// infinite scroll since new members always append at the end).
// Pass nil cursorTime for the first page.
func (r *Repository) ListMembers(ctx context.Context, homeID string, cursorTime *time.Time, cursorID string, limit int) ([]Member, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT m.home_id, m.user_id, m.role, m.invited_by, m.joined_at,
		        u.email, u.name
		 FROM home_members m
		 JOIN users u ON u.id = m.user_id
		 WHERE m.home_id = $1
		   AND ($2::timestamptz IS NULL
		        OR m.joined_at > $2
		        OR (m.joined_at = $2 AND m.user_id::text > $3))
		 ORDER BY m.joined_at ASC, m.user_id::text ASC
		 LIMIT $4`, homeID, cursorTime, cursorID, limit,
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

// ListByHome returns invitations ordered by created_at DESC, optionally filtered
// by status (pass empty string for all). Pass nil cursorTime for the first page.
func (r *Repository) ListByHome(ctx context.Context, homeID, statusFilter string, cursorTime *time.Time, cursorID string, limit int) ([]Invitation, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT i.id, i.home_id, i.inviter_id, i.invitee_email, i.status, i.expires_at, i.created_at,
		        EXISTS(SELECT 1 FROM users u WHERE LOWER(u.email) = LOWER(i.invitee_email)) AS invitee_is_user
		 FROM home_invitations i
		 WHERE i.home_id = $1
		   AND ($2 = '' OR status = $2)
		   AND ($3::timestamptz IS NULL
		        OR created_at < $3
		        OR (created_at = $3 AND id::text < $4))
		 ORDER BY created_at DESC, id::text DESC
		 LIMIT $5`, homeID, statusFilter, cursorTime, cursorID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	defer rows.Close()

	var invitations []Invitation
	for rows.Next() {
		var inv Invitation
		var status string
		if err := rows.Scan(&inv.ID, &inv.HomeID, &inv.InviterID, &inv.InviteeEmail,
			&status, &inv.ExpiresAt, &inv.CreatedAt, &inv.InviteeIsUser); err != nil {
			return nil, fmt.Errorf("scan invitation: %w", err)
		}
		inv.Status = InvitationStatus(status)
		// token intentionally omitted from list responses
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

// AcceptInvitationTx atomically adds a member and marks the invitation as accepted
// in a single database transaction. If the user is already a member (RowsAffected == 0
// on the insert), the invitation is still marked accepted — idempotent behaviour.
func (r *Repository) AcceptInvitationTx(ctx context.Context, invID string, m *Member) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("accept invitation begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`INSERT INTO home_members (home_id, user_id, role, invited_by, joined_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (home_id, user_id) DO NOTHING`,
		m.HomeID, m.UserID, string(m.Role), m.InvitedBy, m.JoinedAt,
	); err != nil {
		return fmt.Errorf("accept invitation add member: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE home_invitations SET status = 'accepted' WHERE id = $1`, invID,
	); err != nil {
		return fmt.Errorf("accept invitation update status: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("accept invitation commit: %w", err)
	}
	return nil
}

// GetInvitationDetail returns an enriched, public view of an invitation by token.
// It joins homes and users to include names, and checks whether the invitee email
// belongs to a registered user.
func (r *Repository) GetInvitationDetail(ctx context.Context, token string) (*InvitationDetail, error) {
	var d InvitationDetail
	var status string
	var inviterName sql.NullString
	err := r.pool.QueryRow(ctx,
		`SELECT i.id, i.home_id, h.name,
		        i.inviter_id, u.name,
		        i.invitee_email, i.status, i.expires_at, i.created_at,
		        EXISTS(SELECT 1 FROM users ux WHERE LOWER(ux.email) = LOWER(i.invitee_email)) AS invitee_is_user
		 FROM home_invitations i
		 JOIN homes h ON h.id = i.home_id
		 LEFT JOIN users u ON u.id = i.inviter_id
		 WHERE i.token = $1`, token,
	).Scan(
		&d.ID, &d.HomeID, &d.HomeName,
		&d.InviterID, &inviterName,
		&d.InviteeEmail, &status, &d.ExpiresAt, &d.CreatedAt,
		&d.InviteeIsUser,
	)
	if err != nil {
		return nil, fmt.Errorf("get invitation detail: %w", err)
	}
	if inviterName.Valid {
		d.InviterName = inviterName.String
	} else {
		d.InviterName = "A Sol member"
	}
	d.Status = InvitationStatus(status)
	return &d, nil
}

// HasPendingInvitation reports whether a pending invitation already exists for the
// given home + email combination.
func (r *Repository) HasPendingInvitation(ctx context.Context, homeID, email string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1 FROM home_invitations
		   WHERE home_id = $1
		     AND LOWER(invitee_email) = LOWER($2)
		     AND status = 'pending'
		     AND expires_at > now()
		 )`, homeID, email,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("has pending invitation: %w", err)
	}
	return exists, nil
}

// TransferOwnership atomically demotes fromUserID to admin, promotes toUserID to
// owner, and updates homes.owner_id — all within a single transaction.
func (r *Repository) TransferOwnership(ctx context.Context, homeID, fromUserID, toUserID string) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("transfer ownership begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`UPDATE home_members SET role = 'admin' WHERE home_id = $1 AND user_id = $2`,
		homeID, fromUserID,
	); err != nil {
		return fmt.Errorf("transfer ownership demote old owner: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE home_members SET role = 'owner' WHERE home_id = $1 AND user_id = $2`,
		homeID, toUserID,
	); err != nil {
		return fmt.Errorf("transfer ownership promote new owner: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE homes SET owner_id = $2, updated_at = now() WHERE id = $1`,
		homeID, toUserID,
	); err != nil {
		return fmt.Errorf("transfer ownership update home: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("transfer ownership commit: %w", err)
	}
	return nil
}
