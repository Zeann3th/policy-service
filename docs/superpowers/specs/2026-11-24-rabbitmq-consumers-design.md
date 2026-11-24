# RabbitMQ Registered Consumers Design

## Intent

Let application services register one handler per configured RabbitMQ queue. The RabbitMQ server owns AMQP channels, consumer lifecycle, and ACK/NACK behavior; services only implement message handling.

The first and only registered consumer in this change is `usage-releases`. Its JSON payload is `{"requestId":"..."}` and it calls `reservation.Service.Release`.

## Goals

- Declare every queue from YAML.
- Consume only queues with registered handlers; unregistered queues remain declared and idle.
- Apply global consumer defaults with optional per-queue overrides.
- ACK successful handlers and NACK failed handlers according to the queue's requeue policy.
- Use a separate AMQP channel per active consumer and close all owned resources during shutdown.
- Expose declared queue metadata through a defensive copy.

## Non-goals

- Publishing API, retries with backoff, dead-letter topology, reconnection, or parallel workers.
- Routing one queue to multiple handlers.
- Dynamic registration after `Start`.
- Publishing the `usage-releases` event.

## Contract

```go
type Handler func(context.Context, amqp.Delivery) error

func (s *Server) RegisterHandler(queue string, handler Handler) error
func (s *Server) Queues() []amqp.Queue
```

Registration rejects unknown queues, nil handlers, duplicate handlers, and registration after startup. Each registered handler runs sequentially for its queue.

## YAML

```yaml
messageBroker:
  consumer:
    autoAck: false
    exclusive: false
    noLocal: false
    noWait: false
    requeueOnError: true
  queue:
    - name: usage-releases
      durability: false
      consumer:
        tag: usage-releases
```

Boolean fields in a queue override are optional so an explicit `false` can override a global `true`.

## Error and Lifecycle Semantics

- `New` owns and cleans up partially created connections/channels on startup failure.
- `Start` opens one channel per registered queue, starts consumption, then waits for connection close.
- With `autoAck: false`, handler success sends ACK; handler error sends NACK using `requeueOnError`.
- Invalid JSON or a blank `requestId` is rejected without requeue.
- A `reservation.Service.Release` error is NACKed using `requeueOnError`; successful or already-completed releases are ACKed.
- With `autoAck: true`, the broker acknowledges delivery immediately and the server does not ACK/NACK.
- `Shutdown` cancels handler contexts, closes consumer channels, then closes the connection.
- Connection-level failure returns from `Start`; automatic reconnection is outside this change.

## Approaches Considered

1. **Registered handlers owned by `Server` — selected.** Centralizes AMQP lifecycle and acknowledgement rules while keeping service handlers small.
2. Expose raw channels to services. Rejected because it spreads concurrency, recovery, and cleanup responsibilities.
3. Resolve handler names from YAML. Rejected because it couples runtime code registration to configuration without a current need.

## Tests

- Configuration default/override merging.
- Registration validation and defensive queue exposure.
- Consumer options forwarded correctly.
- ACK on success and NACK/requeue on failure.
- Unregistered queues are not consumed.
