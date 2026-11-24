# RabbitMQ Registered Consumers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Consume `usage-releases` messages through a registered handler that calls `reservation.Service.Release`, with YAML-controlled consumer defaults and per-queue overrides.

**Architecture:** `rabbitmq.Server` owns the connection, declaration channel, one channel per registered consumer, delivery acknowledgement, and lifecycle. Services register handlers by configured queue name before `Start`; only registered queues are consumed. Configuration merges concrete global defaults with pointer-based per-queue overrides so explicit `false` values remain distinguishable from omitted fields.

**Tech Stack:** Go, `github.com/rabbitmq/amqp091-go`, `github.com/goccy/go-yaml`, standard `encoding/json`.

**Spec:** `docs/superpowers/specs/2026-11-24-rabbitmq-consumers-design.md`

## Global Constraints

- Do not commit, push, tag, release, or publish.
- Keep `usage-releases` as the only registered consumer in this change.
- Payload is exactly a JSON object containing `requestId`; unknown JSON fields are rejected.
- Invalid JSON and blank `requestId` are rejected without requeue.
- Successful releases are ACKed; service errors are NACKed using effective `requeueOnError`.
- Do not expose raw AMQP channels or add publishing/reconnection/dead-letter behavior.
- Preserve caller-owned changes already present in the dirty worktree.

## Review Focus

- Explicit queue-level `false` must override a global `true`; Task 1 tests pointer-based merging.
- An unknown, nil, duplicate, or late handler registration must fail; Task 2 tests every case.
- An unregistered configured queue must not call `Consume`; Task 2 tests the consumer setup seam.
- Invalid payloads must not loop forever; Task 3 tests rejection without requeue.
- Shutdown must not use a channel closed during `New`; Task 2 tests channel ownership and lifecycle.

---

### Task 1: Consumer YAML Configuration

**Files:**
- Modify: `internal/infrastructure/config/config.go`
- Modify: `internal/infrastructure/config/config_test.go`
- Modify: `application.example.yaml`
- Modify: `e2e/application.yaml`
- Modify local ignored file when present: `application.yaml`

**Interfaces:**
- Produces: `MessageConsumer`, `MessageConsumerOverride`, and `MessageQueue.Consumer`.
- Produces: `func (c MessageBroker) ConsumerFor(queue MessageQueue) MessageConsumer`.

- [ ] **Step 1: Write failing configuration tests**

Add tests proving global consumer fields load, omitted queue fields inherit them, explicit queue `false` overrides global `true`, and checked-in YAML files configure only `usage-releases`.

- [ ] **Step 2: Run the focused test and verify RED**

Run: `go test ./internal/infrastructure/config`

Expected: compile failure for missing consumer types/method or assertions failing because consumer YAML is not loaded.

- [ ] **Step 3: Implement configuration types and merging**

Add concrete global fields `tag`, `autoAck`, `exclusive`, `noLocal`, `noWait`, and `requeueOnError`. Add an optional per-queue override whose scalar fields are pointers. `ConsumerFor` copies global values and applies only non-nil override fields; a queue tag overrides the global tag when non-empty.

- [ ] **Step 4: Update YAML files**

Configure global safe defaults (`autoAck: false`, `requeueOnError: true`, other booleans false) and the single `usage-releases` queue. Preserve the user's queue durability value (`false`).

- [ ] **Step 5: Run focused tests and verify GREEN**

Run: `go test ./internal/infrastructure/config`

Expected: PASS.

### Task 2: Registered Consumer Runtime

**Files:**
- Modify: `internal/application/rabbitmq/server.go`
- Modify: `internal/application/rabbitmq/server_test.go`

**Interfaces:**
- Consumes: `config.MessageBroker.ConsumerFor(config.MessageQueue) config.MessageConsumer` from Task 1.
- Produces: `type Handler func(context.Context, amqp.Delivery) error`.
- Produces: `func (s *Server) RegisterHandler(queue string, handler Handler) error`.
- Produces: `func (s *Server) Queues() []amqp.Queue`.
- Changes: `func New(*config.Config, *slog.Logger) (*Server, error)` owns its declared queues instead of returning them separately.

