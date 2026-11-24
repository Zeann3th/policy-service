package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/zeann3th/policy-service/internal/infrastructure/config"
)

func TestDeclareQueuesUsesConfig(t *testing.T) {
	declarer := &recordingDeclarer{}
	queues, err := declareQueues(declarer, []config.MessageQueue{{
		Name:             "reservations",
		Durability:       true,
		DeleteWhenUnused: true,
		Exclusive:        true,
		NoWait:           true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(queues) != 1 || len(declarer.calls) != 1 {
		t.Fatalf("queues = %#v, calls = %#v", queues, declarer.calls)
	}
	if got := declarer.calls[0]; got.name != "reservations" || !got.durable || !got.autoDelete || !got.exclusive || !got.noWait {
		t.Fatalf("QueueDeclare args = %#v", got)
	}
}

func TestServerStartWaitsForConnectionClose(t *testing.T) {
	closed := make(chan *amqp.Error, 1)
	server := &Server{closed: closed}
	done := make(chan error, 1)
	go func() {
		done <- server.Start()
	}()

	want := &amqp.Error{Code: 320, Reason: "connection closed"}
	closed <- want
	if got := <-done; got != want {
		t.Fatalf("Start() error = %v, want %v", got, want)
	}
}

func TestRegisterHandler(t *testing.T) {
	handler := func(context.Context, amqp.Delivery) error { return nil }

	t.Run("registers configured queue", func(t *testing.T) {
		server := testServer("usage-releases")
		if err := server.RegisterHandler("usage-releases", handler); err != nil {
			t.Fatal(err)
		}
	})

	for name, register := range map[string]func(*Server) error{
		"unknown queue": func(server *Server) error {
			return server.RegisterHandler("unknown", handler)
		},
		"nil handler": func(server *Server) error {
			return server.RegisterHandler("usage-releases", nil)
		},
		"duplicate handler": func(server *Server) error {
			if err := server.RegisterHandler("usage-releases", handler); err != nil {
				return err
			}
			return server.RegisterHandler("usage-releases", handler)
		},
		"after start": func(server *Server) error {
			server.started = true
			return server.RegisterHandler("usage-releases", handler)
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := register(testServer("usage-releases")); err == nil {
				t.Fatal("RegisterHandler() error = nil")
			}
		})
	}
}

func TestQueuesReturnsCopy(t *testing.T) {
	server := testServer("usage-releases")
	queues := server.Queues()
	queues[0].Name = "changed"
	if got := server.Queues()[0].Name; got != "usage-releases" {
		t.Fatalf("Queues()[0].Name = %q", got)
	}
}

func TestStartConsumesOnlyRegisteredQueuesWithEffectiveConfig(t *testing.T) {
	consumer := config.MessageConsumer{Tag: "default", AutoAck: true, Exclusive: true, NoLocal: true, NoWait: true, RequeueOnError: true}
	autoAck := false
	channel := &recordingConsumerChannel{consumed: make(chan struct{}), deliveries: make(chan amqp.Delivery)}
	server := testServer("usage-releases", "unused")
	server.broker.Consumer = consumer
	server.broker.Queue[0].Consumer = &config.MessageConsumerOverride{Tag: "release-consumer", AutoAck: &autoAck}
	server.openChannel = func() (consumerChannel, error) { return channel, nil }

	if err := server.RegisterHandler("usage-releases", func(context.Context, amqp.Delivery) error { return nil }); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- server.Start() }()
	<-channel.consumed

	if len(channel.calls) != 1 {
		t.Fatalf("Consume calls = %#v", channel.calls)
	}
	call := channel.calls[0]
	if call.queue != "usage-releases" || call.tag != "release-consumer" || call.autoAck || !call.exclusive || !call.noLocal || !call.noWait {
		t.Fatalf("Consume args = %#v", call)
	}
	close(server.closed)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestHandleDeliveryAcknowledgesByOutcome(t *testing.T) {
	handlerErr := errors.New("handler failed")
	for name, test := range map[string]struct {
		handler     Handler
		consumer    config.MessageConsumer
		wantAck     bool
		wantNack    bool
		wantRequeue bool
		wantErr     error
	}{
		"success": {
			handler:  func(context.Context, amqp.Delivery) error { return nil },
			consumer: config.MessageConsumer{RequeueOnError: true},
			wantAck:  true,
		},
		"handler error": {
			handler:     func(context.Context, amqp.Delivery) error { return handlerErr },
			consumer:    config.MessageConsumer{RequeueOnError: true},
			wantNack:    true,
			wantRequeue: true,
			wantErr:     handlerErr,
		},
		"rejected error": {
			handler:  func(context.Context, amqp.Delivery) error { return fmt.Errorf("bad payload: %w", ErrReject) },
			consumer: config.MessageConsumer{RequeueOnError: true},
			wantNack: true,
			wantErr:  ErrReject,
		},
		"auto ack": {
			handler:  func(context.Context, amqp.Delivery) error { return handlerErr },
			consumer: config.MessageConsumer{AutoAck: true, RequeueOnError: true},
			wantErr:  handlerErr,
		},
	} {
		t.Run(name, func(t *testing.T) {
			acknowledger := &recordingAcknowledger{}
			delivery := amqp.Delivery{Acknowledger: acknowledger, DeliveryTag: 1}
			err := handleDelivery(context.Background(), test.handler, delivery, test.consumer)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("handleDelivery() error = %v", err)
			}
			if acknowledger.acked != test.wantAck || acknowledger.nacked != test.wantNack || acknowledger.requeue != test.wantRequeue {
				t.Fatalf("acknowledger = %#v", acknowledger)
			}
		})
	}
}

func TestShutdownClosesConsumerChannelsAndConnection(t *testing.T) {
	channel := &recordingConsumerChannel{}
	connectionClosed := false
	server := testServer("usage-releases")
	server.consumerChannels = []consumerChannel{channel}
	server.closeConnection = func() error {
		connectionClosed = true
		return nil
	}

	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !channel.closed || !connectionClosed {
		t.Fatalf("channel closed = %v, connection closed = %v", channel.closed, connectionClosed)
	}
}

func TestStartReturnsWhenConsumerStops(t *testing.T) {
	channel := &recordingConsumerChannel{consumed: make(chan struct{}), deliveries: make(chan amqp.Delivery)}
	server := testServer("usage-releases")
	server.openChannel = func() (consumerChannel, error) { return channel, nil }
	if err := server.RegisterHandler("usage-releases", func(context.Context, amqp.Delivery) error { return nil }); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- server.Start() }()
	<-channel.consumed
	close(channel.deliveries)

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Start() error = nil")
		}
	case <-time.After(time.Second):
		t.Fatal("Start() did not return after consumer stopped")
	}
}

