package infra

import (
	"context"
	"log/slog"

	"github.com/cockroachdb/errors"

	"github.com/gedyzed/JobFlow/JobService/infra/configs"
	"github.com/gedyzed/JobFlow/JobService/models"
	rmq "github.com/rabbitmq/rabbitmq-amqp-go-client/pkg/rabbitmqamqp"
)

type RabbitMQService struct {
	Config configs.RabbitMQConfig
	logger *slog.Logger
}

func NewRabbitMQService(config configs.RabbitMQConfig, logger *slog.Logger) *RabbitMQService {
	return &RabbitMQService{
		Config: config,
		logger: logger,
	}
}

// IRMQClient defines the contract for a RabbitMQ client.
// It operates on a single queue configured via RabbitMQConfig.Name.
type IRMQClient interface {
	Connect(ctx context.Context) error
	Publish(ctx context.Context, msg []byte) error
	Consume(ctx context.Context, handler func(ctx context.Context, body []byte) error) error
	Close(ctx context.Context) error
}

// RMQClient manages a single RabbitMQ connection bound to one queue.
type RMQClient struct {
	service   *RabbitMQService
	conn      *rmq.AmqpConnection
	env       *rmq.Environment
	publisher *rmq.Publisher
	consumer  *rmq.Consumer
}

func NewRMQClient(service *RabbitMQService) *RMQClient {
	return &RMQClient{
		service: service,
	}
}

// Connect establishes the AMQP connection and declares the queue.
func (c *RMQClient) Connect(ctx context.Context) error {
	c.env = rmq.NewEnvironment(c.service.Config.URL, nil)

	conn, err := c.env.NewConnection(ctx)
	if err != nil {
		c.service.logger.Error("failed to create RabbitMQ connection", "error", err)
		return errors.Wrap(err, "failed to create RabbitMQ connection")
	}
	c.conn = conn

	// Declare the queue upfront so Publish and Consume are ready to go.
	queueName := c.service.Config.Name
	_, err = c.conn.Management().DeclareQueue(ctx, &rmq.QuorumQueueSpecification{Name: queueName})
	if err != nil {
		c.service.logger.Error("failed to declare queue", "queue", queueName, "error", err)
		return errors.Wrapf(err, "failed to declare queue %q", queueName)
	}

	c.service.logger.Info("RabbitMQ connection established", "queue", queueName)
	return nil
}

// Publish sends raw bytes to the configured queue.
func (c *RMQClient) Publish(ctx context.Context, msg []byte) error {
	if c.conn == nil {
		if err := c.Connect(ctx); err != nil {
			return errors.Wrap(err, "RabbitMQ unavailable")
		}
	}

	if c.publisher == nil {
		queueName := c.service.Config.Name
		pub, err := c.conn.NewPublisher(ctx, &rmq.QueueAddress{Queue: queueName}, nil)
		if err != nil {
			c.service.logger.Error("failed to create publisher", "queue", queueName, "error", err)
			return errors.Wrapf(err, "failed to create publisher for queue %q", queueName)
		}
		c.publisher = pub
		c.service.logger.Info("publisher created", "queue", queueName)
	}

	res, err := c.publisher.Publish(ctx, rmq.NewMessage(msg))
	if err != nil {
		c.service.logger.Error("failed to publish message", "error", err)
		return errors.Wrap(err, "failed to publish message")
	}

	switch res.Outcome.(type) {
	case *rmq.StateAccepted:
		c.service.logger.Info("message published", "queue", c.service.Config.Name, "size", len(msg))
	default:
		c.service.logger.Error("unexpected publish outcome", "outcome", res.Outcome)
		return errors.New("unexpected publish outcome")
	}

	return nil
}

// Consume listens for messages on the configured queue and delegates each to the handler.
func (c *RMQClient) Consume(ctx context.Context, handler func(ctx context.Context, body []byte) error) error {
	queueName := c.service.Config.Name

	if c.consumer == nil {
		con, err := c.conn.NewConsumer(ctx, queueName, nil)
		if err != nil {
			c.service.logger.Error("failed to create consumer", "queue", queueName, "error", err)
			return errors.Wrapf(err, "failed to create consumer for queue %q", queueName)
		}
		c.consumer = con
		c.service.logger.Info("consumer created", "queue", queueName)
	}

	c.service.logger.Info("consumer started, waiting for messages", "queue", queueName)

	for {
		delivery, err := c.consumer.Receive(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				c.service.logger.Info("consumer stopped: context cancelled", "queue", queueName)
				return nil
			}
			c.service.logger.Error("failed to receive message", "queue", queueName, "error", err)
			return errors.Wrap(err, "failed to receive message")
		}

		msg := delivery.Message()
		var body []byte
		if len(msg.Data) > 0 {
			body = msg.Data[0]
		}

		if err := handler(ctx, body); err != nil {
			c.service.logger.Error("handler failed", "queue", queueName, "error", err)
			// Still accept to avoid redelivery loops.
			// Consider a dead-letter strategy for production.
		}

		if err := delivery.Accept(ctx); err != nil {
			c.service.logger.Error("failed to accept delivery", "queue", queueName, "error", err)
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

func (c *RMQClient) PublishEvent(ctx context.Context, event models.Outbox) error {
	return c.Publish(ctx, event.Payload)
}
