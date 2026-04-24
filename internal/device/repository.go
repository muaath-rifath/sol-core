package device

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, d *Device) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO devices (id, name, type, room_id, state, metadata, firmware_id, online, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		d.ID, d.Name, d.Type, d.RoomID, d.State, d.Metadata, d.FirmwareID, d.Online, d.CreatedAt, d.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert device: %w", err)
	}
	return nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*Device, error) {
	var d Device
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, type, room_id, state, metadata, firmware_id, online, created_at, updated_at
		 FROM devices WHERE id = $1`, id,
	).Scan(&d.ID, &d.Name, &d.Type, &d.RoomID, &d.State, &d.Metadata, &d.FirmwareID, &d.Online, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get device: %w", err)
	}
	return &d, nil
}

func (r *Repository) List(ctx context.Context) ([]Device, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, type, room_id, state, metadata, firmware_id, online, created_at, updated_at
		 FROM devices ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.Name, &d.Type, &d.RoomID, &d.State, &d.Metadata, &d.FirmwareID, &d.Online, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		devices = append(devices, d)
	}
	return devices, nil
}

func (r *Repository) ListPaginated(ctx context.Context, cursorTime *time.Time, cursorID string, limit int) ([]Device, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, type, room_id, state, metadata, firmware_id, online, created_at, updated_at
		 FROM devices
		 WHERE ($1::timestamptz IS NULL
		        OR created_at < $1
		        OR (created_at = $1 AND id::text < $2))
		 ORDER BY created_at DESC, id::text DESC
		 LIMIT $3`, cursorTime, cursorID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list devices paginated: %w", err)
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.Name, &d.Type, &d.RoomID, &d.State, &d.Metadata, &d.FirmwareID, &d.Online, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		devices = append(devices, d)
	}
	return devices, nil
}

func (r *Repository) ListByRoom(ctx context.Context, roomID string) ([]Device, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, type, room_id, state, metadata, firmware_id, online, created_at, updated_at
		 FROM devices WHERE room_id = $1 ORDER BY created_at DESC`, roomID,
	)
	if err != nil {
		return nil, fmt.Errorf("list devices by room: %w", err)
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.Name, &d.Type, &d.RoomID, &d.State, &d.Metadata, &d.FirmwareID, &d.Online, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		devices = append(devices, d)
	}
	return devices, nil
}

func (r *Repository) ListByRoomPaginated(ctx context.Context, roomID string, cursorTime *time.Time, cursorID string, limit int) ([]Device, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, type, room_id, state, metadata, firmware_id, online, created_at, updated_at
		 FROM devices
		 WHERE room_id = $1
		   AND ($2::timestamptz IS NULL
		        OR created_at < $2
		        OR (created_at = $2 AND id::text < $3))
		 ORDER BY created_at DESC, id::text DESC
		 LIMIT $4`, roomID, cursorTime, cursorID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list devices by room paginated: %w", err)
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.Name, &d.Type, &d.RoomID, &d.State, &d.Metadata, &d.FirmwareID, &d.Online, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		devices = append(devices, d)
	}
	return devices, nil
}

func (r *Repository) RoomBelongsToHome(ctx context.Context, roomID, homeID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM rooms WHERE id = $1 AND home_id = $2)`, roomID, homeID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check room home membership: %w", err)
	}
	return exists, nil
}

func (r *Repository) GetByIDInRoom(ctx context.Context, id, roomID string) (*Device, error) {
	var d Device
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, type, room_id, state, metadata, firmware_id, online, created_at, updated_at
		 FROM devices WHERE id = $1 AND room_id = $2`, id, roomID,
	).Scan(&d.ID, &d.Name, &d.Type, &d.RoomID, &d.State, &d.Metadata, &d.FirmwareID, &d.Online, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("get device in room: %w", err)
	}
	return &d, nil
}

func (r *Repository) Update(ctx context.Context, d *Device) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE devices SET name=$2, type=$3, room_id=$4, state=$5, metadata=$6, firmware_id=$7, online=$8, updated_at=$9
		 WHERE id=$1`,
		d.ID, d.Name, d.Type, d.RoomID, d.State, d.Metadata, d.FirmwareID, d.Online, d.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update device: %w", err)
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM devices WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete device: %w", err)
	}
	return nil
}

func (r *Repository) InsertTelemetry(ctx context.Context, tp *TelemetryPoint) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO device_telemetry (device_id, timestamp, data) VALUES ($1, $2, $3)`,
		tp.DeviceID, tp.Timestamp, tp.Data,
	)
	if err != nil {
		return fmt.Errorf("insert telemetry: %w", err)
	}
	return nil
}

