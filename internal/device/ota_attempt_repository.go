package device

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OTAAttemptStatus string

const (
	OTAAttemptStatusInitiated    OTAAttemptStatus = "initiated"
	OTAAttemptStatusAcknowledged OTAAttemptStatus = "acknowledged"
	OTAAttemptStatusDownloading  OTAAttemptStatus = "downloading"
	OTAAttemptStatusVerifying    OTAAttemptStatus = "verifying"
	OTAAttemptStatusUpdating     OTAAttemptStatus = "updating"
	OTAAttemptStatusCancelling   OTAAttemptStatus = "cancelling"
	OTAAttemptStatusCancelled    OTAAttemptStatus = "cancelled"
	OTAAttemptStatusTimedOut     OTAAttemptStatus = "timed_out"
	OTAAttemptStatusUpdated      OTAAttemptStatus = "updated"
	OTAAttemptStatusFailed       OTAAttemptStatus = "failed"
)

type OTAAttempt struct {
	ID                string           `json:"id"`
	DeviceID          string           `json:"device_id"`
	RoomID            string           `json:"room_id"`
	HomeID            string           `json:"home_id"`
	FirmwareVersionID string           `json:"firmware_version_id"`
	RequestedBy       *string          `json:"requested_by,omitempty"`
	IdempotencyKey    *string          `json:"idempotency_key,omitempty"`
	RequestID         string           `json:"request_id"`
	Status            OTAAttemptStatus `json:"status"`
	ProgressPct       int              `json:"progress_pct"`
	Logs              string           `json:"logs"`
	ErrorText         *string          `json:"error_text,omitempty"`
	StartedAt         time.Time        `json:"started_at"`
	FinishedAt        *time.Time       `json:"finished_at,omitempty"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
}

type OTAAttemptRepository struct {
	pool *pgxpool.Pool
}

func NewOTAAttemptRepository(pool *pgxpool.Pool) *OTAAttemptRepository {
	return &OTAAttemptRepository{pool: pool}
}

func (r *OTAAttemptRepository) Create(ctx context.Context, a *OTAAttempt) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO ota_update_attempts (
			id, device_id, room_id, home_id, firmware_version_id, requested_by, idempotency_key, request_id,
			status, progress_pct, logs, error_text, started_at, finished_at, created_at, updated_at
		 ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
		a.ID, a.DeviceID, a.RoomID, a.HomeID, a.FirmwareVersionID, a.RequestedBy, a.IdempotencyKey, a.RequestID,
		a.Status, a.ProgressPct, a.Logs, a.ErrorText, a.StartedAt, a.FinishedAt, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create ota attempt: %w", err)
	}
	return nil
}

func (r *OTAAttemptRepository) GetByRequestID(ctx context.Context, requestID string) (*OTAAttempt, error) {
	var a OTAAttempt
	err := r.pool.QueryRow(ctx,
		`SELECT id, device_id, room_id, home_id, firmware_version_id, requested_by, idempotency_key, request_id,
			status, progress_pct, logs, error_text, started_at, finished_at, created_at, updated_at
		 FROM ota_update_attempts
		 WHERE request_id = $1`,
		requestID,
	).Scan(
		&a.ID,
		&a.DeviceID,
		&a.RoomID,
		&a.HomeID,
		&a.FirmwareVersionID,
		&a.RequestedBy,
		&a.IdempotencyKey,
		&a.RequestID,
		&a.Status,
		&a.ProgressPct,
		&a.Logs,
		&a.ErrorText,
		&a.StartedAt,
		&a.FinishedAt,
		&a.CreatedAt,
		&a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get ota attempt by request id: %w", err)
	}
	return &a, nil
}

func (r *OTAAttemptRepository) GetByID(ctx context.Context, id string) (*OTAAttempt, error) {
	var a OTAAttempt
	err := r.pool.QueryRow(ctx,
		`SELECT id, device_id, room_id, home_id, firmware_version_id, requested_by, idempotency_key, request_id,
			status, progress_pct, logs, error_text, started_at, finished_at, created_at, updated_at
		 FROM ota_update_attempts
		 WHERE id = $1`,
		id,
	).Scan(
		&a.ID,
		&a.DeviceID,
		&a.RoomID,
		&a.HomeID,
		&a.FirmwareVersionID,
		&a.RequestedBy,
		&a.IdempotencyKey,
		&a.RequestID,
		&a.Status,
		&a.ProgressPct,
		&a.Logs,
		&a.ErrorText,
		&a.StartedAt,
		&a.FinishedAt,
		&a.CreatedAt,
		&a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get ota attempt by id: %w", err)
	}
	return &a, nil
}

func (r *OTAAttemptRepository) ListByRoom(ctx context.Context, roomID string, limit int) ([]OTAAttempt, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, device_id, room_id, home_id, firmware_version_id, requested_by, idempotency_key, request_id,
			status, progress_pct, logs, error_text, started_at, finished_at, created_at, updated_at
		 FROM ota_update_attempts
		 WHERE room_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2`,
		roomID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list ota attempts by room: %w", err)
	}
	defer rows.Close()

	attempts := make([]OTAAttempt, 0)
	for rows.Next() {
		var a OTAAttempt
		if err := rows.Scan(
			&a.ID,
			&a.DeviceID,
			&a.RoomID,
			&a.HomeID,
			&a.FirmwareVersionID,
			&a.RequestedBy,
			&a.IdempotencyKey,
			&a.RequestID,
			&a.Status,
			&a.ProgressPct,
			&a.Logs,
			&a.ErrorText,
			&a.StartedAt,
			&a.FinishedAt,
			&a.CreatedAt,
			&a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ota attempt: %w", err)
		}
		attempts = append(attempts, a)
	}

	return attempts, nil
}

