package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

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

// ToolContext carries caller context for permission scoping and audit logging.
// RoomID is non-empty only for ESP32 voice sessions; it restricts all device
// operations to appliances belonging to that room.
type ToolContext struct {
	HomeID    string // always required
	RoomID    string // non-empty → restrict to this room (ESP32 voice sessions)
	ActorType string // "user" or "esp32"
	ActorID   string // user.ID for users, device.ID for ESP32
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
func (t *Tools) Dispatch(ctx context.Context, name, arguments string, u *user.User, tc ToolContext) string {
	slog.Info("chat: tool call", "tool", name, "home", tc.HomeID, "user", u.ID, "actor_type", tc.ActorType, "room", tc.RoomID)
	switch name {
	case "discover_devices":
		var args struct {
			Query string `json:"query"`
		}
		_ = json.Unmarshal([]byte(arguments), &args)
		result, err := t.discoverDevices(ctx, u, tc, args.Query)
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
		result, err := t.controlDevice(ctx, u, tc, args.ApplianceID, args.Action)
		if err != nil {
			slog.Warn("chat: control_device error", "appliance", args.ApplianceID, "action", args.Action, "error", err)
		}
		slog.Info("chat: control_device result", "appliance", args.ApplianceID, "action", args.Action, "ok", result.OK)
		b, _ := json.Marshal(result)
		return string(b)

	case "check_device_online":
		var args struct {
			ApplianceID string `json:"appliance_id"`
		}
		_ = json.Unmarshal([]byte(arguments), &args)
		result := t.checkDeviceOnline(ctx, u, tc, args.ApplianceID)
		b, _ := json.Marshal(result)
		return string(b)

	case "get_device_state":
		var args struct {
			ApplianceID string `json:"appliance_id"`
		}
		_ = json.Unmarshal([]byte(arguments), &args)
		result := t.getDeviceState(ctx, u, tc, args.ApplianceID)
		b, _ := json.Marshal(result)
		return string(b)

	default:
		slog.Warn("chat: unknown tool", "tool", name)
		return fmt.Sprintf("unknown tool: %s", name)
	}
}

func (t *Tools) discoverDevices(ctx context.Context, u *user.User, tc ToolContext, query string) ([]ApplianceSummary, error) {
	ids, allAccess, err := t.permGate.ListAccessibleApplianceIDs(ctx, tc.HomeID, u.ID)
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
		return t.listAllAppliances(ctx, tc.HomeID, ids, allAccess, tc.RoomID)
	}

	return t.vectorSearchAppliances(ctx, tc.HomeID, ids, allAccess, formatVector(vec), tc.RoomID)
}

