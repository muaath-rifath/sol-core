package room

import "context"

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, homeID string, req CreateRoomRequest) (*Room, error) {
	return s.repo.Create(ctx, homeID, req)
}

func (s *Service) Get(ctx context.Context, id, homeID string) (*Room, error) {
	return s.repo.GetByID(ctx, id, homeID)
}

func (s *Service) List(ctx context.Context, homeID string) ([]Room, error) {
	return s.repo.ListByHome(ctx, homeID)
}

func (s *Service) Update(ctx context.Context, id, homeID string, req UpdateRoomRequest) (*Room, error) {
	return s.repo.Update(ctx, id, homeID, req)
}

func (s *Service) Delete(ctx context.Context, id, homeID string) error {
	return s.repo.Delete(ctx, id, homeID)
}
