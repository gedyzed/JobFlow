package infra

import (
	"context"
	"log/slog"

	"github.com/cockroachdb/errors"

	"github.com/gedyzed/JobFlow/JobService/infra/configs"
	"github.com/gedyzed/JobFlow/JobService/repositories"
	rmq "github.com/rabbitmq/rabbitmq-amqp-go-client/pkg/rabbitmqamqp"
)

type RabbitMQService struct {
	Config configs.RabbitMQConfig
	repo   repositories.IJobRepo
	logger *slog.Logger
}

func NewRabbitMQService(config configs.RabbitMQConfig, repo repositories.IJobRepo, logger *slog.Logger) *RabbitMQService {
	return &RabbitMQService{
		Config: config,
		repo:   repo,
		logger: logger,
	}
}

// IRMQClient defines the contract for a RabbitMQ client.
// Connect establishes the connection. Publishers and consumers are created on demand per queue.
type IRMQClient interface {
	Connect(ctx context.Context) error
	NewPublisher(ctx context.Context, queueName string) (*rmq.Publisher, error)
	NewConsumer(ctx context.Context, queueName string) (*rmq.Consumer, error)
	Publish(ctx context.Context, publisher *rmq.Publisher, message []byte) error
	Consume(ctx context.Context, consumer *rmq.Consumer, handler func(ctx context.Context, body []byte) error) error
	Close(ctx context.Context) error
}

// RMQClient manages a single RabbitMQ connection and environment.
// Publishers and consumers are created on demand via factory methods.
type RMQClient struct {
	service RabbitMQService
	conn    *rmq.AmqpConnection
	env     *rmq.Environment
}

func NewRMQClient(service RabbitMQService) *RMQClient {
	return &RMQClient{
		service: service,
	}
}

// Connect establishes the AMQP connection. No queues are declared here.
func (c *RMQClient) Connect(ctx context.Context) error {
	c.env = rmq.NewEnvironment(c.service.Config.URL, nil)

	conn, err := c.env.NewConnection(ctx)
	if err != nil {
		c.service.logger.Error("failed to create RabbitMQ connection", "error", err)
		return errors.Wrap(err, "failed to create RabbitMQ connection")
	}
	c.conn = conn

	c.service.logger.Info("RabbitMQ connection established")
	return nil
}

// declareQueue ensures the quorum queue exists. Called internally before creating publishers/consumers.
func (c *RMQClient) declareQueue(ctx context.Context, queueName string) error {
	_, err := c.conn.Management().DeclareQueue(ctx, &rmq.QuorumQueueSpecification{Name: queueName})
	if err != nil {
		c.service.logger.Error("failed to declare queue", "queue", queueName, "error", err)
		return errors.Wrapf(err, "failed to declare queue %q", queueName)
	}
	return nil
}

// NewPublisher declares the queue and creates a publisher bound to it.
func (c *RMQClient) NewPublisher(ctx context.Context, queueName string) (*rmq.Publisher, error) {
	if err := c.declareQueue(ctx, queueName); err != nil {
		return nil, err
	}

	publisher, err := c.conn.NewPublisher(ctx, &rmq.QueueAddress{Queue: queueName}, nil)
	if err != nil {
		c.service.logger.Error("failed to create publisher", "queue", queueName, "error", err)
		return nil, errors.Wrapf(err, "failed to create publisher for queue %q", queueName)
	}

	c.service.logger.Info("publisher created", "queue", queueName)
	return publisher, nil
}

// NewConsumer declares the queue and creates a consumer bound to it.
func (c *RMQClient) NewConsumer(ctx context.Context, queueName string) (*rmq.Consumer, error) {
	if err := c.declareQueue(ctx, queueName); err != nil {
		return nil, err
	}

	consumer, err := c.conn.NewConsumer(ctx, queueName, nil)
	if err != nil {
		c.service.logger.Error("failed to create consumer", "queue", queueName, "error", err)
		return nil, errors.Wrapf(err, "failed to create consumer for queue %q", queueName)
	}

	c.service.logger.Info("consumer created", "queue", queueName)
	return consumer, nil
}

// Publish sends a message through the given publisher and verifies the outcome.
func (c *RMQClient) Publish(ctx context.Context, publisher *rmq.Publisher, message []byte) error {
	res, err := publisher.Publish(ctx, rmq.NewMessage(message))
	if err != nil {
		c.service.logger.Error("failed to publish message", "error", err)
		return errors.Wrap(err, "failed to publish message")
	}

	switch res.Outcome.(type) {
	case *rmq.StateAccepted:
		c.service.logger.Info("message published successfully", "size", len(message))
	default:
		c.service.logger.Error("unexpected publish outcome", "outcome", res.Outcome)
		return errors.New("unexpected publish outcome")
	}

	return nil
}

// Consume listens for messages on the given consumer and delegates each to the handler.
// It blocks until the context is cancelled or an unrecoverable error occurs.
func (c *RMQClient) Consume(ctx context.Context, consumer *rmq.Consumer, handler func(ctx context.Context, body []byte) error) error {
	c.service.logger.Info("consumer started, waiting for messages")

	for {
		delivery, err := consumer.Receive(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				c.service.logger.Info("consumer stopped: context cancelled")
				return nil
			}
			c.service.logger.Error("failed to receive message", "error", err)
			return errors.Wrap(err, "failed to receive message")
		}

		msg := delivery.Message()
		var body []byte
		if len(msg.Data) > 0 {
			body = msg.Data[0]
		}

		if err := handler(ctx, body); err != nil {
			c.service.logger.Error("handler failed", "error", err)
			// Still accept the message to avoid redelivery loops.
			// Consider a dead-letter strategy for production.
		}

		if err := delivery.Accept(ctx); err != nil {
			c.service.logger.Error("failed to accept delivery", "error", err)
			return errors.Wrap(err, "failed to accept delivery")
		}
	}
}

// Close tears down the connection and environment.
func (c *RMQClient) Close(ctx context.Context) error {
	if c.conn != nil {
		if err := c.conn.Close(ctx); err != nil {
			c.service.logger.Error("failed to close RabbitMQ connection", "error", err)
			return errors.Wrap(err, "failed to close RabbitMQ connection")
		}
	}

	if c.env != nil {
		if err := c.env.CloseConnections(ctx); err != nil {
			c.service.logger.Error("failed to close RabbitMQ environment", "error", err)
			return errors.Wrap(err, "failed to close RabbitMQ environment")
		}
	}

	c.service.logger.Info("RabbitMQ client closed")
	return nil
}
