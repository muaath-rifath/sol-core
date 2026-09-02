package home

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/muaathrifath/sol-core/internal/platform"
	"github.com/muaathrifath/sol-core/internal/user"
	"github.com/redis/go-redis/v9"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
	ErrConflict   = errors.New("conflict")
	ErrValidation = errors.New("validation error")
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func validateEmail(email string) error {
	if email == "" || !emailRegex.MatchString(email) {
		return fmt.Errorf("%w: invalid email address", ErrValidation)
	}
	return nil
}

func validateName(name string) error {
	name = strings.TrimSpace(name)
	if len(name) == 0 {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	if len(name) > 100 {
		return fmt.Errorf("%w: name must be 100 characters or fewer", ErrValidation)
	}
	return nil
}

const inviteKeyPrefix = "sol:invite:"
const inviteTTL = 7 * 24 * time.Hour

const defaultLimit = 20
const maxLimit = 100

func normalizeLimit(limit int) int {
	if limit < 1 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

// encodeCursor encodes a (timestamp, id) pair as a URL-safe base64 string.
func encodeCursor(t time.Time, id string) string {
	raw := t.UTC().Format(time.RFC3339Nano) + "|" + id
	return base64.URLEncoding.EncodeToString([]byte(raw))
}

// decodeCursor decodes a cursor string back into (time, id).
// Returns (zero, "", nil) for an empty cursor (first page).
func decodeCursor(s string) (*time.Time, string, error) {
	if s == "" {
		return nil, "", nil
	}
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return nil, "", fmt.Errorf("%w: invalid cursor", ErrValidation)
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return nil, "", fmt.Errorf("%w: invalid cursor format", ErrValidation)
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, "", fmt.Errorf("%w: invalid cursor timestamp", ErrValidation)
	}
	return &t, parts[1], nil
}

// buildCursorResponse constructs a CursorResponse from a slice that was fetched
// with limit+1 rows. The cursorFn extracts (time, id) from the last returned item.
func buildCursorResponse[T any](items []T, limit int, cursorFn func(T) (time.Time, string)) *CursorResponse[T] {
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	resp := &CursorResponse[T]{Data: items, HasMore: hasMore}
	if hasMore && len(items) > 0 {
		t, id := cursorFn(items[len(items)-1])
		encoded := encodeCursor(t, id)
		resp.NextCursor = &encoded
	}
	return resp
}

func (s *Service) setInviteToken(ctx context.Context, token, inviteID string) {
	if s.rdb == nil || token == "" {
		return
	}
	if err := s.rdb.Set(ctx, inviteKeyPrefix+token, inviteID, inviteTTL).Err(); err != nil {
		slog.Warn("failed to store invite token in redis", "invite_id", inviteID, "error", err)
	}
}

func (s *Service) deleteInviteToken(ctx context.Context, token string) {
	if s.rdb == nil || token == "" {
		return
	}
	if err := s.rdb.Del(ctx, inviteKeyPrefix+token).Err(); err != nil {
		slog.Warn("failed to delete invite token from redis", "token", token, "error", err)
	}
}

func (s *Service) checkInviteTokenCache(ctx context.Context, token string) {
	if s.rdb == nil || token == "" {
		return
	}
	if err := s.rdb.Get(ctx, inviteKeyPrefix+token).Err(); err != nil && !errors.Is(err, redis.Nil) {
		slog.Warn("redis unavailable for invite lookup, falling back to db", "error", err)
	}
}

type Service struct {
	repo     *Repository
	userRepo *user.Repository
	rdb      *redis.Client
	brevo    *platform.BrevoClient // nil = email disabled
	frontend string
}

func NewService(repo *Repository, userRepo *user.Repository, rdb *redis.Client, brevo *platform.BrevoClient, frontendURL string) *Service {
	return &Service{
		repo:     repo,
		userRepo: userRepo,
		rdb:      rdb,
		brevo:    brevo,
		frontend: frontendURL,
	}
}

func (s *Service) CreateHome(ctx context.Context, ownerID, name string) (*Home, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	now := time.Now()
	h := &Home{
		ID:        uuid.NewString(),
		Name:      name,
		OwnerID:   ownerID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, h); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("%w: a home with this name already exists", ErrConflict)
		}
		return nil, err
	}
	m := &Member{
		HomeID:   h.ID,
		UserID:   ownerID,
		Role:     RoleOwner,
		JoinedAt: now,
	}
	if err := s.repo.AddMember(ctx, m); err != nil {
		return nil, fmt.Errorf("add owner as member: %w", err)
	}
	h.MyRole = RoleOwner
	h.MemberCount = 1
	return h, nil
}

