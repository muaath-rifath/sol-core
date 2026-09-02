package firmware

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type FirmwareVersion struct {
	ID            string    `json:"id"`
	TemplateID    string    `json:"template_id"`
	Version       string    `json:"version"`
	BootloaderKey string    `json:"bootloader_key"`
	PartitionKey  string    `json:"partition_key"`
	AppKey        string    `json:"app_key"`
	ModelKey      *string   `json:"model_key,omitempty"`
	SourceKey     *string   `json:"source_key,omitempty"`
	SizeBytes     *int64    `json:"size_bytes"`
	UploadedBy    *string   `json:"uploaded_by,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type VersionRepository struct {
	pool *pgxpool.Pool
}

func NewVersionRepository(pool *pgxpool.Pool) *VersionRepository {
	return &VersionRepository{pool: pool}
}

func (r *VersionRepository) Create(ctx context.Context, v *FirmwareVersion) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO firmware_versions (id, template_id, version, bootloader_key, partition_key, app_key, model_key, source_key, size_bytes, uploaded_by, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		v.ID, v.TemplateID, v.Version, v.BootloaderKey, v.PartitionKey, v.AppKey, v.ModelKey, v.SourceKey, v.SizeBytes, v.UploadedBy, v.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create firmware version: %w", err)
	}
	return nil
}

func (r *VersionRepository) List(ctx context.Context, templateID string) ([]FirmwareVersion, error) {
	query := `SELECT id, template_id, version, bootloader_key, partition_key, app_key, model_key, source_key, size_bytes, uploaded_by, created_at
		FROM firmware_versions`
	args := []any{}
	if templateID != "" {
		query += ` WHERE template_id = $1`
		args = append(args, templateID)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list firmware versions: %w", err)
	}
	defer rows.Close()

	versions := make([]FirmwareVersion, 0)
	for rows.Next() {
		var v FirmwareVersion
		if err := rows.Scan(
			&v.ID,
			&v.TemplateID,
			&v.Version,
			&v.BootloaderKey,
			&v.PartitionKey,
			&v.AppKey,
			&v.ModelKey,
			&v.SourceKey,
			&v.SizeBytes,
			&v.UploadedBy,
			&v.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan firmware version: %w", err)
		}
		versions = append(versions, v)
	}

	return versions, nil
}

func (r *VersionRepository) GetByID(ctx context.Context, id string) (*FirmwareVersion, error) {
	var v FirmwareVersion
	err := r.pool.QueryRow(ctx,
		`SELECT id, template_id, version, bootloader_key, partition_key, app_key, model_key, source_key, size_bytes, uploaded_by, created_at
		 FROM firmware_versions WHERE id = $1`, id,
	).Scan(
		&v.ID,
		&v.TemplateID,
		&v.Version,
		&v.BootloaderKey,
		&v.PartitionKey,
		&v.AppKey,
		&v.ModelKey,
		&v.SourceKey,
		&v.SizeBytes,
		&v.UploadedBy,
		&v.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get firmware version: %w", err)
	}

	return &v, nil
}
