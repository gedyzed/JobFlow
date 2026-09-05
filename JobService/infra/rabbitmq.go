package infra

import (
	"context"
	"log/slog"
	"sync"

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

// RMQPublisher publishes messages to a specified queue or exchange.
type RMQPublisher struct {
	target    models.PublisherTarget
	publisher *rmq.Publisher
	logger    *slog.Logger
}

func (p *RMQPublisher) Publish(ctx context.Context, msg []byte) error {
	res, err := p.publisher.Publish(ctx, rmq.NewMessage(msg))
	if err != nil {
		p.logger.Error("failed to publish message", "target", p.target, "error", err)
		return errors.Wrap(err, "failed to publish message")
	}

	switch res.Outcome.(type) {
	case *rmq.StateAccepted:
		p.logger.Debug("message published successfully", "target", p.target, "size", len(msg))
		return nil
	default:
		p.logger.Error("unexpected publish outcome", "outcome", res.Outcome, "target", p.target)
		return errors.Errorf("unexpected publish outcome: %v", res.Outcome)
	}
}

func (p *RMQPublisher) PublishEvent(ctx context.Context, event models.Outbox) error {
	return p.Publish(ctx, event.Payload)
}

func (p *RMQPublisher) Close(ctx context.Context) error {
	if p.publisher != nil {
		if err := p.publisher.Close(ctx); err != nil {
			p.logger.Error("failed to close publisher", "target", p.target, "error", err)
			return errors.Wrap(err, "failed to close publisher")
		}
	}
	return nil
}

// RMQConsumer consumes messages from a specified queue.
type RMQConsumer struct {
	queueName string
	consumer  *rmq.Consumer
	logger    *slog.Logger
}

func (c *RMQConsumer) Consume(ctx context.Context, handler func(ctx context.Context, body []byte) error) error {
	c.logger.Info("consumer started, waiting for messages", "queue", c.queueName)

	for {
		delivery, err := c.consumer.Receive(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				c.logger.Info("consumer stopped: context canceled", "queue", c.queueName)
				return nil
			}
			c.logger.Error("failed to receive message", "queue", c.queueName, "error", err)
			return errors.Wrap(err, "failed to receive message")
		}

		msg := delivery.Message()
		var body []byte
		if len(msg.Data) > 0 {
			body = msg.Data[0]
		}

		if err := handler(ctx, body); err != nil {
			c.logger.Error("handler failed to process message", "queue", c.queueName, "error", err)
			if discardErr := delivery.Discard(ctx, nil); discardErr != nil {
				c.logger.Error("failed to discard failed message", "queue", c.queueName, "error", discardErr)
			}
			continue
		}

		if err := delivery.Accept(ctx); err != nil {
			c.logger.Error("failed to accept delivery", "queue", c.queueName, "error", err)
			return errors.Wrap(err, "failed to accept delivery")
		}
	}
}

func (c *RMQConsumer) Close(ctx context.Context) error {
	if c.consumer != nil {
		if err := c.consumer.Close(ctx); err != nil {
			c.logger.Error("failed to close consumer", "queue", c.queueName, "error", err)
			return errors.Wrap(err, "failed to close consumer")
		}
	}
	return nil
}

// RMQClient manages the RabbitMQ connection and creates publishers and consumers.
type RMQClient struct {
	service          *RabbitMQService
	conn             *rmq.AmqpConnection
	env              *rmq.Environment
	mu               sync.Mutex
	defaultPublisher models.Publisher
	defaultConsumer  models.Consumer
}

func NewRMQClient(service *RabbitMQService) *RMQClient {
	return &RMQClient{
		service: service,
	}
}

// Connect establishes the AMQP environment and connection.
func (c *RMQClient) Connect(ctx context.Context) error {
	c.env = rmq.NewEnvironment(c.service.Config.URL, nil)

	conn, err := c.env.NewConnection(ctx)
	if err != nil {
		c.service.logger.Error("failed to create RabbitMQ connection", "error", err)
		return errors.Wrap(err, "failed to create RabbitMQ connection")
	}
	c.conn = conn

	// If a default queue is configured, declare it for ease of use
	if c.service.Config.Name != "" {
		if err := c.DeclareQueue(ctx, c.service.Config.Name); err != nil {
			c.service.logger.Warn("failed to declare default queue", "queue", c.service.Config.Name, "error", err)
		}
	}

	c.service.logger.Info("RabbitMQ connection established")
	return nil
}

// DeclareQueue declares a quorum queue on RabbitMQ.
func (c *RMQClient) DeclareQueue(ctx context.Context, queueName string) error {
	if c.conn == nil {
		return errors.New("RabbitMQ connection is not established")
	}

	_, err := c.conn.Management().DeclareQueue(ctx, &rmq.QuorumQueueSpecification{Name: queueName})
	if err != nil {
		c.service.logger.Error("failed to declare queue", "queue", queueName, "error", err)
		return errors.Wrapf(err, "failed to declare queue %q", queueName)
	}

	c.service.logger.Info("queue declared successfully", "queue", queueName)
	return nil
}

