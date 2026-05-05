package permission

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// GetMemberRole reads role from home_members directly (no home package import).
// Returns ("", ErrNotFound) when the user is not a member of the home.
func (r *Repository) GetMemberRole(ctx context.Context, homeID, userID string) (string, error) {
	var role string
	err := r.pool.QueryRow(ctx,
		`SELECT role FROM home_members WHERE home_id = $1 AND user_id = $2`,
		homeID, userID,
	).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("get member role: %w", err)
	}
	return role, nil
}

// GetApplianceContext returns the (home_id, device_id, room_id) tuple for an appliance.
// Used by CheckAppliance to walk ancestors in a single query.
func (r *Repository) GetApplianceContext(ctx context.Context, applianceID string) (homeID, deviceID, roomID string, err error) {
	err = r.pool.QueryRow(ctx,
		`SELECT r.home_id, a.device_id, a.room_id
		   FROM appliances a
		   JOIN rooms r ON a.room_id = r.id
		  WHERE a.id = $1`,
		applianceID,
	).Scan(&homeID, &deviceID, &roomID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", "", ErrNotFound
		}
		return "", "", "", fmt.Errorf("get appliance context: %w", err)
	}
	return homeID, deviceID, roomID, nil
}

// FindApplianceByDeviceChannel resolves an appliance from (device_id, channel)
// and returns the appliance ID and its home_id. Used for command-time gating.
func (r *Repository) FindApplianceByDeviceChannel(ctx context.Context, deviceID string, channel int) (applianceID, homeID string, err error) {
	err = r.pool.QueryRow(ctx,
		`SELECT a.id, r.home_id
		   FROM appliances a
		   JOIN rooms r ON a.room_id = r.id
		  WHERE a.device_id = $1 AND a.channel = $2`,
		deviceID, channel,
	).Scan(&applianceID, &homeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", ErrNotFound
		}
		return "", "", fmt.Errorf("find appliance by channel: %w", err)
	}
	return applianceID, homeID, nil
}

// HasEffectiveGrant returns true if there is a grant covering the appliance via
// any of its ancestors (room or device) or directly on the appliance itself.
// Single round-trip to the DB.
func (r *Repository) HasEffectiveGrant(ctx context.Context, homeID, userID, roomID, deviceID, applianceID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM member_permissions
		    WHERE home_id = $1 AND user_id = $2
		      AND ((scope_type = 'appliance' AND scope_id = $3)
		        OR (scope_type = 'device'    AND scope_id = $4)
		        OR (scope_type = 'room'      AND scope_id = $5))
		 )`,
		homeID, userID, applianceID, deviceID, roomID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("has effective grant: %w", err)
	}
	return exists, nil
}

// ListGrants returns every grant row for (homeID, userID).
func (r *Repository) ListGrants(ctx context.Context, homeID, userID string) ([]Grant, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, home_id, user_id, scope_type, scope_id, granted_at, granted_by
		   FROM member_permissions
		  WHERE home_id = $1 AND user_id = $2`,
		homeID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list grants: %w", err)
	}
	defer rows.Close()

	var grants []Grant
	for rows.Next() {
		var g Grant
		var scope string
		if err := rows.Scan(&g.ID, &g.HomeID, &g.UserID, &scope, &g.ScopeID, &g.GrantedAt, &g.GrantedBy); err != nil {
			return nil, fmt.Errorf("scan grant: %w", err)
		}
		g.ScopeType = ScopeType(scope)
		grants = append(grants, g)
	}
	return grants, nil
}

