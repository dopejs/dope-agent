# P0 Daemon Roadmaps

## Purpose

This document defines the execution structure for the P0 daemon.

The delivery unit is a **roadmap**, not an isolated task.

Each roadmap:

- forms a closed vertical slice
- contains multiple tasks
- has a roadmap-level definition of done
- is only considered complete when every in-scope task meets its own completion standard

Each implementation round should finish **one whole roadmap**.

## Execution Standard

- A roadmap is the planning and delivery boundary.
- A task is an auditable work item inside a roadmap.
- A task can only be marked `[x]` when its task-level definition of done is fully satisfied.
- A roadmap can only be marked `[x]` when every required task inside it is fully satisfied and the roadmap-level definition of done is met.
- Partial, provisional, or narrow implementations stay `[ ]` with a note like `(partial)` or `(blocked)`.
- If a roadmap is too large to finish in one round, it must be re-cut into smaller roadmaps. It should not be "partially completed and counted as done".

## Current State

The daemon now has a closed P0 control plane slice:

- system routes
- run and step resources
- event bus and SSE
- SQLite persistence
- checkpoint and restart recovery
- session routing
- supervision state
- LLM dispatch
- auth and approval gating
- contract tests
- versioned migrations

## Roadmap 1: Runtime Closure

Status: `[x] complete`

### Goal

Turn the daemon runtime from a set of resource handlers into a closed, restart-safe execution core.

### Tasks

#### 1. Runtime Lifecycle Commands

Scope:

- add run cancel command
- add run resume command
- add step cancel command

Task definition of done:

- explicit command routes exist for run cancel, run resume, and step cancel
- runtime applies legal transitions only
- invalid transitions return stable API errors
- emitted events cover the command result and resulting runtime state changes
- persistence and checkpoint state remain correct after each command
- unit and API tests cover happy path and rejection path

#### 2. Terminal-State And Idempotency Rules

Scope:

- define terminal-state validation
- define command idempotency rules

Task definition of done:

- terminal states are explicit for run and step
- illegal post-terminal mutations are rejected consistently
- duplicate command submissions have defined idempotent behavior
- behavior is documented in API contract or task notes
- tests cover duplicate submissions and terminal-state violations

#### 3. Daemon Lifecycle Hooks And System Events

Scope:

- add graceful subsystem shutdown hooks
- emit startup and shutdown system events

Task definition of done:

- daemon startup emits a durable startup event
- daemon shutdown emits a durable shutdown event when shutdown is orderly
- event bus, HTTP server, persistence layer, and checkpoint manager are shut down in defined order
- shutdown does not corrupt persisted runtime state
- tests cover orderly shutdown behavior at subsystem level

#### 4. Event Continuation Semantics

Scope:

- add event sequence or cursor semantics

Task definition of done:

- every persisted event has a stable ordering token
- SSE replay can resume from a cursor or sequence boundary
- event listing and streaming semantics are documented and tested
- duplicate or skipped replay behavior is explicit

#### 5. Config Inspection API

Scope:

- add config read API

Task definition of done:

- daemon exposes a read-only config inspection route
- response shape is schema-backed
- sensitive fields are redacted or omitted by policy
- tests cover config load, config read, and redaction behavior

#### 6. Runtime Contract Hardening

Scope:

- align runtime, session, and config responses with schemas

Task definition of done:

- current runtime and session resource responses match committed schemas
- drift between docs, schemas, and implementation is removed for in-scope routes
- tests fail on contract mismatch or fixture mismatch

#### 7. Restart Integration Coverage

Scope:

- add daemon restart integration test for runtime boundary

Task definition of done:

- an integration test proves runtime state, events, and checkpoints survive daemon restart
- the test covers at least one non-trivial run with steps and state transitions
- restart test fails if replay, ordering, or checkpoint restore breaks

### Roadmap Definition Of Done

- runtime lifecycle commands are explicit and restart-safe
- terminal-state and idempotency behavior are defined and enforced
- daemon lifecycle events and graceful shutdown are implemented
- event continuation semantics are stable enough for operator clients
- config can be inspected safely through API
- runtime-facing contracts are aligned with schemas
- restart integration coverage proves the runtime slice is closed

### Explicitly Out Of Scope

- connector supervision
- capability supervision
- LLM provider integration
- auth and pairing

## Roadmap 2: Supervision Plane

Status: `[x] complete`

### Goal

Turn connectors and capabilities into daemon-managed supervised units with explicit health and restart semantics.

### Tasks

#### 1. Connector Supervisor Contract

Scope:

- define connector supervisor contract
- define connector health model

Task definition of done:

- connector lifecycle states are explicit
- daemon can register, inspect, and report connector state
- health model and failure state are exposed through API and schema
- tests cover registration, health updates, and failure observation

#### 2. Capability Supervisor Contract

Scope:

- define capability supervisor contract
- define capability health model

Task definition of done:

- capability lifecycle states are explicit
- daemon can register, inspect, and report capability state
- health model and failure state are exposed through API and schema
- tests cover registration, health updates, and failure observation

#### 3. Restart And Backoff Policy

Scope:

- add restart and backoff policy

Task definition of done:

- restart policy is explicit for connectors and capabilities
- backoff state is observable
- repeated failures do not create uncontrolled restart loops
- tests cover restart, backoff escalation, and terminal failure

