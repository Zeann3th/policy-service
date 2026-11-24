# Implementation Plan: YAML Application Configuration

## Overview

Replace application environment-variable loading with a structured `application.yaml`, including message-broker queues, and make local and E2E containers consume matching YAML files.

## Architecture Decisions

- Decode YAML directly into the existing config structs with explicit YAML field names and strict unknown-field rejection.
- Keep `Load()` as the application entry point and use a private path-based loader for isolated tests.
- Preserve current defaults in code; validate fields that were previously required.
- Mount an E2E-specific YAML file rather than adding an environment override system.

## Task List

### Phase 1: Config loader
- [x] Add failing YAML loader tests for nested config, queues, defaults, malformed YAML, unknown fields, and missing required fields.
- [x] Replace dotenv helpers with YAML decode and validation.

### Checkpoint: Config loader
- [x] `go test ./internal/infrastructure/config` passes.

### Phase 2: Runtime files
- [x] Add local and E2E YAML configuration files.
- [x] Wire RabbitMQ and the E2E config mount into Compose.
- [x] Remove the remaining test dependency on `.env` and tidy modules.

### Checkpoint: Complete
- [x] Full tests, build, vet, and Compose validation pass.
- [x] Final standards and requirements review has no unresolved findings.

## Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Working-directory-dependent config path | Startup failure | Containers mount the file at `/app/application.yaml`; local runs use the repository root. |
| Silent YAML typos | Incorrect runtime settings | Reject unknown YAML fields. |
| Missing broker dependency in E2E | Application startup failure | Add RabbitMQ with a health check and dependency ordering. |

## Open Questions

- None; `application.yaml` is the requested deployment contract.
