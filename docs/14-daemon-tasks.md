# P0 Daemon Task Registry

## Purpose

This document is the audit registry for P0 daemon tasks.

The execution unit is defined in [15-daemon-roadmaps.md](/Users/John/Code/agent-os/docs/15-daemon-roadmaps.md). A roadmap contains multiple tasks and is only complete when those tasks are fully complete.

This file exists to:

- track task status
- make incomplete work visible
- prevent narrow implementations from being counted as done

## Status Rules

- `[x]` means the full task definition of done is satisfied
- `[ ]` means the task is not complete
- use a short suffix like `(partial)`, `(in_progress)`, or `(blocked)` when needed
- a task stays `[ ] (partial)` if any required API, persistence, recovery, eventing, contract, or testing boundary is still open

## Current Overall Read

The daemon has a closed runtime foundation, but it does not yet have a supervision plane, model plane, or operator trust boundary.

Current completion should be read as:

- foundation exists
- several subsystems are usable
- many tasks are still partial against their real boundary

## Roadmap 1: Runtime Closure

- [x] Runtime lifecycle commands
- [x] Terminal-state and idempotency rules
- [x] Daemon lifecycle hooks and system events
- [x] Event continuation semantics
- [x] Config inspection API
- [x] Runtime contract hardening
- [x] Restart integration coverage

## Roadmap 2: Supervision Plane

- [x] Connector supervisor contract
- [x] Capability supervisor contract
- [x] Restart and backoff policy
- [x] Connector and capability APIs
- [x] Tool-call dispatch boundary and durable execution history

## Roadmap 3: LLM Dispatch Plane

- [ ] Provider abstraction
- [ ] Usage accounting
- [ ] Retry and timeout policy
- [ ] Streaming integration

## Roadmap 4: Operator Trust And Security

- [ ] Local-first auth and pairing
- [ ] Approval enforcement integration
- [ ] Approval durability

## Roadmap 5: Contract Hardening And Ship Readiness

- [ ] Schema enforcement pipeline
- [ ] Migration and versioning plan
- [ ] Restart and recovery coverage expansion
- [ ] Release contract review

## Completed Foundational Work

These are implemented pieces that support later roadmaps. They are real progress, but they do not by themselves close the roadmap-level work above.

### Daemon Kernel

- [x] Create Go daemon entrypoint and app assembly
- [x] Define daemon module boundaries
- [x] Add health and version endpoints
- [x] Add `/v1/system/info` endpoint
- [x] Add structured config load path
- [x] Add config file loading from `~/.dope`
- [x] Add startup directory initialization for `~/.dope`
- [x] Add graceful shutdown lifecycle hooks
- [x] Add daemon startup and shutdown events

### Runtime Foundation

- [x] Implement create/list/get run
- [x] Implement create/list/get step
- [x] Implement step status transitions
- [x] Implement run aggregate status reconciliation
- [x] Define run domain model
- [x] Define step domain model
- [x] Add run cancel command
- [x] Add run resume command
- [x] Add step cancel command
- [x] Add runtime validation around terminal states
- [x] Add runtime idempotency rules for commands

### Event Foundation

- [x] Define common event envelope
- [x] Add in-memory event history
- [x] Add filtered event subscriptions
- [x] Add SSE replay and live stream
- [x] Emit `run.created`
- [x] Emit `step.created`
- [x] Emit `step.status_changed`
- [x] Emit `run.status_changed`
- [x] Persist event history to SQLite
- [x] Add event sequence or cursor semantics
- [x] Emit startup and shutdown system events

### API Foundation

- [x] Define P0 API categories
- [x] Implement system routes
- [x] Implement run routes
- [x] Implement step routes
- [x] Implement step status command route
- [x] Implement run events route
- [x] Add session routes
- [x] Add policy approval routes
- [x] Add config inspection route
- [x] Align all responses with schemas

### Tool Call Foundation

- [x] Define tool call domain model
- [x] Add create/list/get tool call records
- [x] Add `tool_call.requested` event
- [x] Add `tool_call.completed` event
- [x] Add `tool_call.failed` event
- [x] Attach tool calls to step execution history
- [x] Define capability dispatch boundary for tool calls

### Persistence And Recovery Foundation

- [x] Create SQLite persistence layer skeleton
- [x] Define persistent entities for runs, steps, and events
- [x] Persist runs
- [x] Persist steps
- [x] Persist events
- [x] Add checkpoint model
- [x] Add checkpoint persistence
- [x] Add restart recovery flow
- [ ] Add data migration and versioning plan

### Sessions And Routing Foundation

- [x] Define session resource model
- [x] Implement direct and group session isolation
- [x] Implement session list/get/reset routes
- [ ] Implement inbound routing model `(partial: router model exists, connector ingress is not wired)`
- [ ] Bind runs to sessions where needed `(partial: minimal default binding exists, but not full ingress-aware binding)`

### Policy Foundation

- [x] Define policy engine contract
- [x] Add approval model
- [x] Add policy decision events
- [ ] Add local-first auth and pairing model
- [ ] Approval durability
- [ ] Approval enforcement integration

### Contracts And Testing Foundation

- [x] Create base config schema
- [x] Create run and step resource schemas
- [x] Create event envelope schema
- [x] Create create-run schema
- [x] Create create-step schema
- [x] Create update-step-status schema
- [x] Create tool call schemas
- [x] Create session schemas
- [x] Create policy and approval schemas
- [x] Create connector and capability schemas
- [ ] Add schema enforcement and generation pipeline
- [x] Add runtime unit tests
- [x] Add API unit tests
- [x] Add event bus unit tests
- [x] Add persistence tests
- [x] Add checkpoint tests
- [x] Add config loading tests
- [x] Add integration tests across daemon restart

## Working Rule

When work completes:

- update the task in this registry
- update the roadmap status in [15-daemon-roadmaps.md](/Users/John/Code/agent-os/docs/15-daemon-roadmaps.md) if the task affects roadmap closure
- do not mark a task or roadmap complete until its full boundary is actually closed
