package home

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/muaathrifath/sol-core/internal/user"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
	ErrConflict   = errors.New("conflict")
)

type Service struct {
	repo     *Repository
	userRepo *user.Repository
}

func NewService(repo *Repository, userRepo *user.Repository) *Service {
	return &Service{repo: repo, userRepo: userRepo}
}

func (s *Service) CreateHome(ctx context.Context, ownerID, name string) (*Home, error) {
	now := time.Now()
	h := &Home{
		ID:        uuid.NewString(),
		Name:      name,
		OwnerID:   ownerID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, h); err != nil {
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
	return h, nil
}

func (s *Service) GetHome(ctx context.Context, userID, homeID string) (*Home, error) {
	if _, err := s.repo.GetMember(ctx, homeID, userID); err != nil {
		return nil, ErrForbidden
	}
	return s.repo.GetByID(ctx, homeID)
}

func (s *Service) ListHomes(ctx context.Context, userID string) ([]Home, error) {
	homes, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if homes == nil {
		homes = []Home{}
	}
	return homes, nil
}

func (s *Service) UpdateHome(ctx context.Context, userID, homeID, name string) (*Home, error) {
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

func (s *Service) ListMembers(ctx context.Context, userID, homeID string) ([]Member, error) {
	if _, err := s.repo.GetMember(ctx, homeID, userID); err != nil {
		return nil, ErrForbidden
	}
	members, err := s.repo.ListMembers(ctx, homeID)
	if err != nil {
		return nil, err
	}
	if members == nil {
		members = []Member{}
	}
	return members, nil
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
		return ErrForbidden
	}
	return s.repo.RemoveMember(ctx, homeID, targetUserID)
}

func (s *Service) InviteByEmail(ctx context.Context, actorID, homeID, email string) (*Invitation, error) {
	actor, err := s.repo.GetMember(ctx, homeID, actorID)
	if err != nil {
		return nil, ErrForbidden
	}
	if actor.Role != RoleOwner && actor.Role != RoleAdmin {
		return nil, ErrForbidden
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
		ExpiresAt:    now.Add(7 * 24 * time.Hour),
		CreatedAt:    now,
	}
	if err := s.repo.CreateInvitation(ctx, inv); err != nil {
		return nil, err
	}
	return inv, nil
}

func (s *Service) ListInvitations(ctx context.Context, actorID, homeID string) ([]Invitation, error) {
	actor, err := s.repo.GetMember(ctx, homeID, actorID)
	if err != nil {
		return nil, ErrForbidden
	}
	if actor.Role != RoleOwner && actor.Role != RoleAdmin {
		return nil, ErrForbidden
	}
	invitations, err := s.repo.ListByHome(ctx, homeID)
	if err != nil {
		return nil, err
	}
	if invitations == nil {
		invitations = []Invitation{}
	}
	return invitations, nil
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
	return s.repo.UpdateInvitationStatus(ctx, invID, StatusExpired)
}

func (s *Service) AcceptInvitation(ctx context.Context, userID, token string) (*Home, error) {
	if err := s.repo.ExpireOldInvitations(ctx); err != nil {
		return nil, err
	}
	inv, err := s.repo.GetByToken(ctx, token)
	if err != nil {
		return nil, ErrNotFound
	}
	if inv.Status != StatusPending {
		return nil, ErrConflict
	}
	u, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, ErrNotFound
	}
	if u.Email != inv.InviteeEmail {
		return nil, ErrForbidden
	}
	m := &Member{
		HomeID:    inv.HomeID,
		UserID:    userID,
		Role:      RoleMember,
		InvitedBy: &inv.InviterID,
		JoinedAt:  time.Now(),
	}
	if err := s.repo.AddMember(ctx, m); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateInvitationStatus(ctx, inv.ID, StatusAccepted); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, inv.HomeID)
}

func (s *Service) DeclineInvitation(ctx context.Context, userID, token string) error {
	inv, err := s.repo.GetByToken(ctx, token)
	if err != nil {
		return ErrNotFound
	}
	if inv.Status != StatusPending {
		return ErrConflict
	}
	u, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return ErrNotFound
	}
	if u.Email != inv.InviteeEmail {
		return ErrForbidden
	}
	return s.repo.UpdateInvitationStatus(ctx, inv.ID, StatusDeclined)
}
