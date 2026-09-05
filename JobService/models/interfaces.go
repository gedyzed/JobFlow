package models

import (
	"context"
)

type PublisherTarget struct {
	Queue      string
	Exchange   string
	RoutingKey string
}

type Publisher interface {
	Publish(ctx context.Context, msg []byte) error
	PublishEvent(ctx context.Context, event Outbox) error
	Close(ctx context.Context) error
}

type Consumer interface {
	Consume(ctx context.Context, handler func(ctx context.Context, body []byte) error) error
	Close(ctx context.Context) error
}

type IRMQClient interface {
	Connect(ctx context.Context) error
	Close(ctx context.Context) error

	// Topology management
	DeclareQueue(ctx context.Context, queueName string) error
	DeclareExchange(ctx context.Context, exchangeName, kind string) error
	BindQueue(ctx context.Context, queueName, exchangeName, routingKey string) error

	// Factory methods
	NewPublisher(ctx context.Context, target PublisherTarget) (Publisher, error)
	NewConsumer(ctx context.Context, queueName string) (Consumer, error)
}