func (s *Service) GetHome(ctx context.Context, userID, homeID string) (*Home, error) {
	if userID == "" || homeID == "" {
		return nil, ErrForbidden
	}
	h, err := s.repo.GetByIDForUser(ctx, homeID, userID)
	if err != nil {
		return nil, ErrForbidden
	}
	return h, nil
}

func (s *Service) ListHomes(ctx context.Context, userID, cursor string, limit int) (*CursorResponse[Home], error) {
	limit = normalizeLimit(limit)
	cursorTime, cursorID, err := decodeCursor(cursor)
	if err != nil {
		return nil, err
	}
	homes, err := s.repo.ListByUserID(ctx, userID, cursorTime, cursorID, limit+1)
	if err != nil {
		return nil, err
	}
	return buildCursorResponse(homes, limit, func(h Home) (time.Time, string) {
		return h.CreatedAt, h.ID
	}), nil
}

func (s *Service) UpdateHome(ctx context.Context, userID, homeID, name string) (*Home, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	member, err := s.repo.GetMember(ctx, homeID, userID)
	if err != nil {
		return nil, ErrForbidden
	}
	if member.Role != RoleOwner && member.Role != RoleAdmin {
		return nil, ErrForbidden
	}
	h, err := s.repo.GetByID(ctx, homeID)
	if err != nil {
		return nil, ErrNotFound
	}
	h.Name = name
	h.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, h); err != nil {
		return nil, err
	}
	return h, nil
}

func (s *Service) DeleteHome(ctx context.Context, userID, homeID string) error {
	member, err := s.repo.GetMember(ctx, homeID, userID)
	if err != nil {
		return ErrForbidden
	}
	if member.Role != RoleOwner {
		return ErrForbidden
	}
	return s.repo.Delete(ctx, homeID)
}

func (s *Service) ListMembers(ctx context.Context, userID, homeID, cursor string, limit int) (*CursorResponse[Member], error) {
	if _, err := s.repo.GetMember(ctx, homeID, userID); err != nil {
		return nil, ErrForbidden
	}
	limit = normalizeLimit(limit)
	cursorTime, cursorID, err := decodeCursor(cursor)
	if err != nil {
		return nil, err
	}
	members, err := s.repo.ListMembers(ctx, homeID, cursorTime, cursorID, limit+1)
	if err != nil {
		return nil, err
	}
	return buildCursorResponse(members, limit, func(m Member) (time.Time, string) {
		return m.JoinedAt, m.UserID
	}), nil
}

