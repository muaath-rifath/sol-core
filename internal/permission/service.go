package permission

import (
	"context"
	"errors"
	"fmt"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// CheckAppliance returns true if userID may see/control applianceID.
// Owner and admin always pass. Non-members get false. Members are checked
// against effective grants (appliance / device / room).
func (s *Service) CheckAppliance(ctx context.Context, userID, applianceID string) (bool, error) {
	homeID, deviceID, roomID, err := s.repo.GetApplianceContext(ctx, applianceID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, ErrNotFound
		}
		return false, err
	}

	role, err := s.repo.GetMemberRole(ctx, homeID, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	if role == "owner" || role == "admin" {
		return true, nil
	}

	return s.repo.HasEffectiveGrant(ctx, homeID, userID, roomID, deviceID, applianceID)
}

// CheckApplianceByChannel resolves (deviceID, channel) to an appliance and gates
// access. Used by command handlers where the request specifies a channel rather
// than an appliance ID. Returns the resolved applianceID alongside the decision.
func (s *Service) CheckApplianceByChannel(ctx context.Context, userID, deviceID string, channel int) (applianceID string, allowed bool, err error) {
	applianceID, homeID, err := s.repo.FindApplianceByDeviceChannel(ctx, deviceID, channel)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", false, ErrNotFound
		}
		return "", false, err
	}

	role, err := s.repo.GetMemberRole(ctx, homeID, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return applianceID, false, nil
		}
		return "", false, err
	}
	if role == "owner" || role == "admin" {
		return applianceID, true, nil
	}

	// We need the room ID for an effective-grant probe. Fetch via the appliance
	// context (cheap — already-cached in pgx prepared statements).
	_, _, roomID, err := s.repo.GetApplianceContext(ctx, applianceID)
	if err != nil {
		return "", false, err
	}
	ok, err := s.repo.HasEffectiveGrant(ctx, homeID, userID, roomID, deviceID, applianceID)
	if err != nil {
		return "", false, err
	}
	return applianceID, ok, nil
}

// ListAccessibleApplianceIDs returns the set of appliance IDs a user may see/control
// in a home. Owner/admin → (nil, true, nil) signaling "no filter". Members get the
// expanded list (may be empty).
func (s *Service) ListAccessibleApplianceIDs(ctx context.Context, homeID, userID string) (ids []string, allAccess bool, err error) {
	role, err := s.repo.GetMemberRole(ctx, homeID, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if role == "owner" || role == "admin" {
		return nil, true, nil
	}

	grants, err := s.repo.ListGrants(ctx, homeID, userID)
	if err != nil {
		return nil, false, err
	}
	if len(grants) == 0 {
		return []string{}, false, nil
	}
	expanded, err := s.repo.ExpandToApplianceIDs(ctx, homeID, grants)
	if err != nil {
		return nil, false, err
	}
	return expanded, false, nil
}

// MemberRole returns the caller's role in homeID, or ErrNotFound if not a member.
// Convenience for handlers that need to gate management operations.
func (s *Service) MemberRole(ctx context.Context, homeID, userID string) (string, error) {
	return s.repo.GetMemberRole(ctx, homeID, userID)
}

// DirectGrantsFor returns the three sets of directly-granted scope IDs.
// Used by the GET handler to annotate the tree response.
func (s *Service) DirectGrantsFor(ctx context.Context, homeID, userID string) (rooms, devices, appliances map[string]struct{}, err error) {
	grants, err := s.repo.ListGrants(ctx, homeID, userID)
	if err != nil {
		return nil, nil, nil, err
	}
	rooms = map[string]struct{}{}
	devices = map[string]struct{}{}
	appliances = map[string]struct{}{}
	for _, g := range grants {
		switch g.ScopeType {
		case ScopeRoom:
			rooms[g.ScopeID] = struct{}{}
		case ScopeDevice:
			devices[g.ScopeID] = struct{}{}
		case ScopeAppliance:
			appliances[g.ScopeID] = struct{}{}
		}
	}
	return rooms, devices, appliances, nil
}

// ListAccessibleRoomIDs returns the set of room IDs a user may see in a home.
// Owner/admin → (nil, true, nil) signaling "no filter". Members get the filtered list.
func (s *Service) ListAccessibleRoomIDs(ctx context.Context, homeID, userID string) (ids []string, allAccess bool, err error) {
	role, err := s.repo.GetMemberRole(ctx, homeID, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return []string{}, false, nil
		}
		return nil, false, err
	}
	if role == "owner" || role == "admin" {
		return nil, true, nil
	}
	ids, err = s.repo.ListAccessibleRoomIDs(ctx, homeID, userID)
	if err != nil {
		return nil, false, err
	}
	if ids == nil {
		ids = []string{}
	}
	return ids, false, nil
}