func (r *Repository) GetRecentTelemetry(ctx context.Context, deviceID string, limit int) ([]TelemetryPoint, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT device_id, timestamp, data
		 FROM device_telemetry WHERE device_id = $1 ORDER BY timestamp DESC LIMIT $2`, deviceID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get telemetry: %w", err)
	}
	defer rows.Close()

	var points []TelemetryPoint
	for rows.Next() {
		var tp TelemetryPoint
		if err := rows.Scan(&tp.DeviceID, &tp.Timestamp, &tp.Data); err != nil {
			return nil, fmt.Errorf("scan telemetry: %w", err)
		}
		points = append(points, tp)
	}
	return points, nil
}

func (r *Repository) CreateAppliance(ctx context.Context, a *Appliance) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO appliances (id, device_id, room_id, name, type, channel, gpio_pin, active_low, state, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		a.ID, a.DeviceID, a.RoomID, a.Name, a.Type, a.Channel, a.GPIOPin, a.ActiveLow, a.State, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert appliance: %w", err)
	}
	return nil
}

func (r *Repository) GetApplianceByID(ctx context.Context, id string) (*Appliance, error) {
	var a Appliance
	err := r.pool.QueryRow(ctx,
		`SELECT id, device_id, room_id, name, type, channel, gpio_pin, active_low, state, created_at, updated_at
		 FROM appliances WHERE id = $1`, id,
	).Scan(&a.ID, &a.DeviceID, &a.RoomID, &a.Name, &a.Type, &a.Channel, &a.GPIOPin, &a.ActiveLow, &a.State, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get appliance: %w", err)
	}
	return &a, nil
}

func (r *Repository) ListAppliancesByRoom(ctx context.Context, roomID string) ([]Appliance, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, device_id, room_id, name, type, channel, gpio_pin, active_low, state, created_at, updated_at
		 FROM appliances WHERE room_id = $1 ORDER BY created_at DESC`, roomID,
	)
	if err != nil {
		return nil, fmt.Errorf("list appliances by room: %w", err)
	}
	defer rows.Close()

	var appliances []Appliance
	for rows.Next() {
		var a Appliance
		if err := rows.Scan(&a.ID, &a.DeviceID, &a.RoomID, &a.Name, &a.Type, &a.Channel, &a.GPIOPin, &a.ActiveLow, &a.State, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan appliance: %w", err)
		}
		appliances = append(appliances, a)
	}
	return appliances, nil
}

func (r *Repository) ListAppliancesByDevice(ctx context.Context, deviceID string) ([]Appliance, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, device_id, room_id, name, type, channel, gpio_pin, active_low, state, created_at, updated_at
		 FROM appliances WHERE device_id = $1 ORDER BY created_at DESC`, deviceID,
	)
	if err != nil {
		return nil, fmt.Errorf("list appliances by device: %w", err)
	}
	defer rows.Close()

	var appliances []Appliance
	for rows.Next() {
		var a Appliance
		if err := rows.Scan(&a.ID, &a.DeviceID, &a.RoomID, &a.Name, &a.Type, &a.Channel, &a.GPIOPin, &a.ActiveLow, &a.State, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan appliance: %w", err)
		}
		appliances = append(appliances, a)
	}
	return appliances, nil
}

func (r *Repository) ListAllAppliances(ctx context.Context) ([]Appliance, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, device_id, room_id, name, type, channel, gpio_pin, active_low, state, created_at, updated_at
		 FROM appliances ORDER BY name ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list all appliances: %w", err)
	}
	defer rows.Close()

	var appliances []Appliance
	for rows.Next() {
		var a Appliance
		if err := rows.Scan(&a.ID, &a.DeviceID, &a.RoomID, &a.Name, &a.Type, &a.Channel, &a.GPIOPin, &a.ActiveLow, &a.State, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan appliance: %w", err)
		}
		appliances = append(appliances, a)
	}
	return appliances, nil
}

func (r *Repository) UpdateAppliance(ctx context.Context, a *Appliance) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE appliances SET name = $2, channel = $3, gpio_pin = $4, active_low = $5, state = $6, updated_at = $7
		 WHERE id = $1`,
		a.ID, a.Name, a.Channel, a.GPIOPin, a.ActiveLow, a.State, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update appliance: %w", err)
	}
	return nil
}

func (r *Repository) DeleteAppliance(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM appliances WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete appliance: %w", err)
	}
	return nil
}
