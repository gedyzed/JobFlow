package infra

import (
	"context"
	"log/slog"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/gedyzed/JobFlow/JobService/models"
)

// ExchangeRelayPublisher routes outbox events to a topic exchange,
// using the event's EventType as the routing key.
// Publishers are lazily created and cached per routing key.
type ExchangeRelayPublisher struct {
	client     models.IRMQClient
	exchange   string
	publishers map[string]models.Publisher
	mu         sync.Mutex
	logger     *slog.Logger
}

func NewExchangeRelayPublisher(client models.IRMQClient, exchange string, logger *slog.Logger) *ExchangeRelayPublisher {
	return &ExchangeRelayPublisher{
		client:     client,
		exchange:   exchange,
		publishers: make(map[string]models.Publisher),
		logger:     logger,
	}
}

// PublishEvent publishes the outbox event to the exchange using EventType as the routing key.
func (p *ExchangeRelayPublisher) PublishEvent(ctx context.Context, event models.Outbox) error {
	routingKey := event.EventType

	p.mu.Lock()
	pub, exists := p.publishers[routingKey]
	if !exists {
		var err error
		pub, err = p.client.NewPublisher(ctx, models.PublisherTarget{
			Exchange:   p.exchange,
			RoutingKey: routingKey,
		})
		if err != nil {
			p.mu.Unlock()
			p.logger.Error("failed to create publisher for routing key", "exchange", p.exchange, "routing_key", routingKey, "error", err)
			return errors.Wrapf(err, "create publisher for exchange %q routing key %q", p.exchange, routingKey)
		}
		p.publishers[routingKey] = pub
		p.logger.Info("created publisher for routing key", "exchange", p.exchange, "routing_key", routingKey)
	}
	p.mu.Unlock()

	return pub.Publish(ctx, event.Payload)
}

// Close closes all cached publishers.
func (p *ExchangeRelayPublisher) Close(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for key, pub := range p.publishers {
		if err := pub.Close(ctx); err != nil {
			p.logger.Error("failed to close publisher", "routing_key", key, "error", err)
		}
	}
	p.publishers = make(map[string]models.Publisher)
	return nil
}