func (s *Service) AddMember(ctx context.Context, actorID, homeID, targetUserID string, role MemberRole) (*Member, error) {
	actor, err := s.repo.GetMember(ctx, homeID, actorID)
	if err != nil {
		return nil, ErrForbidden
	}
	if actor.Role != RoleOwner && actor.Role != RoleAdmin {
		return nil, ErrForbidden
	}
	if role == RoleOwner {
		return nil, ErrForbidden
	}
	// Verify target user exists in our system
	if _, err := s.userRepo.GetByID(ctx, targetUserID); err != nil {
		return nil, ErrNotFound
	}
	// Reject if already a member
	if _, err := s.repo.GetMember(ctx, homeID, targetUserID); err == nil {
		return nil, fmt.Errorf("%w: user is already a member of this home", ErrConflict)
	}
	m := &Member{
		HomeID:    homeID,
		UserID:    targetUserID,
		Role:      role,
		InvitedBy: &actorID,
		JoinedAt:  time.Now(),
	}
	if err := s.repo.AddMember(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Service) UpdateMemberRole(ctx context.Context, actorID, homeID, targetUserID string, role MemberRole) error {
	actor, err := s.repo.GetMember(ctx, homeID, actorID)
	if err != nil {
		return ErrForbidden
	}
	if actor.Role != RoleOwner {
		return ErrForbidden
	}
	if role == RoleOwner {
		// Ownership transfer must go through TransferOwnership
		return ErrForbidden
	}
	target, err := s.repo.GetMember(ctx, homeID, targetUserID)
	if err != nil {
		return ErrNotFound
	}
	if target.Role == RoleOwner {
		return ErrForbidden
	}
	return s.repo.UpdateMemberRole(ctx, homeID, targetUserID, role)
}

func (s *Service) RemoveMember(ctx context.Context, actorID, homeID, targetUserID string) error {
	actor, err := s.repo.GetMember(ctx, homeID, actorID)
	if err != nil {
		return ErrForbidden
	}
	// Can remove self (leave), or owner/admin can remove others
	if actorID != targetUserID {
		if actor.Role != RoleOwner && actor.Role != RoleAdmin {
			return ErrForbidden
		}
	}
	target, err := s.repo.GetMember(ctx, homeID, targetUserID)
	if err != nil {
		return ErrNotFound
	}
	if target.Role == RoleOwner {
		// Owner cannot be removed; they must transfer ownership first
		return ErrForbidden
	}
	return s.repo.RemoveMember(ctx, homeID, targetUserID)
}

// TransferOwnership hands ownership of homeID from ownerID to newOwnerID.
// The old owner becomes an admin. newOwnerID must already be a member.
func (s *Service) TransferOwnership(ctx context.Context, ownerID, homeID, newOwnerID string) error {
	if ownerID == "" || homeID == "" || newOwnerID == "" {
		return ErrValidation
	}
	if ownerID == newOwnerID {
		return fmt.Errorf("%w: cannot transfer ownership to yourself", ErrConflict)
	}
	actor, err := s.repo.GetMember(ctx, homeID, ownerID)
	if err != nil {
		return ErrForbidden
	}
	if actor.Role != RoleOwner {
		return ErrForbidden
	}
	if _, err := s.repo.GetMember(ctx, homeID, newOwnerID); err != nil {
		return fmt.Errorf("%w: target user is not a member of this home", ErrNotFound)
	}
	return s.repo.TransferOwnership(ctx, homeID, ownerID, newOwnerID)
}

// GetInvitation returns the public details of an invitation by token.
// No authentication required — the token is the secret.
// Returns ErrNotFound only when the token doesn't exist.
func (s *Service) GetInvitation(ctx context.Context, token string) (*InvitationDetail, error) {
	if token == "" {
		return nil, ErrNotFound
	}
	s.checkInviteTokenCache(ctx, token)

	detail, err := s.repo.GetInvitationDetail(ctx, token)
	if err != nil {
		return nil, ErrNotFound
	}
	if detail.Status == StatusPending && time.Now().After(detail.ExpiresAt) {
		_ = s.repo.UpdateInvitationStatus(ctx, detail.ID, StatusExpired)
		detail.Status = StatusExpired
		s.deleteInviteToken(ctx, token)
	}
	return detail, nil
}

func (s *Service) InviteByEmail(ctx context.Context, actorID, homeID, email string) (*Invitation, error) {
	if err := validateEmail(email); err != nil {
		return nil, err
	}
	email = strings.ToLower(strings.TrimSpace(email))

	if err := s.repo.ExpireOldInvitations(ctx); err != nil {
		return nil, fmt.Errorf("expire old invitations: %w", err)
	}

	actor, err := s.repo.GetMember(ctx, homeID, actorID)
	if err != nil {
		return nil, ErrForbidden
	}
	if actor.Role != RoleOwner && actor.Role != RoleAdmin {
		return nil, ErrForbidden
	}

	// Check if invitee is a registered user and, if so, whether they're already a member
	var inviteeIsUser bool
	existing, lookupErr := s.userRepo.GetByEmail(ctx, email)
	if lookupErr == nil && existing != nil {
		inviteeIsUser = true
		if _, memberErr := s.repo.GetMember(ctx, homeID, existing.ID); memberErr == nil {
			return nil, fmt.Errorf("%w: user is already a member of this home", ErrConflict)
		}
	}

	// Check for an existing pending invitation
	hasPending, err := s.repo.HasPendingInvitation(ctx, homeID, email)
	if err != nil {
		return nil, fmt.Errorf("check pending invitation: %w", err)
	}
	if hasPending {
		return nil, fmt.Errorf("%w: a pending invitation already exists for this email", ErrConflict)
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	now := time.Now()
	inv := &Invitation{
		ID:           uuid.NewString(),
		HomeID:       homeID,
		InviterID:    actorID,
		InviteeEmail: email,
		Token:        hex.EncodeToString(tokenBytes),
		Status:       StatusPending,
		ExpiresAt:    now.Add(inviteTTL),
		CreatedAt:    now,
	}
	if err := s.repo.CreateInvitation(ctx, inv); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("%w: a pending invitation already exists for this email", ErrConflict)
		}
		return nil, err
	}
	inv.InviteeIsUser = inviteeIsUser

	// Store token in Redis for fast TTL-based expiry checks
	s.setInviteToken(ctx, inv.Token, inv.ID)

	// Send invitation email (non-fatal — invite is already persisted)
	if s.brevo != nil {
		inviterUser, _ := s.userRepo.GetByID(ctx, actorID)
		home, _ := s.repo.GetByID(ctx, homeID)
		inviterName := actorID
		if inviterUser != nil && inviterUser.Name != "" {
			inviterName = inviterUser.Name
		}
		homeName := homeID
		if home != nil {
			homeName = home.Name
		}
		inviteURL := fmt.Sprintf("%s/invitations/%s", strings.TrimRight(s.frontend, "/"), inv.Token)
		if emailErr := s.brevo.SendInvitationEmail(ctx, email, "", inviterName, homeName, inviteURL); emailErr != nil {
			slog.Warn("failed to send invitation email", "invite_id", inv.ID, "error", emailErr)
		}
	}

	return inv, nil
}