// ListAccessibleDeviceIDs returns the set of device IDs a user may see in a home.
// Owner/admin → (nil, true, nil). Members get the filtered list.
func (s *Service) ListAccessibleDeviceIDs(ctx context.Context, homeID, userID string) (ids []string, allAccess bool, err error) {
	role, err := s.repo.GetMemberRole(ctx, homeID, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return []string{}, false, nil
		}
		return nil, false, err
	}
	if role == "owner" || role == "admin" {
		return nil, true, nil
	}
	ids, err = s.repo.ListAccessibleDeviceIDs(ctx, homeID, userID)
	if err != nil {
		return nil, false, err
	}
	if ids == nil {
		ids = []string{}
	}
	return ids, false, nil
}

// CheckRoomAccess returns true if userID may see the given room.
// homeID is passed directly (already known from the URL param).
func (s *Service) CheckRoomAccess(ctx context.Context, userID, homeID, roomID string) (bool, error) {
	role, err := s.repo.GetMemberRole(ctx, homeID, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	if role == "owner" || role == "admin" {
		return true, nil
	}
	return s.repo.CheckRoomAccess(ctx, homeID, userID, roomID)
}

// CheckDevice returns true if userID may see/control deviceID.
// Resolves homeID and roomID internally (single query), then checks effective grants.
func (s *Service) CheckDevice(ctx context.Context, userID, deviceID string) (bool, error) {
	homeID, roomID, err := s.repo.GetDeviceContext(ctx, deviceID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, ErrNotFound
		}
		return false, err
	}
	role, err := s.repo.GetMemberRole(ctx, homeID, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	if role == "owner" || role == "admin" {
		return true, nil
	}
	return s.repo.HasEffectiveDeviceGrant(ctx, homeID, userID, roomID, deviceID)
}

// SetGrants replaces the grant set for (targetUserID) in homeID.
// Caller must be owner or admin of homeID. Target must be a 'member' (not owner/admin).
// Refs that don't belong to homeID are rejected as ErrValidation.
func (s *Service) SetGrants(ctx context.Context, callerID, homeID, targetUserID string, refs []ScopeRef) error {
	callerRole, err := s.repo.GetMemberRole(ctx, homeID, callerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrForbidden
		}
		return err
	}
	if callerRole != "owner" && callerRole != "admin" {
		return ErrForbidden
	}

	targetRole, err := s.repo.GetMemberRole(ctx, homeID, targetUserID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	if targetRole != "member" {
		// Owners and admins have all-access; refusing here protects the rule
		// "owner/admin can't be scoped" expressed in the data model.
		return fmt.Errorf("%w: target user role '%s' cannot be scoped", ErrForbidden, targetRole)
	}

	// Dedup by (type, id).
	dedup := make(map[ScopeRef]struct{}, len(refs))
	cleaned := make([]ScopeRef, 0, len(refs))
	for _, ref := range refs {
		if !ref.Type.Valid() || ref.ID == "" {
			return fmt.Errorf("%w: invalid scope ref", ErrValidation)
		}
		if _, seen := dedup[ref]; seen {
			continue
		}
		dedup[ref] = struct{}{}
		cleaned = append(cleaned, ref)
	}

	filtered, err := s.repo.FilterScopesForHome(ctx, homeID, cleaned)
	if err != nil {
		return err
	}
	if len(filtered) != len(cleaned) {
		return fmt.Errorf("%w: one or more scopes do not belong to this home", ErrValidation)
	}

	return s.repo.ReplaceGrants(ctx, homeID, targetUserID, filtered, &callerID)
}
