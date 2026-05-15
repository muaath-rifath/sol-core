package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/muaathrifath/sol-core/internal/device"
	"github.com/muaathrifath/sol-core/internal/user"
)

// Embedder is satisfied by *CohereClient.
type Embedder interface {
	Embed(ctx context.Context, text, inputType string) ([]float32, error)
}

// PermGate is satisfied by *permission.Service (subset needed here).
type PermGate interface {
	ListAccessibleApplianceIDs(ctx context.Context, homeID, userID string) (ids []string, allAccess bool, err error)
	CheckAppliance(ctx context.Context, userID, applianceID string) (bool, error)
}

type Tools struct {
	permGate  PermGate
	deviceSvc *device.Service
	embedder  Embedder
	pool      *pgxpool.Pool
}

func NewTools(permGate PermGate, deviceSvc *device.Service, embedder Embedder, pool *pgxpool.Pool) *Tools {
	return &Tools{permGate: permGate, deviceSvc: deviceSvc, embedder: embedder, pool: pool}
}

type ApplianceSummary struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Type  string         `json:"type"`
	Room  string         `json:"room,omitempty"`
	State map[string]any `json:"state"`
}

// Dispatch routes a tool call from the Realtime API to the correct implementation.
func (t *Tools) Dispatch(ctx context.Context, name, arguments string, u *user.User, homeID string) string {
	slog.Info("chat: tool call", "tool", name, "home", homeID, "user", u.ID)
	switch name {
	case "discover_devices":
		var args struct {
			Query string `json:"query"`
		}
		_ = json.Unmarshal([]byte(arguments), &args)
		result, err := t.discoverDevices(ctx, u, homeID, args.Query)
		if err != nil {
			slog.Warn("chat: discover_devices error", "query", args.Query, "error", err)
			return fmt.Sprintf("error: %s", err.Error())
		}
		slog.Info("chat: discover_devices result", "query", args.Query, "count", len(result))
		b, _ := json.Marshal(result)
		return string(b)

	case "control_device":
		var args struct {
			ApplianceID string `json:"appliance_id"`
			Action      string `json:"action"`
		}
		_ = json.Unmarshal([]byte(arguments), &args)
		if err := t.controlDevice(ctx, u, args.ApplianceID, args.Action); err != nil {
			slog.Warn("chat: control_device error", "appliance", args.ApplianceID, "action", args.Action, "error", err)
			return fmt.Sprintf("error: %s", err.Error())
		}
		slog.Info("chat: control_device ok", "appliance", args.ApplianceID, "action", args.Action)
		return "done"

	default:
		slog.Warn("chat: unknown tool", "tool", name)
		return fmt.Sprintf("unknown tool: %s", name)
	}
}

func (t *Tools) discoverDevices(ctx context.Context, u *user.User, homeID, query string) ([]ApplianceSummary, error) {
	ids, allAccess, err := t.permGate.ListAccessibleApplianceIDs(ctx, homeID, u.ID)
	if err != nil {
		return nil, fmt.Errorf("list accessible appliances: %w", err)
	}
	if !allAccess && len(ids) == 0 {
		return []ApplianceSummary{}, nil
	}

	// Try vector search; fall back to listing all appliances when the embedder is unavailable.
	// The model receives the full list with room names and picks the right one itself.
	vec, embedErr := t.embedder.Embed(ctx, query, "search_query")
	if embedErr != nil {
		slog.Warn("chat: embedder unavailable, listing all appliances", "error", embedErr)
		return t.listAllAppliances(ctx, homeID, ids, allAccess)
	}

	return t.vectorSearchAppliances(ctx, homeID, ids, allAccess, formatVector(vec))
}