// ListInvitations returns a cursor-paginated list of invitations for a home.
// statusFilter can be "pending", "accepted", "declined", "expired", or "" for all.
func (s *Service) ListInvitations(ctx context.Context, actorID, homeID, statusFilter, cursor string, limit int) (*CursorResponse[Invitation], error) {
	actor, err := s.repo.GetMember(ctx, homeID, actorID)
	if err != nil {
		return nil, ErrForbidden
	}
	if actor.Role != RoleOwner && actor.Role != RoleAdmin {
		return nil, ErrForbidden
	}
	// Validate status filter if provided
	if statusFilter != "" {
		switch InvitationStatus(statusFilter) {
		case StatusPending, StatusAccepted, StatusDeclined, StatusExpired:
		default:
			return nil, fmt.Errorf("%w: status must be pending, accepted, declined, or expired", ErrValidation)
		}
	}

	limit = normalizeLimit(limit)
	cursorTime, cursorID, err := decodeCursor(cursor)
	if err != nil {
		return nil, err
	}
	invitations, err := s.repo.ListByHome(ctx, homeID, statusFilter, cursorTime, cursorID, limit+1)
	if err != nil {
		return nil, err
	}
	return buildCursorResponse(invitations, limit, func(inv Invitation) (time.Time, string) {
		return inv.CreatedAt, inv.ID
	}), nil
}

func (s *Service) CancelInvitation(ctx context.Context, actorID, homeID, invID string) error {
	actor, err := s.repo.GetMember(ctx, homeID, actorID)
	if err != nil {
		return ErrForbidden
	}
	if actor.Role != RoleOwner && actor.Role != RoleAdmin {
		return ErrForbidden
	}
	inv, err := s.repo.GetInvitationByID(ctx, invID)
	if err != nil {
		return ErrNotFound
	}
	if inv.HomeID != homeID {
		return ErrForbidden
	}
	if inv.Status != StatusPending {
		return ErrConflict
	}
	if err := s.repo.UpdateInvitationStatus(ctx, invID, StatusExpired); err != nil {
		return err
	}
	// Remove from Redis so future accept/decline attempts fail immediately
	s.deleteInviteToken(ctx, inv.Token)
	return nil
}

func (s *Service) AcceptInvitation(ctx context.Context, userID, token string) (*Home, error) {
	if token == "" {
		return nil, ErrNotFound
	}
	s.checkInviteTokenCache(ctx, token)

	inv, err := s.repo.GetByToken(ctx, token)
	if err != nil {
		return nil, ErrNotFound
	}
	if inv.Status != StatusPending {
		return nil, ErrConflict
	}
	// Expiry check covers both the Redis-available and Redis-unavailable paths
	if time.Now().After(inv.ExpiresAt) {
		s.repo.UpdateInvitationStatus(ctx, inv.ID, StatusExpired)
		s.deleteInviteToken(ctx, token)
		return nil, ErrConflict
	}

	u, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		// Authenticated user missing from our DB is an internal inconsistency, not a 404
		return nil, fmt.Errorf("accept invitation: user not in db: %w", err)
	}
	if !strings.EqualFold(u.Email, inv.InviteeEmail) {
		return nil, ErrForbidden
	}

	m := &Member{
		HomeID:    inv.HomeID,
		UserID:    userID,
		Role:      RoleMember,
		InvitedBy: &inv.InviterID,
		JoinedAt:  time.Now(),
	}
	// Atomic: add member + mark accepted in one transaction
	if err := s.repo.AcceptInvitationTx(ctx, inv.ID, m); err != nil {
		return nil, err
	}
	s.deleteInviteToken(ctx, token)

	return s.repo.GetByID(ctx, inv.HomeID)
}

// DeclineInvitation declines an invitation by token. No authentication is required —
// the token (256-bit random secret) is sufficient proof. This allows non-registered
// users to decline without creating an account.
func (s *Service) DeclineInvitation(ctx context.Context, token string) error {
	if token == "" {
		return ErrNotFound
	}
	s.checkInviteTokenCache(ctx, token)

	inv, err := s.repo.GetByToken(ctx, token)
	if err != nil {
		return ErrNotFound
	}
	if inv.Status != StatusPending {
		return ErrConflict
	}
	if time.Now().After(inv.ExpiresAt) {
		s.repo.UpdateInvitationStatus(ctx, inv.ID, StatusExpired)
		s.deleteInviteToken(ctx, token)
		return ErrConflict
	}

	if err := s.repo.UpdateInvitationStatus(ctx, inv.ID, StatusDeclined); err != nil {
		return err
	}
	s.deleteInviteToken(ctx, token)
	return nil
}