func TestShutdownHonorsContextWhileHandlerIsRunning(t *testing.T) {
	channel := &recordingConsumerChannel{consumed: make(chan struct{}), deliveries: make(chan amqp.Delivery)}
	server := testServer("usage-releases")
	server.openChannel = func() (consumerChannel, error) { return channel, nil }
	server.closeConnection = func() error {
		close(server.closed)
		return nil
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	if err := server.RegisterHandler("usage-releases", func(context.Context, amqp.Delivery) error {
		close(entered)
		<-release
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	startDone := make(chan error, 1)
	go func() { startDone <- server.Start() }()
	<-channel.consumed
	channel.deliveries <- amqp.Delivery{Acknowledger: &recordingAcknowledger{}, DeliveryTag: 1}
	<-entered

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := server.Shutdown(ctx)
	close(release)
	<-startDone
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown() error = %v, want context deadline exceeded", err)
	}
}

func testServer(queues ...string) *Server {
	server := &Server{
		broker:          config.MessageBroker{},
		closed:          make(chan *amqp.Error),
		handlers:        make(map[string]Handler),
		closeConnection: func() error { return nil },
		logger:          slog.Default(),
	}
	for _, name := range queues {
		server.broker.Queue = append(server.broker.Queue, config.MessageQueue{Name: name})
		server.queues = append(server.queues, amqp.Queue{Name: name})
	}
	return server
}

type queueDeclareCall struct {
	name                                   string
	durable, autoDelete, exclusive, noWait bool
}

type recordingDeclarer struct {
	calls []queueDeclareCall
}

func (d *recordingDeclarer) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, _ amqp.Table) (amqp.Queue, error) {
	d.calls = append(d.calls, queueDeclareCall{name, durable, autoDelete, exclusive, noWait})
	return amqp.Queue{Name: name}, nil
}

type consumeCall struct {
	queue, tag                          string
	autoAck, exclusive, noLocal, noWait bool
}

type recordingConsumerChannel struct {
	calls      []consumeCall
	consumed   chan struct{}
	deliveries chan amqp.Delivery
	closed     bool
}

func (c *recordingConsumerChannel) Consume(queue, tag string, autoAck, exclusive, noLocal, noWait bool, _ amqp.Table) (<-chan amqp.Delivery, error) {
	c.calls = append(c.calls, consumeCall{queue, tag, autoAck, exclusive, noLocal, noWait})
	close(c.consumed)
	return c.deliveries, nil
}

func (c *recordingConsumerChannel) Close() error {
	c.closed = true
	return nil
}

type recordingAcknowledger struct {
	acked, nacked bool
	requeue       bool
}

func (a *recordingAcknowledger) Ack(uint64, bool) error {
	a.acked = true
	return nil
}

func (a *recordingAcknowledger) Nack(_ uint64, _ bool, requeue bool) error {
	a.nacked = true
	a.requeue = requeue
	return nil
}

func (a *recordingAcknowledger) Reject(_ uint64, requeue bool) error {
	a.nacked = true
	a.requeue = requeue
	return nil
}
