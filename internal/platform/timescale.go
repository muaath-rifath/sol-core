package platform

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateHypertable(ctx context.Context, pool *pgxpool.Pool, table, timeColumn string) error {
	query := fmt.Sprintf(
		`SELECT create_hypertable('%s', '%s', if_not_exists => TRUE)`,
		table, timeColumn,
	)
	_, err := pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("create hypertable %s: %w", table, err)
	}
	return nil
}

func SetRetentionPolicy(ctx context.Context, pool *pgxpool.Pool, table string, intervalDays int) error {
	query := fmt.Sprintf(
		`SELECT add_retention_policy('%s', INTERVAL '%d days', if_not_exists => TRUE)`,
		table, intervalDays,
	)
	_, err := pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("set retention policy %s: %w", table, err)
	}
	return nil
}
