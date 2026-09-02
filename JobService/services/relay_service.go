package services

import (
	"context"
	"log/slog"
	repo "github.com/gedyzed/JobFlow/JobService/repositories"
)

type IRelayService interface {
	PollAndPublish(ctx context.Context, limit int) error
}

type RelayService struct {
	repo   repo.IRelayRepo
	logger *slog.Logger
}

func NewRelayService(repo repo.IRelayRepo, logger *slog.Logger) IRelayService {
	return &RelayService{
		repo:   repo,
		logger: logger,
	}
}

func (s *RelayService) PollAndPublish(ctx context.Context, limit int) error {
	return s.repo.PollAndPublish(ctx, limit)
}