- [ ] **Step 1: Write failing registration and exposure tests**

Test successful registration, unknown queue, nil handler, duplicate handler, registration after start, and mutation of the returned `Queues` slice not changing server state.

- [ ] **Step 2: Write failing consumer behavior tests**

Use the existing AMQP seams/fakes to assert only registered queues call `Consume`, effective options are forwarded, handler success ACKs, ordinary handler errors NACK with configured requeue, and `ErrReject` NACKs without requeue.

- [ ] **Step 3: Run the focused test and verify RED**

Run: `go test ./internal/application/rabbitmq`

Expected: compile failure for missing interfaces/methods or assertions failing against the current no-op consumer.

- [ ] **Step 4: Implement ownership and registration**

Store the connection and declared queue metadata on `Server`; close the declaration channel after setup. Maintain a queue-name lookup and handler registry guarded against late registration. `Queues` returns `slices.Clone(s.queues)`.

- [ ] **Step 5: Implement per-consumer channels and delivery processing**

Open one channel for every registered handler during `Start`, call `Consume` with effective YAML options, and process each delivery sequentially. With manual ACK, ACK nil handler errors, NACK ordinary errors using `requeueOnError`, and NACK `ErrReject` with `requeue=false`. With `autoAck`, never ACK/NACK in application code.

- [ ] **Step 6: Implement shutdown ownership**

Cancel handler contexts, close consumer channels, then close the AMQP connection. Keep startup-failure cleanup inside `New` because no outside owner receives the failed server.

- [ ] **Step 7: Run focused tests and verify GREEN**

Run: `go test ./internal/application/rabbitmq`

Expected: PASS.

### Task 3: Usage Release Handler and Wiring

**Files:**
- Create: `internal/application/rabbitmq/usage_release.go`
- Create: `internal/application/rabbitmq/usage_release_test.go`
- Modify: `internal/application/app.go`

**Interfaces:**
- Consumes: `rabbitmq.Handler` and `Server.RegisterHandler` from Task 2.
- Consumes: `reservation.Service.Release(context.Context, string) (reservation.ReleaseResponse, error)`.
- Produces: `func UsageReleaseHandler(reservation.Service) Handler`.

- [ ] **Step 1: Write failing handler tests**

Test strict JSON `{"requestId":"req-1"}` calls `Release` once and returns nil; malformed JSON, unknown fields, and blank IDs return `ErrReject`; service errors are returned unchanged.

- [ ] **Step 2: Run the focused test and verify RED**

Run: `go test ./internal/application/rabbitmq`

Expected: compile failure for missing `UsageReleaseHandler`.

- [ ] **Step 3: Implement the handler**

Decode one JSON object with `DisallowUnknownFields`, reject trailing JSON, trim and require `requestId`, then call `reservation.Service.Release`.

- [ ] **Step 4: Register the handler during application assembly**

After `rabbitmq.New`, call `RegisterHandler("usage-releases", UsageReleaseHandler(reservationService))`; wrap registration errors using the existing `fmt.Errorf` convention.

- [ ] **Step 5: Run focused tests and verify GREEN**

Run: `go test ./internal/application/rabbitmq ./internal/application`

Expected: PASS.

### Task 4: Verification and Review

**Files:**
- Review all files above plus `go.mod`, `go.sum`, and both Compose files.

**Interfaces:**
- Consumes: completed Tasks 1–3.
- Produces: verified implementation and review report.

- [ ] **Step 1: Run repository verification**

Run: `go test ./...`, `go build ./...`, and `go vet ./...`.

Expected: all exit 0.

- [ ] **Step 2: Validate deployment configuration**

Run: `docker compose -f cicd/docker-compose.yml config --quiet` and `docker compose -f e2e/docker-compose.yml config --quiet`.

Expected: both exit 0.

- [ ] **Step 3: Run diff and static-analysis checks**

Run: `git diff --check`; run Sonar if configured, otherwise record that it is unavailable.

- [ ] **Step 4: Review standards and specification compliance**

Check channel ownership, transaction boundaries, ACK/NACK behavior, config merging, registration timing, test mutation sensitivity, and exact adherence to the approved design.
