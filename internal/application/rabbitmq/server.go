package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/zeann3th/policy-service/internal/infrastructure/config"
)

var ErrReject = errors.New("reject message")

type Handler func(context.Context, amqp.Delivery) error

type consumerChannel interface {
	Consume(string, string, bool, bool, bool, bool, amqp.Table) (<-chan amqp.Delivery, error)
	Close() error
}

type Server struct {
	broker           config.MessageBroker
	closed           chan *amqp.Error
	queues           []amqp.Queue
	handlers         map[string]Handler
	openChannel      func() (consumerChannel, error)
	closeConnection  func() error
	consumerChannels []consumerChannel
	cancel           context.CancelFunc
	started          bool
	logger           *slog.Logger
	mu               sync.Mutex
	wg               sync.WaitGroup
}

func New(cfg *config.Config, logger *slog.Logger) (*Server, error) {
	conn, err := amqp.Dial(cfg.MessageBroker.Addrs[0])
	if err != nil {
		return nil, err
	}
	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	queues, declareErr := declareQueues(channel, cfg.MessageBroker.Queue)
	closeErr := channel.Close()
	if declareErr != nil {
		_ = conn.Close()
		return nil, declareErr
	}
	if closeErr != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("close queue declaration channel: %w", closeErr)
	}

	return &Server{
		broker:          cfg.MessageBroker,
		closed:          conn.NotifyClose(make(chan *amqp.Error, 1)),
		queues:          queues,
		handlers:        make(map[string]Handler),
		openChannel:     func() (consumerChannel, error) { return conn.Channel() },
		closeConnection: conn.Close,
		logger:          logger,
	}, nil
}

func (s *Server) RegisterHandler(queue string, handler Handler) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return fmt.Errorf("register handler for %s: server already started", queue)
	}
	if handler == nil {
		return fmt.Errorf("register handler for %s: handler is required", queue)
	}
	if !slices.ContainsFunc(s.broker.Queue, func(config config.MessageQueue) bool { return config.Name == queue }) {
		return fmt.Errorf("register handler: unknown queue %s", queue)
	}
	if _, exists := s.handlers[queue]; exists {
		return fmt.Errorf("register handler for %s: handler already registered", queue)
	}
	s.handlers[queue] = handler
	return nil
}

func (s *Server) Queues() []amqp.Queue {
	return slices.Clone(s.queues)
}

type queueDeclarer interface {
	QueueDeclare(string, bool, bool, bool, bool, amqp.Table) (amqp.Queue, error)
}

func declareQueues(channel queueDeclarer, configs []config.MessageQueue) ([]amqp.Queue, error) {
	queues := make([]amqp.Queue, 0, len(configs))
	for _, cfg := range configs {
		queue, err := channel.QueueDeclare(cfg.Name, cfg.Durability, cfg.DeleteWhenUnused, cfg.Exclusive, cfg.NoWait, nil)
		if err != nil {
			return nil, fmt.Errorf("declare queue %s: %w", cfg.Name, err)
		}
		queues = append(queues, queue)
	}
	return queues, nil
}

func (s *Server) Start() error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return fmt.Errorf("message broker server already started")
	}
	s.started = true
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	handlers := make(map[string]Handler, len(s.handlers))
	for queue, handler := range s.handlers {
		handlers[queue] = handler
	}
	s.mu.Unlock()
	consumerErrors := make(chan error, len(handlers))

	for _, queue := range s.broker.Queue {
		handler, registered := handlers[queue.Name]
		if !registered {
			continue
		}
		channel, err := s.openChannel()
		if err != nil {
			cancel()
			s.closeConsumerChannels()
			return fmt.Errorf("open consumer channel for %s: %w", queue.Name, err)
		}
		consumer := s.broker.ConsumerFor(queue)
		deliveries, err := channel.Consume(queue.Name, consumer.Tag, consumer.AutoAck, consumer.Exclusive, consumer.NoLocal, consumer.NoWait, nil)
		if err != nil {
			_ = channel.Close()
			cancel()
			s.closeConsumerChannels()
			return fmt.Errorf("consume queue %s: %w", queue.Name, err)
		}
		s.mu.Lock()
		s.consumerChannels = append(s.consumerChannels, channel)
		s.mu.Unlock()
		s.wg.Add(1)
		s.wg.Go(func() {
			if err := s.consume(ctx, queue.Name, deliveries, handler, consumer); err != nil {
				consumerErrors <- err
			}
		})
	}

	select {
	case err := <-s.closed:
		cancel()
		if err != nil {
			return err
		}
		return nil
	case err := <-consumerErrors:
		cancel()
		_ = s.closeConsumerChannels()
		return err
	}
}

func (s *Server) consume(ctx context.Context, queue string, deliveries <-chan amqp.Delivery, handler Handler, consumer config.MessageConsumer) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				if ctx.Err() != nil {
					return nil
				}
				return fmt.Errorf("consumer for queue %s stopped", queue)
			}
			if err := handleDelivery(ctx, handler, delivery, consumer); err != nil {
				s.logger.Error("[RabbitMQ] Message handler failed", "queue", queue, "error", err)
			}
		}
	}
}

func handleDelivery(ctx context.Context, handler Handler, delivery amqp.Delivery, consumer config.MessageConsumer) error {
	handlerErr := handler(ctx, delivery)
	if consumer.AutoAck {
		return handlerErr
	}
	if handlerErr == nil {
		return delivery.Ack(false)
	}
	requeue := consumer.RequeueOnError && !errors.Is(handlerErr, ErrReject)
	return errors.Join(handlerErr, delivery.Nack(false, requeue))
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("[RabbitMQ] Shutting down message broker connection")
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	waited := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(waited)
	}()
	var waitErr error
	select {
	case <-waited:
	case <-ctx.Done():
		waitErr = ctx.Err()
	}
	return errors.Join(waitErr, s.closeConsumerChannels(), s.closeConnection())
}

func (s *Server) closeConsumerChannels() error {
	s.mu.Lock()
	channels := s.consumerChannels
	s.consumerChannels = nil
	s.mu.Unlock()

	var err error
	for _, channel := range channels {
		err = errors.Join(err, channel.Close())
	}
	return err
}