func (t *Tools) vectorSearchAppliances(ctx context.Context, homeID string, ids []string, allAccess bool, vecStr string) ([]ApplianceSummary, error) {
	var rows []ApplianceSummary

	if allAccess {
		result, err := t.pool.Query(ctx,
			`SELECT a.id, a.name, a.type, COALESCE(r.name, ''), a.state
			 FROM appliances a
			 LEFT JOIN rooms r ON r.id = a.room_id
			 WHERE r.home_id = $1
			 ORDER BY a.embedding <=> $2::vector
			 LIMIT 10`,
			homeID, vecStr,
		)
		if err != nil {
			return nil, fmt.Errorf("vector search (all): %w", err)
		}
		defer result.Close()
		for result.Next() {
			var s ApplianceSummary
			if err := result.Scan(&s.ID, &s.Name, &s.Type, &s.Room, &s.State); err != nil {
				continue
			}
			rows = append(rows, s)
		}
	} else {
		result, err := t.pool.Query(ctx,
			`SELECT a.id, a.name, a.type, COALESCE(r.name, ''), a.state
			 FROM appliances a
			 LEFT JOIN rooms r ON r.id = a.room_id
			 WHERE a.id = ANY($1)
			 ORDER BY a.embedding <=> $2::vector
			 LIMIT 10`,
			ids, vecStr,
		)
		if err != nil {
			return nil, fmt.Errorf("vector search (filtered): %w", err)
		}
		defer result.Close()
		for result.Next() {
			var s ApplianceSummary
			if err := result.Scan(&s.ID, &s.Name, &s.Type, &s.Room, &s.State); err != nil {
				continue
			}
			rows = append(rows, s)
		}
	}

	if rows == nil {
		rows = []ApplianceSummary{}
	}
	return rows, nil
}

// listAllAppliances is the fallback used when the embedder is unavailable.
// Returns every accessible appliance with its room name so the model can
// match the user's description itself instead of relying on search.
func (t *Tools) listAllAppliances(ctx context.Context, homeID string, ids []string, allAccess bool) ([]ApplianceSummary, error) {
	var rows []ApplianceSummary

	if allAccess {
		result, err := t.pool.Query(ctx,
			`SELECT a.id, a.name, a.type, COALESCE(r.name, ''), a.state
			 FROM appliances a
			 LEFT JOIN rooms r ON r.id = a.room_id
			 WHERE r.home_id = $1
			 ORDER BY r.name, a.name`,
			homeID,
		)
		if err != nil {
			return nil, fmt.Errorf("list appliances (all): %w", err)
		}
		defer result.Close()
		for result.Next() {
			var s ApplianceSummary
			if err := result.Scan(&s.ID, &s.Name, &s.Type, &s.Room, &s.State); err != nil {
				continue
			}
			rows = append(rows, s)
		}
	} else {
		result, err := t.pool.Query(ctx,
			`SELECT a.id, a.name, a.type, COALESCE(r.name, ''), a.state
			 FROM appliances a
			 LEFT JOIN rooms r ON r.id = a.room_id
			 WHERE a.id = ANY($1)
			 ORDER BY r.name, a.name`,
			ids,
		)
		if err != nil {
			return nil, fmt.Errorf("list appliances (filtered): %w", err)
		}
		defer result.Close()
		for result.Next() {
			var s ApplianceSummary
			if err := result.Scan(&s.ID, &s.Name, &s.Type, &s.Room, &s.State); err != nil {
				continue
			}
			rows = append(rows, s)
		}
	}

	if rows == nil {
		rows = []ApplianceSummary{}
	}
	return rows, nil
}

func (t *Tools) controlDevice(ctx context.Context, u *user.User, applianceID, action string) error {
	// Permission check — same service used by REST handlers.
	allowed, err := t.permGate.CheckAppliance(ctx, u.ID, applianceID)
	if err != nil {
		return fmt.Errorf("permission check: %w", err)
	}
	if !allowed {
		return fmt.Errorf("access denied")
	}

	appliance, err := t.deviceSvc.GetAppliance(ctx, applianceID)
	if err != nil {
		return fmt.Errorf("get appliance: %w", err)
	}

	cmd := device.Command{
		DeviceID: appliance.DeviceID,
		Action:   action,
	}
	if appliance.Channel != nil {
		cmd.Params = map[string]any{"channel": *appliance.Channel}
	}

	return t.deviceSvc.SendCommand(ctx, cmd)
}

// formatVector converts []float32 to pgvector text literal: "[0.1,0.2,...]"
func formatVector(v []float32) string {
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = fmt.Sprintf("%g", f)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
