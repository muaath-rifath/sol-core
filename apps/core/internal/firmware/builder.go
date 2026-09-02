package firmware

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Builder struct {
	rdb    *redis.Client
	builds *BuildRepository
}

func NewBuilder(rdb *redis.Client, builds *BuildRepository) *Builder {
	return &Builder{
		rdb:    rdb,
		builds: builds,
	}
}

type BuildJobPayload struct {
	JobID       string `json:"job_id"`
	TemplateID  string `json:"template_id"`
	TargetBoard string `json:"target_board"`
}

func (b *Builder) Submit(ctx context.Context, templateID, targetBoard string) (string, error) {
	jobID := uuid.NewString()
	
	job := &FirmwareBuild{
		ID:          jobID,
		TemplateID:  templateID,
		TargetBoard: targetBoard,
		Status:      StatusQueued,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := b.builds.Create(ctx, job); err != nil {
		return "", err
	}

	payload := BuildJobPayload{
		JobID:       jobID,
		TemplateID:  templateID,
		TargetBoard: targetBoard,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	// Push to Redis queue
	if err := b.rdb.LPush(ctx, "firmware_build_queue", data).Err(); err != nil {
		return "", err
	}

	return jobID, nil
}
