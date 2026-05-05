package room

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrValidation = errors.New("validation error")

// PermissionGate is the subset of permission.Service used by the room package.
// Defined here (duck-typed, satisfied by *permission.Service) to keep the
// dependency graph one-directional — permission already imports room indirectly.
type PermissionGate interface {
	MemberRole(ctx context.Context, homeID, userID string) (string, error)
	ListAccessibleRoomIDs(ctx context.Context, homeID, userID string) (ids []string, allAccess bool, err error)
	CheckRoomAccess(ctx context.Context, userID, homeID, roomID string) (bool, error)
}

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

func encodeCursor(t time.Time, id string) string {
	raw := t.UTC().Format(time.RFC3339Nano) + "|" + id
	return base64.URLEncoding.EncodeToString([]byte(raw))
}

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
		return nil, "", fmt.Errorf("%w: invalid cursor", ErrValidation)
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, "", fmt.Errorf("%w: invalid cursor", ErrValidation)
	}
	return &t, parts[1], nil
}

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

type Service struct {
	repo     *Repository
	permGate PermissionGate
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) SetPermissionGate(g PermissionGate) { s.permGate = g }
func (s *Service) PermGate() PermissionGate            { return s.permGate }

func (s *Service) Create(ctx context.Context, homeID string, req CreateRoomRequest) (*Room, error) {
	return s.repo.Create(ctx, homeID, req)
}

func (s *Service) Get(ctx context.Context, id, homeID string) (*Room, error) {
	return s.repo.GetByID(ctx, id, homeID)
}

// ListAllByHome returns every room in a home (no pagination). Suitable for
// internal callers like the permission tree composer where the room count is
// bounded.
func (s *Service) ListAllByHome(ctx context.Context, homeID string) ([]Room, error) {
	return s.repo.ListByHome(ctx, homeID)
}

// ListFiltered is like List but restricts results to rooms whose IDs are in ids.
// Called for member-role users who only see their accessible rooms.
func (s *Service) ListFiltered(ctx context.Context, homeID string, ids []string, cursor string, limit int) (*CursorResponse[Room], error) {
	limit = normalizeLimit(limit)
	cursorTime, cursorID, err := decodeCursor(cursor)
	if err != nil {
		return nil, err
	}
	rooms, err := s.repo.ListByHomePaginatedFiltered(ctx, homeID, ids, cursorTime, cursorID, limit+1)
	if err != nil {
		return nil, err
	}
	return buildCursorResponse(rooms, limit, func(r Room) (time.Time, string) {
		return r.CreatedAt, r.ID
	}), nil
}

func (s *Service) List(ctx context.Context, homeID, cursor string, limit int) (*CursorResponse[Room], error) {
	limit = normalizeLimit(limit)
	cursorTime, cursorID, err := decodeCursor(cursor)
	if err != nil {
		return nil, err
	}
	rooms, err := s.repo.ListByHomePaginated(ctx, homeID, cursorTime, cursorID, limit+1)
	if err != nil {
		return nil, err
	}
	return buildCursorResponse(rooms, limit, func(r Room) (time.Time, string) {
		return r.CreatedAt, r.ID
	}), nil
}

func (s *Service) Update(ctx context.Context, id, homeID string, req UpdateRoomRequest) (*Room, error) {
	return s.repo.Update(ctx, id, homeID, req)
}

func (s *Service) Delete(ctx context.Context, id, homeID string) error {
	return s.repo.Delete(ctx, id, homeID)
}

func (s *Service) InsertActivityLog(ctx context.Context, log *ActivityLog) error {
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}
	return s.repo.InsertActivityLog(ctx, log)
}

func (s *Service) ListActivityLogs(ctx context.Context, id, homeID string, limit int) ([]ActivityLog, error) {
	// Let's verify the room belongs to the home before returning its logs.
	if _, err := s.Get(ctx, id, homeID); err != nil {
		return nil, err
	}
	limit = normalizeLimit(limit)
	logs, err := s.repo.ListActivityLogs(ctx, id, limit)
	if err != nil {
		return nil, err
	}
	if logs == nil {
		logs = []ActivityLog{}
	}
	return logs, nil
}