// FilterScopesForHome returns the subset of refs that genuinely belong to homeID,
// resolved through the appropriate FK chain. The caller compares input length
// against output length to detect cross-home leaks.
func (r *Repository) FilterScopesForHome(ctx context.Context, homeID string, refs []ScopeRef) ([]ScopeRef, error) {
	if len(refs) == 0 {
		return refs, nil
	}

	var rooms, devices, appliances []string
	for _, ref := range refs {
		switch ref.Type {
		case ScopeRoom:
			rooms = append(rooms, ref.ID)
		case ScopeDevice:
			devices = append(devices, ref.ID)
		case ScopeAppliance:
			appliances = append(appliances, ref.ID)
		}
	}

	keep := map[ScopeType]map[string]struct{}{
		ScopeRoom:      {},
		ScopeDevice:    {},
		ScopeAppliance: {},
	}

	if len(rooms) > 0 {
		rs, err := r.pool.Query(ctx,
			`SELECT id FROM rooms WHERE home_id = $1 AND id = ANY($2)`,
			homeID, rooms,
		)
		if err != nil {
			return nil, fmt.Errorf("filter rooms: %w", err)
		}
		for rs.Next() {
			var id string
			if err := rs.Scan(&id); err != nil {
				rs.Close()
				return nil, fmt.Errorf("scan room: %w", err)
			}
			keep[ScopeRoom][id] = struct{}{}
		}
		rs.Close()
	}

	if len(devices) > 0 {
		rs, err := r.pool.Query(ctx,
			`SELECT d.id FROM devices d
			   JOIN rooms r ON d.room_id = r.id
			  WHERE r.home_id = $1 AND d.id = ANY($2)`,
			homeID, devices,
		)
		if err != nil {
			return nil, fmt.Errorf("filter devices: %w", err)
		}
		for rs.Next() {
			var id string
			if err := rs.Scan(&id); err != nil {
				rs.Close()
				return nil, fmt.Errorf("scan device: %w", err)
			}
			keep[ScopeDevice][id] = struct{}{}
		}
		rs.Close()
	}

	if len(appliances) > 0 {
		rs, err := r.pool.Query(ctx,
			`SELECT a.id FROM appliances a
			   JOIN rooms r ON a.room_id = r.id
			  WHERE r.home_id = $1 AND a.id = ANY($2)`,
			homeID, appliances,
		)
		if err != nil {
			return nil, fmt.Errorf("filter appliances: %w", err)
		}
		for rs.Next() {
			var id string
			if err := rs.Scan(&id); err != nil {
				rs.Close()
				return nil, fmt.Errorf("scan appliance: %w", err)
			}
			keep[ScopeAppliance][id] = struct{}{}
		}
		rs.Close()
	}

	out := make([]ScopeRef, 0, len(refs))
	for _, ref := range refs {
		if _, ok := keep[ref.Type][ref.ID]; ok {
			out = append(out, ref)
		}
	}
	return out, nil
}

// ReplaceGrants atomically deletes all existing grants for (homeID, userID)
// and inserts the provided set in a single transaction.
func (r *Repository) ReplaceGrants(ctx context.Context, homeID, userID string, refs []ScopeRef, grantedBy *string) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("replace grants begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`DELETE FROM member_permissions WHERE home_id = $1 AND user_id = $2`,
		homeID, userID,
	); err != nil {
		return fmt.Errorf("replace grants delete: %w", err)
	}

	for _, ref := range refs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO member_permissions (home_id, user_id, scope_type, scope_id, granted_by)
			 VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT (home_id, user_id, scope_type, scope_id) DO NOTHING`,
			homeID, userID, string(ref.Type), ref.ID, grantedBy,
		); err != nil {
			return fmt.Errorf("replace grants insert: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("replace grants commit: %w", err)
	}
	return nil
}

// ListAccessibleRoomIDs returns the IDs of all rooms the user has any effective
// grant touching (direct room grant, device grant in room, or appliance grant in room).
func (r *Repository) ListAccessibleRoomIDs(ctx context.Context, homeID, userID string) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT room_id FROM (
		     SELECT mp.scope_id AS room_id
		       FROM member_permissions mp
		       JOIN rooms rm ON rm.id = mp.scope_id AND rm.home_id = $1
		      WHERE mp.home_id = $1 AND mp.user_id = $2 AND mp.scope_type = 'room'
		     UNION
		     SELECT d.room_id
		       FROM member_permissions mp
		       JOIN devices d ON d.id = mp.scope_id
		       JOIN rooms rm ON rm.id = d.room_id AND rm.home_id = $1
		      WHERE mp.home_id = $1 AND mp.user_id = $2 AND mp.scope_type = 'device'
		     UNION
		     SELECT a.room_id
		       FROM member_permissions mp
		       JOIN appliances a ON a.id = mp.scope_id
		       JOIN rooms rm ON rm.id = a.room_id AND rm.home_id = $1
		      WHERE mp.home_id = $1 AND mp.user_id = $2 AND mp.scope_type = 'appliance'
		 ) sub`,
		homeID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list accessible room ids: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan room id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// ListAccessibleDeviceIDs returns the IDs of all devices the user has any effective
