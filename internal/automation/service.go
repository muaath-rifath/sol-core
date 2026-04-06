package automation

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/muaathrifath/sol-core/internal/ai"
	"github.com/muaathrifath/sol-core/internal/device"
)

type Service struct {
	repo      *Repository
	deviceSvc *device.Service
	aiClient  *ai.Client
}

func NewService(repo *Repository, deviceSvc *device.Service, aiClient *ai.Client) *Service {
	return &Service{repo: repo, deviceSvc: deviceSvc, aiClient: aiClient}
}

func (s *Service) Create(ctx context.Context, req CreateRuleRequest) (*Rule, error) {
	rule := &Rule{
		ID:          uuid.NewString(),
		Name:        req.Name,
		Description: req.Description,
		Enabled:     true,
		Trigger:     req.Trigger,
		Conditions:  req.Conditions,
		Actions:     req.Actions,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.Create(ctx, rule); err != nil {
		return nil, fmt.Errorf("create rule: %w", err)
	}
	return rule, nil
}

func (s *Service) Get(ctx context.Context, id string) (*Rule, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]Rule, error) {
	return s.repo.List(ctx)
}

func (s *Service) Update(ctx context.Context, id string, req UpdateRuleRequest) (*Rule, error) {
	rule, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		rule.Name = *req.Name
	}
	if req.Description != nil {
		rule.Description = *req.Description
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	if req.Trigger != nil {
		rule.Trigger = *req.Trigger
	}
	if req.Conditions != nil {
		rule.Conditions = req.Conditions
	}
	if req.Actions != nil {
		rule.Actions = req.Actions
	}
	rule.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, rule); err != nil {
		return nil, fmt.Errorf("update rule: %w", err)
	}
	return rule, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) Evaluate(ctx context.Context, triggerType string, payload map[string]any) error {
	rules, err := s.repo.ListEnabled(ctx)
	if err != nil {
		return err
	}

	for _, rule := range rules {
		if rule.Trigger.Type != triggerType {
			continue
		}
		// TODO: evaluate conditions and execute actions
		_ = rule
	}

	return nil
}
