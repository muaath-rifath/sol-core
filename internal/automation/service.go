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
		if !s.matchTrigger(rule.Trigger, payload) {
			continue
		}
		if !s.evalConditions(ctx, rule.Conditions) {
			continue
		}
		s.execActions(ctx, rule)
	}

	return nil
}

// matchTrigger returns true when the trigger config matches the event payload.
// For device_state triggers, trigger.Config["device_id"] must equal payload["device_id"].
func (s *Service) matchTrigger(t Trigger, payload map[string]any) bool {
	switch t.Type {
	case "device_state":
		cfgDeviceID, _ := t.Config["device_id"].(string)
		payloadDeviceID, _ := payload["device_id"].(string)
		return cfgDeviceID != "" && cfgDeviceID == payloadDeviceID
	default:
		return false
	}
}

// evalConditions returns true when all conditions pass (vacuously true for empty list).
func (s *Service) evalConditions(ctx context.Context, conds []Cond) bool {
	for _, c := range conds {
		switch c.Type {
		case "device_state":
			if !s.evalDeviceStateCond(ctx, c) {
				return false
			}
		default:
			// Unknown condition types are logged and treated as blocking.
			fmt.Printf("automation: unsupported condition type %q — skipping rule\n", c.Type)
			return false
		}
	}
	return true
}

// evalDeviceStateCond checks a single device_state condition.
// Config keys: device_id (string), field (string), value (any).
// Only "eq" operator is supported; field must equal value exactly.
func (s *Service) evalDeviceStateCond(ctx context.Context, c Cond) bool {
	deviceID, _ := c.Config["device_id"].(string)
	field, _ := c.Config["field"].(string)
	want := c.Config["value"]
	if deviceID == "" || field == "" {
		return false
	}
	d, err := s.deviceSvc.Get(ctx, deviceID)
	if err != nil {
		return false
	}
	if d.State == nil {
		return false
	}
	got, ok := d.State[field]
	if !ok {
		return false
	}
	return fmt.Sprintf("%v", got) == fmt.Sprintf("%v", want)
}

// execActions runs each action in the rule.
func (s *Service) execActions(ctx context.Context, rule Rule) {
	for _, a := range rule.Actions {
		switch a.Type {
		case "device_command":
			s.execDeviceCommand(ctx, rule, a)
		default:
			fmt.Printf("automation: unsupported action type %q in rule %s — skipping\n", a.Type, rule.ID)
		}
	}
}

// execDeviceCommand sends a device command defined in the action config.
// Config keys: device_id (string), action (string), params (map[string]any).
func (s *Service) execDeviceCommand(ctx context.Context, rule Rule, a Action) {
	deviceID, _ := a.Config["device_id"].(string)
	action, _ := a.Config["action"].(string)
	params, _ := a.Config["params"].(map[string]any)
	if deviceID == "" || action == "" {
		fmt.Printf("automation: device_command in rule %s missing device_id or action\n", rule.ID)
		return
	}
	cmd := device.Command{
		DeviceID: deviceID,
		Action:   action,
		Params:   params,
	}
	if err := s.deviceSvc.SendCommand(ctx, cmd); err != nil {
		fmt.Printf("automation: rule %s device_command failed: %v\n", rule.ID, err)
	}
}