// grant touching (via room grant, direct device grant, or appliance grant on device).
func (r *Repository) ListAccessibleDeviceIDs(ctx context.Context, homeID, userID string) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT device_id FROM (
		     SELECT d.id AS device_id
		       FROM member_permissions mp
		       JOIN devices d ON d.room_id = mp.scope_id
		       JOIN rooms rm ON rm.id = d.room_id AND rm.home_id = $1
		      WHERE mp.home_id = $1 AND mp.user_id = $2 AND mp.scope_type = 'room'
		     UNION
		     SELECT mp.scope_id AS device_id
		       FROM member_permissions mp
		       JOIN devices d ON d.id = mp.scope_id
		       JOIN rooms rm ON rm.id = d.room_id AND rm.home_id = $1
		      WHERE mp.home_id = $1 AND mp.user_id = $2 AND mp.scope_type = 'device'
		     UNION
		     SELECT a.device_id
		       FROM member_permissions mp
		       JOIN appliances a ON a.id = mp.scope_id
		       JOIN rooms rm ON rm.id = a.room_id AND rm.home_id = $1
		      WHERE mp.home_id = $1 AND mp.user_id = $2 AND mp.scope_type = 'appliance'
		 ) sub`,
		homeID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list accessible device ids: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan device id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// GetDeviceContext returns the (home_id, room_id) for a device. Used by
// CheckDevice to resolve ancestors in a single query.
func (r *Repository) GetDeviceContext(ctx context.Context, deviceID string) (homeID, roomID string, err error) {
	err = r.pool.QueryRow(ctx,
		`SELECT rm.home_id, d.room_id
		   FROM devices d
		   JOIN rooms rm ON d.room_id = rm.id
		  WHERE d.id = $1`,
		deviceID,
	).Scan(&homeID, &roomID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", ErrNotFound
		}
		return "", "", fmt.Errorf("get device context: %w", err)
	}
	return homeID, roomID, nil
}

// CheckRoomAccess returns true if the user has any grant that touches the given room.
func (r *Repository) CheckRoomAccess(ctx context.Context, homeID, userID, roomID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (
		     SELECT 1 FROM member_permissions mp
		      WHERE mp.home_id = $1 AND mp.user_id = $2
		        AND (
		            (mp.scope_type = 'room' AND mp.scope_id = $3)
		         OR (mp.scope_type = 'device'
		             AND mp.scope_id IN (SELECT id FROM devices WHERE room_id = $3))
		         OR (mp.scope_type = 'appliance'
		             AND mp.scope_id IN (SELECT id FROM appliances WHERE room_id = $3))
		        )
		 )`,
		homeID, userID, roomID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check room access: %w", err)
	}
	return exists, nil
}

// HasEffectiveDeviceGrant returns true if the user has any grant that covers the
// given device: a room grant on the device's parent room, a direct device grant,
// or an appliance grant on any appliance belonging to the device.
func (r *Repository) HasEffectiveDeviceGrant(ctx context.Context, homeID, userID, roomID, deviceID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (
		     SELECT 1 FROM member_permissions mp
		      WHERE mp.home_id = $1 AND mp.user_id = $2
		        AND (
		            (mp.scope_type = 'room'   AND mp.scope_id = $3)
		         OR (mp.scope_type = 'device' AND mp.scope_id = $4)
		         OR (mp.scope_type = 'appliance'
		             AND mp.scope_id IN (SELECT id FROM appliances WHERE device_id = $4))
		        )
		 )`,
		homeID, userID, roomID, deviceID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("has effective device grant: %w", err)
	}
	return exists, nil
}

// ExpandToApplianceIDs expands a set of grant rows into the full set of
// appliance IDs the user can access. It runs three queries in parallel-ready
// fashion: appliance grants pass through directly, device grants expand to
// their appliances, room grants expand to all appliances in the room.
func (r *Repository) ExpandToApplianceIDs(ctx context.Context, homeID string, grants []Grant) ([]string, error) {
	var rooms, devices, appliances []string
	for _, g := range grants {
		switch g.ScopeType {
		case ScopeRoom:
			rooms = append(rooms, g.ScopeID)
		case ScopeDevice:
			devices = append(devices, g.ScopeID)
		case ScopeAppliance:
			appliances = append(appliances, g.ScopeID)
		}
	}

	out := map[string]struct{}{}
	for _, id := range appliances {
		out[id] = struct{}{}
	}

	if len(devices) > 0 {
		rs, err := r.pool.Query(ctx,
			`SELECT a.id FROM appliances a
			   JOIN rooms r ON a.room_id = r.id
			  WHERE r.home_id = $1 AND a.device_id = ANY($2)`,
			homeID, devices,
		)
		if err != nil {
			return nil, fmt.Errorf("expand devices: %w", err)
		}
		for rs.Next() {
			var id string
			if err := rs.Scan(&id); err != nil {
				rs.Close()
				return nil, fmt.Errorf("scan: %w", err)
			}
			out[id] = struct{}{}
		}
		rs.Close()
	}

	if len(rooms) > 0 {
		rs, err := r.pool.Query(ctx,
			`SELECT a.id FROM appliances a
			  WHERE a.room_id = ANY($1)`,
			rooms,
		)
		if err != nil {
			return nil, fmt.Errorf("expand rooms: %w", err)
		}
		for rs.Next() {
			var id string
			if err := rs.Scan(&id); err != nil {
				rs.Close()
				return nil, fmt.Errorf("scan: %w", err)
			}
			out[id] = struct{}{}
		}
		rs.Close()
	}

	ids := make([]string, 0, len(out))
	for id := range out {
		ids = append(ids, id)
	}
	return ids, nil
}