// DeclareExchange declares an exchange of specified kind ("direct", "topic", "fanout").
func (c *RMQClient) DeclareExchange(ctx context.Context, exchangeName, kind string) error {
	if c.conn == nil {
		return errors.New("RabbitMQ connection is not established")
	}

	var spec rmq.IExchangeSpecification
	switch kind {
	case "topic":
		spec = &rmq.TopicExchangeSpecification{Name: exchangeName}
	case "fanout":
		spec = &rmq.FanOutExchangeSpecification{Name: exchangeName}
	case "direct", "":
		spec = &rmq.DirectExchangeSpecification{Name: exchangeName}
	default:
		spec = &rmq.CustomExchangeSpecification{Name: exchangeName, ExchangeTypeName: kind}
	}

	_, err := c.conn.Management().DeclareExchange(ctx, spec)
	if err != nil {
		c.service.logger.Error("failed to declare exchange", "exchange", exchangeName, "kind", kind, "error", err)
		return errors.Wrapf(err, "failed to declare exchange %q", exchangeName)
	}

	c.service.logger.Info("exchange declared successfully", "exchange", exchangeName, "kind", kind)
	return nil
}

// BindQueue binds a queue to an exchange using a routing key pattern.
func (c *RMQClient) BindQueue(ctx context.Context, queueName, exchangeName, routingKey string) error {
	if c.conn == nil {
		return errors.New("RabbitMQ connection is not established")
	}

	spec := &rmq.ExchangeToQueueBindingSpecification{
		SourceExchange:   exchangeName,
		DestinationQueue: queueName,
		BindingKey:       routingKey,
	}

	_, err := c.conn.Management().Bind(ctx, spec)
	if err != nil {
		c.service.logger.Error("failed to bind queue to exchange", "queue", queueName, "exchange", exchangeName, "routing_key", routingKey, "error", err)
		return errors.Wrapf(err, "failed to bind queue %q to exchange %q", queueName, exchangeName)
	}

	c.service.logger.Info("queue bound to exchange successfully", "queue", queueName, "exchange", exchangeName, "routing_key", routingKey)
	return nil
}

// NewPublisher creates an independent publisher targeting either a queue or exchange + routing key.
func (c *RMQClient) NewPublisher(ctx context.Context, target models.PublisherTarget) (models.Publisher, error) {
	if c.conn == nil {
		return nil, errors.New("RabbitMQ connection is not established")
	}

	var address rmq.ITargetAddress
	if target.Exchange != "" {
		address = &rmq.ExchangeAddress{
			Exchange: target.Exchange,
			Key:      target.RoutingKey,
		}
	} else if target.Queue != "" {
		address = &rmq.QueueAddress{
			Queue: target.Queue,
		}
	} else {
		return nil, errors.New("publisher target must have either Queue or Exchange specified")
	}

	pub, err := c.conn.NewPublisher(ctx, address, nil)
	if err != nil {
		c.service.logger.Error("failed to create publisher", "target", target, "error", err)
		return nil, errors.Wrapf(err, "failed to create publisher for target %+v", target)
	}

	c.service.logger.Info("publisher created successfully", "target", target)
	return &RMQPublisher{
		target:    target,
		publisher: pub,
		logger:    c.service.logger,
	}, nil
}

// NewConsumer creates an independent consumer listening on a specific queue.
func (c *RMQClient) NewConsumer(ctx context.Context, queueName string) (models.Consumer, error) {
	if c.conn == nil {
		return nil, errors.New("RabbitMQ connection is not established")
	}

	con, err := c.conn.NewConsumer(ctx, queueName, nil)
	if err != nil {
		c.service.logger.Error("failed to create consumer", "queue", queueName, "error", err)
		return nil, errors.Wrapf(err, "failed to create consumer for queue %q", queueName)
	}

	c.service.logger.Info("consumer created successfully", "queue", queueName)
	return &RMQConsumer{
		queueName: queueName,
		consumer:  con,
		logger:    c.service.logger,
	}, nil
}

// Publish provides convenience single-queue publishing using the configured default queue.
func (c *RMQClient) Publish(ctx context.Context, msg []byte) error {
	c.mu.Lock()
	if c.defaultPublisher == nil {
		queueName := c.service.Config.Name
		if queueName == "" {
			c.mu.Unlock()
			return errors.New("no default queue configured for client-level Publish")
		}
		pub, err := c.NewPublisher(ctx, models.PublisherTarget{Queue: queueName})
		if err != nil {
			c.mu.Unlock()
			return err
		}
		c.defaultPublisher = pub
	}
	pub := c.defaultPublisher
	c.mu.Unlock()

	return pub.Publish(ctx, msg)
}

// PublishEvent provides convenience publishing for outbox events.
func (c *RMQClient) PublishEvent(ctx context.Context, event models.Outbox) error {
	return c.Publish(ctx, event.Payload)
}

// Consume provides convenience single-queue consuming using the configured default queue.
func (c *RMQClient) Consume(ctx context.Context, handler func(ctx context.Context, body []byte) error) error {
	c.mu.Lock()
	if c.defaultConsumer == nil {
		queueName := c.service.Config.Name
		if queueName == "" {
			c.mu.Unlock()
			return errors.New("no default queue configured for client-level Consume")
		}
		con, err := c.NewConsumer(ctx, queueName)
		if err != nil {
			c.mu.Unlock()
			return err
		}
		c.defaultConsumer = con
	}
	con := c.defaultConsumer
	c.mu.Unlock()

	return con.Consume(ctx, handler)
}

// Close tears down all resources, connections, and environments.
func (c *RMQClient) Close(ctx context.Context) error {
	c.mu.Lock()
	if c.defaultPublisher != nil {
		_ = c.defaultPublisher.Close(ctx)
		c.defaultPublisher = nil
	}
	if c.defaultConsumer != nil {
		_ = c.defaultConsumer.Close(ctx)
		c.defaultConsumer = nil
	}
	c.mu.Unlock()

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
