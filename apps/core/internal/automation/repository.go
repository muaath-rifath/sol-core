package automation

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, rule *Rule) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO automation_rules (id, name, description, enabled, trigger_config, conditions, actions, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		rule.ID, rule.Name, rule.Description, rule.Enabled, rule.Trigger, rule.Conditions, rule.Actions, rule.CreatedAt, rule.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert rule: %w", err)
	}
	return nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*Rule, error) {
	var rule Rule
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, description, enabled, trigger_config, conditions, actions, created_at, updated_at
		 FROM automation_rules WHERE id = $1`, id,
	).Scan(&rule.ID, &rule.Name, &rule.Description, &rule.Enabled, &rule.Trigger, &rule.Conditions, &rule.Actions, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get rule: %w", err)
	}
	return &rule, nil
}

func (r *Repository) List(ctx context.Context) ([]Rule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, description, enabled, trigger_config, conditions, actions, created_at, updated_at
		 FROM automation_rules ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list rules: %w", err)
	}
	defer rows.Close()

	var rules []Rule
	for rows.Next() {
		var rule Rule
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.Description, &rule.Enabled, &rule.Trigger, &rule.Conditions, &rule.Actions, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan rule: %w", err)
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (r *Repository) Update(ctx context.Context, rule *Rule) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE automation_rules SET name=$2, description=$3, enabled=$4, trigger_config=$5, conditions=$6, actions=$7, updated_at=$8
		 WHERE id=$1`,
		rule.ID, rule.Name, rule.Description, rule.Enabled, rule.Trigger, rule.Conditions, rule.Actions, rule.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update rule: %w", err)
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM automation_rules WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete rule: %w", err)
	}
	return nil
}

func (r *Repository) ListEnabled(ctx context.Context) ([]Rule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, description, enabled, trigger_config, conditions, actions, created_at, updated_at
		 FROM automation_rules WHERE enabled = true ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list enabled rules: %w", err)
	}
	defer rows.Close()

	var rules []Rule
	for rows.Next() {
		var rule Rule
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.Description, &rule.Enabled, &rule.Trigger, &rule.Conditions, &rule.Actions, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan rule: %w", err)
		}
		rules = append(rules, rule)
	}
	return rules, nil
}
