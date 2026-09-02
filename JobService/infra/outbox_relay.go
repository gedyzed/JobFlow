package infra

import (
	"context"
	"log/slog"
	"time"

	"github.com/gedyzed/JobFlow/JobService/services"
)

type OutboxRelay struct {
	ticker  time.Duration
	service services.IRelayService
	logger  *slog.Logger
}

func NewOutboxRelay(ticker time.Duration, svs services.IRelayService, logger *slog.Logger) *OutboxRelay {
	return &OutboxRelay{
		ticker:  ticker,
		service: svs,
		logger:  logger,
	}
}

func (o *OutboxRelay) Start(ctx context.Context) {

	o.logger.Info("outbox relay started")
	ticker := time.NewTicker(o.ticker)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			o.Stop()
			return
		case <-ticker.C:
			o.pollAndPublish(ctx)
		}
	}
}

func (o *OutboxRelay) Stop() {
	// Implement graceful shutdown logic if needed
}

func (o *OutboxRelay) pollAndPublish(ctx context.Context) {
	err := o.service.PollAndPublish(ctx, 100)
	if err != nil {
		o.logger.Error("failed to poll and publish outbox events", "error", err)
	}
}
