package firmware

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type BuildStatus string

const (
	StatusQueued   BuildStatus = "queued"
	StatusBuilding BuildStatus = "building"
	StatusSuccess  BuildStatus = "success"
	StatusFailed   BuildStatus = "failed"
)

type FirmwareBuild struct {
	ID                string      `json:"id"`
	TemplateID        string      `json:"template_id"`
	TargetBoard       string      `json:"target_board"`
	Status            BuildStatus `json:"status"`
	Logs              string      `json:"logs"`
	FirmwareVersionID *string     `json:"firmware_version_id,omitempty"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

type BuildRepository struct {
	pool *pgxpool.Pool
}

func NewBuildRepository(pool *pgxpool.Pool) *BuildRepository {
	return &BuildRepository{pool: pool}
}

func (r *BuildRepository) Create(ctx context.Context, b *FirmwareBuild) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO firmware_builds (id, template_id, target_board, status, logs, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		b.ID, b.TemplateID, b.TargetBoard, b.Status, b.Logs, b.CreatedAt, b.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create firmware build: %w", err)
	}
	return nil
}

func (r *BuildRepository) GetByID(ctx context.Context, id string) (*FirmwareBuild, error) {
	var b FirmwareBuild
	err := r.pool.QueryRow(ctx,
		`SELECT id, template_id, target_board, status, logs, firmware_version_id, created_at, updated_at
		 FROM firmware_builds WHERE id = $1`, id,
	).Scan(
		&b.ID,
		&b.TemplateID,
		&b.TargetBoard,
		&b.Status,
		&b.Logs,
		&b.FirmwareVersionID,
		&b.CreatedAt,
		&b.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get firmware build: %w", err)
	}
	return &b, nil
}

func (r *BuildRepository) UpdateStatus(ctx context.Context, id string, status BuildStatus, logs string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE firmware_builds SET status = $1, logs = $2, updated_at = $3 WHERE id = $4`,
		status, logs, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("update firmware build status: %w", err)
	}
	return nil
}

func (r *BuildRepository) UpdateSuccess(ctx context.Context, id string, versionID string, logs string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE firmware_builds SET status = $1, logs = $2, firmware_version_id = $3, updated_at = $4 WHERE id = $5`,
		StatusSuccess, logs, versionID, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("update firmware build success: %w", err)
	}
	return nil
}

func (r *BuildRepository) AppendLogs(ctx context.Context, id string, newLogs string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE firmware_builds SET logs = logs || $1, updated_at = $2 WHERE id = $3`,
		newLogs, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("append firmware build logs: %w", err)
	}
	return nil
}