#### 4. Connector And Capability APIs

Scope:

- add connector routes
- add capability routes
- create connector and capability schemas

Task definition of done:

- daemon exposes list/get and health routes for connectors and capabilities
- responses align with schemas
- tests cover API behavior and schema-backed fixtures

#### 5. Tool-Call Dispatch Boundary

Scope:

- define capability dispatch boundary for tool calls
- attach tool calls to durable execution history

Task definition of done:

- tool calls have a durable store, not only runtime attachment
- tool call records identify dispatched capability boundary
- requested/completed/failed trail is queryable through API
- tests cover dispatch recording and failure recording

### Roadmap Definition Of Done

- daemon can supervise connectors and capabilities as first-class managed units
- health, restart, and backoff behavior are explicit and observable
- tool calls cross a defined capability boundary with durable execution history
- connector and capability contracts are schema-backed and tested

### Explicitly Out Of Scope

- real external IM integrations beyond supervised contract surface
- full browser or media capability implementations
- auth and pairing

## Roadmap 3: LLM Dispatch Plane

Status: `[x] complete`

### Goal

Make model invocation a first-class daemon subsystem instead of a placeholder.

### Tasks

#### 1. Provider Abstraction

Scope:

- define provider abstraction
- define request and response model

Task definition of done:

- daemon has a concrete provider interface with request/response types
- provider errors and cancellation behavior are explicit
- schema coverage exists for dispatch resources or events as needed
- tests cover provider success and failure paths

#### 2. Usage Accounting

Scope:

- add usage accounting model

Task definition of done:

- requests and responses can record token or usage metadata
- usage data is persisted or durably attached to dispatch history
- tests cover accounting write path

#### 3. Retry And Timeout Policy

Scope:

- add retries and timeout policy

Task definition of done:

- timeout behavior is explicit
- retry policy distinguishes retryable and non-retryable failures
- tests cover timeout, retry, and final failure paths

#### 4. Streaming Integration

Scope:

- add streaming response integration

Task definition of done:

- daemon can stream model output through a stable contract
- streaming cancellation and completion semantics are explicit
- tests cover streamed success and interrupted stream cases

### Roadmap Definition Of Done

- daemon can issue model requests through a real provider abstraction
- usage, retries, timeout, and streaming semantics are explicit
- dispatch contracts are durable enough to support real runtime execution

### Explicitly Out Of Scope

- provider-specific optimization
- long-context memory integration
- advanced tool routing policy

## Roadmap 4: Operator Trust And Security

Status: `[x] complete`

### Goal

Make the daemon operable as a local-first personal agent service with a concrete trust boundary.

### Tasks

#### 1. Local-First Auth And Pairing

Scope:

- add local-first auth model
- add pairing model

Task definition of done:

- daemon exposes a concrete local auth mechanism
- pairing flow is explicit for operator clients
- auth failure behavior is tested
- operator-facing docs explain trust assumptions

#### 2. Approval Enforcement Integration

Scope:

- connect policy approvals to real guarded actions

Task definition of done:

- high-risk actions can require approval before execution
- approval state gates execution rather than existing only as a resource
- tests cover allowed, pending, approved, and rejected flows

#### 3. Approval Durability

Scope:

- persist approvals and decisions if required by final contract

Task definition of done:

- approval and decision state survive daemon restart
- recovery behavior is explicit and tested
- API behavior after restart is stable

### Roadmap Definition Of Done

- daemon has a real local trust boundary
- operator clients can authenticate or pair against that boundary
- approval flows can gate real actions and survive restart if required

### Explicitly Out Of Scope

- org-scale RBAC
- remote multi-tenant control plane

## Roadmap 5: Contract Hardening And Ship Readiness

Status: `[x] complete`

### Goal

Close the remaining contract, migration, and release-readiness gaps that block P0.

### Tasks

#### 1. Schema Enforcement Pipeline

Scope:

- add schema enforcement and generation pipeline

Task definition of done:

- implementation path validates or reliably checks schema conformance
- contract drift is detectable in CI or test workflow
- schema update process is documented

#### 2. Migration And Versioning Plan

Scope:

- add data migration and versioning plan

Task definition of done:

- persisted state has a versioning strategy
- migration steps and rollback expectations are documented
- tests cover at least one migration or version check path

#### 3. Restart And Recovery Coverage Expansion

Scope:

- expand restart and recovery integration coverage beyond runtime

Task definition of done:

- restart coverage includes the major persisted subsystems in scope
- failure modes around replay and partial recovery are tested

#### 4. Release Contract Review

Scope:

- clean remaining API, schema, and documentation drift

Task definition of done:

- critical API/doc/schema mismatches are closed
- daemon can be reviewed as a coherent P0 foundation

### Roadmap Definition Of Done

- persisted contracts are enforceable and evolvable
- restart and migration behavior are defined
- API, schema, and docs are aligned enough for a P0 release decision

### Explicitly Out Of Scope

- advanced memory system
- rich workflow automation
- broad ecosystem integrations

## Recommended Order

1. Roadmap 1: Runtime Closure
2. Roadmap 2: Supervision Plane
3. Roadmap 3: LLM Dispatch Plane
4. Roadmap 4: Operator Trust And Security
5. Roadmap 5: Contract Hardening And Ship Readiness