func (t *Tools) vectorSearchAppliances(ctx context.Context, homeID string, ids []string, allAccess bool, vecStr, roomID string) ([]ApplianceSummary, error) {
	var rows []ApplianceSummary

	if allAccess {
		result, err := t.pool.Query(ctx,
			`SELECT a.id, a.name, a.type, COALESCE(r.name, ''), a.state
			 FROM appliances a
			 LEFT JOIN rooms r ON r.id = a.room_id
			 WHERE r.home_id = $1
			 AND ($3 = '' OR a.room_id::text = $3)
			 ORDER BY a.embedding <=> $2::vector
			 LIMIT 10`,
			homeID, vecStr, roomID,
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
			 AND ($3 = '' OR a.room_id::text = $3)
			 ORDER BY a.embedding <=> $2::vector
			 LIMIT 10`,
			ids, vecStr, roomID,
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
func (t *Tools) listAllAppliances(ctx context.Context, homeID string, ids []string, allAccess bool, roomID string) ([]ApplianceSummary, error) {
	var rows []ApplianceSummary

	if allAccess {
		result, err := t.pool.Query(ctx,
			`SELECT a.id, a.name, a.type, COALESCE(r.name, ''), a.state
			 FROM appliances a
			 LEFT JOIN rooms r ON r.id = a.room_id
			 WHERE r.home_id = $1
			 AND ($2 = '' OR a.room_id::text = $2)
			 ORDER BY r.name, a.name`,
			homeID, roomID,
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
			 AND ($2 = '' OR a.room_id::text = $2)
			 ORDER BY r.name, a.name`,
			ids, roomID,
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

type controlDeviceResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type checkDeviceOnlineResult struct {
	Online bool   `json:"online"`
	Reason string `json:"reason,omitempty"`
}

type getDeviceStateResult struct {
	State     map[string]any `json:"state"`
	UpdatedAt string         `json:"updated_at,omitempty"`
	Error     string         `json:"error,omitempty"`
}

func (t *Tools) controlDevice(ctx context.Context, u *user.User, tc ToolContext, applianceID, action string) (controlDeviceResult, error) {
	// Permission check — same service used by REST handlers.
	allowed, err := t.permGate.CheckAppliance(ctx, u.ID, applianceID)
	if err != nil {
		msg := fmt.Sprintf("permission check: %s", err.Error())
		return controlDeviceResult{OK: false, Message: msg}, err
	}
	if !allowed {
		return controlDeviceResult{OK: false, Message: "access denied"}, nil
	}

	appliance, err := t.deviceSvc.GetAppliance(ctx, applianceID)
	if err != nil {
		msg := fmt.Sprintf("get appliance: %s", err.Error())
		return controlDeviceResult{OK: false, Message: msg}, err
	}

	// Room scope enforcement: ESP32 voice sessions may only control appliances
	// in their own room.
	if tc.RoomID != "" && appliance.RoomID != tc.RoomID {
		slog.Warn("chat: cross-room control attempt blocked",
			"actor", tc.ActorID, "actor_type", tc.ActorType,
			"session_room", tc.RoomID, "appliance_room", appliance.RoomID)
		return controlDeviceResult{OK: false, Message: "access denied: appliance is not in your room"}, nil
	}

	stateBefore := appliance.State

	// Register actor attribution before sending so HandleStateUpdate can label the log.
	actorLabel := u.Name
	if tc.ActorType == "esp32" {
		if dev, devErr := t.deviceSvc.Get(ctx, tc.ActorID); devErr == nil {
			actorLabel = dev.Name + " (voice)"
		}
	}
	t.deviceSvc.SetPendingActor(appliance.ID, tc.ActorType, actorLabel)

	cmd := device.Command{
		DeviceID: appliance.DeviceID,
		Action:   action,
	}
	switch action {
	case "on", "off":
		cmd.Action = "set_relay"
		cmd.Params = map[string]any{"power": action == "on"}
		if appliance.Channel != nil {
			cmd.Params["channel"] = *appliance.Channel
		}
	default:
		if appliance.Channel != nil {
			cmd.Params = map[string]any{"channel": *appliance.Channel}
		}
	}

	ack, err := t.deviceSvc.SendCommandAwait(ctx, cmd, 5*time.Second)

	// Write audit log regardless of success/failure.
	t.insertControlLog(ctx, tc, appliance.RoomID, appliance.DeviceID, appliance.ID, action, cmd.Params, stateBefore, ack.OK, ack.Message)

	if err != nil {
		return controlDeviceResult{OK: false, Message: ack.Message}, err
	}
	return controlDeviceResult{OK: ack.OK, Message: ack.Message}, nil
}

func (t *Tools) checkDeviceOnline(ctx context.Context, u *user.User, tc ToolContext, applianceID string) checkDeviceOnlineResult {
	allowed, err := t.permGate.CheckAppliance(ctx, u.ID, applianceID)
	if err != nil || !allowed {
		return checkDeviceOnlineResult{Online: false, Reason: "access denied"}
	}

	appliance, err := t.deviceSvc.GetAppliance(ctx, applianceID)
	if err != nil {
		return checkDeviceOnlineResult{Online: false, Reason: "appliance not found"}
	}

	if tc.RoomID != "" && appliance.RoomID != tc.RoomID {
		return checkDeviceOnlineResult{Online: false, Reason: "access denied: appliance is not in your room"}
	}

	dev, err := t.deviceSvc.Get(ctx, appliance.DeviceID)
	if err != nil {
		return checkDeviceOnlineResult{Online: false, Reason: "device not found"}
	}

	if !dev.Online {
		return checkDeviceOnlineResult{Online: false, Reason: "device reports offline"}
	}
	if time.Since(dev.UpdatedAt) > t.deviceSvc.OnlineFreshness() {
		return checkDeviceOnlineResult{Online: false, Reason: "device status is stale"}
	}
	return checkDeviceOnlineResult{Online: true}
}

func (t *Tools) getDeviceState(ctx context.Context, u *user.User, tc ToolContext, applianceID string) getDeviceStateResult {
	allowed, err := t.permGate.CheckAppliance(ctx, u.ID, applianceID)
	if err != nil || !allowed {
		return getDeviceStateResult{State: map[string]any{}, Error: "access denied"}
	}

	appliance, err := t.deviceSvc.GetAppliance(ctx, applianceID)
	if err != nil {
		return getDeviceStateResult{State: map[string]any{}, Error: "appliance not found"}
	}

	if tc.RoomID != "" && appliance.RoomID != tc.RoomID {
		return getDeviceStateResult{State: map[string]any{}, Error: "access denied: appliance is not in your room"}
	}

	return getDeviceStateResult{
		State:     appliance.State,
		UpdatedAt: appliance.UpdatedAt.Format(time.RFC3339),
	}
}

// insertControlLog writes a device_control_logs row. Errors are logged but not
// propagated — audit failure must not block device control.
func (t *Tools) insertControlLog(ctx context.Context, tc ToolContext, roomID, deviceID, applianceID, action string, cmdParams, stateBefore map[string]any, success bool, errMsg string) {
	actorID := tc.ActorID
	if actorID == "" {
		return // nothing useful to log
	}
	roomIDVal := roomID
	if roomIDVal == "" {
		roomIDVal = tc.RoomID
	}

	var errMsgPtr *string
	if !success && errMsg != "" {
		errMsgPtr = &errMsg
	}

	_, dbErr := t.pool.Exec(ctx,
		`INSERT INTO device_control_logs
		 (home_id, room_id, device_id, appliance_id, actor_type, actor_id,
		  action, command_params, state_before, success, error_message)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tc.HomeID, roomIDVal, deviceID, applianceID,
		tc.ActorType, actorID,
		action, cmdParams, stateBefore,
		success, errMsgPtr,
	)
	if dbErr != nil {
		slog.Warn("chat: failed to write device_control_log", "error", dbErr)
	}
}

// formatVector converts []float32 to pgvector text literal: "[0.1,0.2,...]"
func formatVector(v []float32) string {
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = fmt.Sprintf("%g", f)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