func (r *OTAAttemptRepository) GetByIdempotencyKey(ctx context.Context, key string) (*OTAAttempt, error) {
	var a OTAAttempt
	err := r.pool.QueryRow(ctx,
		`SELECT id, device_id, room_id, home_id, firmware_version_id, requested_by, idempotency_key, request_id,
			status, progress_pct, logs, error_text, started_at, finished_at, created_at, updated_at
		 FROM ota_update_attempts
		 WHERE idempotency_key = $1`,
		key,
	).Scan(
		&a.ID,
		&a.DeviceID,
		&a.RoomID,
		&a.HomeID,
		&a.FirmwareVersionID,
		&a.RequestedBy,
		&a.IdempotencyKey,
		&a.RequestID,
		&a.Status,
		&a.ProgressPct,
		&a.Logs,
		&a.ErrorText,
		&a.StartedAt,
		&a.FinishedAt,
		&a.CreatedAt,
		&a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get ota attempt by idempotency key: %w", err)
	}
	return &a, nil
}

func (r *OTAAttemptRepository) HasActiveForDevice(ctx context.Context, deviceID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM ota_update_attempts
			WHERE device_id = $1
			  AND status IN ('initiated', 'acknowledged', 'downloading', 'verifying', 'updating', 'cancelling')
		)`,
		deviceID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check active ota attempt: %w", err)
	}
	return exists, nil
}

func (r *OTAAttemptRepository) AppendLog(ctx context.Context, requestID string, line string) error {
	if line == "" {
		return nil
	}
	_, err := r.pool.Exec(ctx,
		`UPDATE ota_update_attempts
		 SET logs = logs || CASE WHEN logs = '' THEN $2 ELSE E'\n' || $2 END,
		     updated_at = $3
		 WHERE request_id = $1`,
		requestID, line, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("append ota attempt log: %w", err)
	}
	return nil
}

func (r *OTAAttemptRepository) UpdateProgress(
	ctx context.Context,
	requestID string,
	status OTAAttemptStatus,
	progressPct int,
	errorText *string,
	finishedAt *time.Time,
) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE ota_update_attempts
		 SET status = $2,
		     progress_pct = $3,
		     error_text = $4,
		     finished_at = $5,
		     updated_at = $6
		 WHERE request_id = $1`,
		requestID,
		status,
		progressPct,
		errorText,
		finishedAt,
		time.Now(),
	)
	if err != nil {
		return fmt.Errorf("update ota attempt progress: %w", err)
	}
	return nil
}

func (r *OTAAttemptRepository) ListStaleActive(ctx context.Context, olderThan time.Time, limit int) ([]OTAAttempt, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, device_id, room_id, home_id, firmware_version_id, requested_by, idempotency_key, request_id,
			status, progress_pct, logs, error_text, started_at, finished_at, created_at, updated_at
		 FROM ota_update_attempts
		 WHERE status IN ('initiated', 'acknowledged', 'downloading', 'verifying', 'updating', 'cancelling')
		   AND updated_at < $1
		 ORDER BY updated_at ASC
		 LIMIT $2`,
		olderThan, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list stale ota attempts: %w", err)
	}
	defer rows.Close()

	attempts := make([]OTAAttempt, 0)
	for rows.Next() {
		var a OTAAttempt
		if err := rows.Scan(
			&a.ID,
			&a.DeviceID,
			&a.RoomID,
			&a.HomeID,
			&a.FirmwareVersionID,
			&a.RequestedBy,
			&a.IdempotencyKey,
			&a.RequestID,
			&a.Status,
			&a.ProgressPct,
			&a.Logs,
			&a.ErrorText,
			&a.StartedAt,
			&a.FinishedAt,
			&a.CreatedAt,
			&a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan stale ota attempt: %w", err)
		}
		attempts = append(attempts, a)
	}

	return attempts, nil
}
