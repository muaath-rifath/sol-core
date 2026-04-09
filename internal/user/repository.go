package user

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Upsert inserts or updates the user record from OIDC claims.
// created is true when the row was newly inserted (first login).
func (r *Repository) Upsert(ctx context.Context, keycloakID, email, name string) (*User, bool, error) {
	var u User
	var created bool
	err := r.pool.QueryRow(ctx,
		`INSERT INTO users (keycloak_id, email, name, updated_at)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (keycloak_id) DO UPDATE
		   SET email = EXCLUDED.email, name = EXCLUDED.name, updated_at = EXCLUDED.updated_at
		 RETURNING id, keycloak_id, email, name, created_at, updated_at, (xmax = 0) AS is_new`,
		keycloakID, email, name, time.Now(),
	).Scan(&u.ID, &u.KeycloakID, &u.Email, &u.Name, &u.CreatedAt, &u.UpdatedAt, &created)
	if err != nil {
		return nil, false, fmt.Errorf("upsert user: %w", err)
	}
	return &u, created, nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*User, error) {
	var u User
	err := r.pool.QueryRow(ctx,
		`SELECT id, keycloak_id, email, name, created_at, updated_at FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.KeycloakID, &u.Email, &u.Name, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}

func (r *Repository) GetByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := r.pool.QueryRow(ctx,
		`SELECT id, keycloak_id, email, name, created_at, updated_at FROM users WHERE LOWER(email) = LOWER($1)`, email,
	).Scan(&u.ID, &u.KeycloakID, &u.Email, &u.Name, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil
}
