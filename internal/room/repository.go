package room

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, homeID string, req CreateRoomRequest) (*Room, error) {
	room := &Room{
		ID:        uuid.NewString(),
		HomeID:    homeID,
		Name:      req.Name,
		Floor:     req.Floor,
		Metadata:  map[string]any{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err := r.pool.Exec(ctx,
		`INSERT INTO rooms (id, name, floor, metadata, home_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		room.ID, room.Name, room.Floor, room.Metadata, room.HomeID, room.CreatedAt, room.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create room: %w", err)
	}
	return room, nil
}

func (r *Repository) GetByID(ctx context.Context, id, homeID string) (*Room, error) {
	var room Room
	err := r.pool.QueryRow(ctx,
		`SELECT id, home_id, name, floor, metadata, created_at, updated_at
		 FROM rooms WHERE id = $1 AND home_id = $2`,
		id, homeID,
	).Scan(&room.ID, &room.HomeID, &room.Name, &room.Floor, &room.Metadata, &room.CreatedAt, &room.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get room: %w", err)
	}
	return &room, nil
}

func (r *Repository) ListByHome(ctx context.Context, homeID string) ([]Room, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, home_id, name, floor, metadata, created_at, updated_at
		 FROM rooms WHERE home_id = $1 ORDER BY created_at ASC`,
		homeID,
	)
	if err != nil {
		return nil, fmt.Errorf("list rooms: %w", err)
	}
	defer rows.Close()

	var rooms []Room
	for rows.Next() {
		var room Room
		if err := rows.Scan(&room.ID, &room.HomeID, &room.Name, &room.Floor, &room.Metadata, &room.CreatedAt, &room.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan room: %w", err)
		}
		rooms = append(rooms, room)
	}
	return rooms, nil
}

func (r *Repository) Update(ctx context.Context, id, homeID string, req UpdateRoomRequest) (*Room, error) {
	room, err := r.GetByID(ctx, id, homeID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		room.Name = *req.Name
	}
	if req.Floor != nil {
		room.Floor = req.Floor
	}
	room.UpdatedAt = time.Now()

	_, err = r.pool.Exec(ctx,
		`UPDATE rooms SET name=$3, floor=$4, updated_at=$5 WHERE id=$1 AND home_id=$2`,
		room.ID, room.HomeID, room.Name, room.Floor, room.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update room: %w", err)
	}
	return room, nil
}

func (r *Repository) Delete(ctx context.Context, id, homeID string) error {
	result, err := r.pool.Exec(ctx, `DELETE FROM rooms WHERE id=$1 AND home_id=$2`, id, homeID)
	if err != nil {
		return fmt.Errorf("delete room: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("room not found")
	}
	return nil
}
