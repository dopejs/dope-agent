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
- A roadmap remains the only acceptable implementation delivery unit.
- A task is an auditable work item inside a roadmap.
- Tasks exist to make progress and gaps inspectable. They do not authorize shipping a partial roadmap.
- A task can only be marked `[x]` when its task-level definition of done is fully satisfied.
- A roadmap can only be marked `[x]` when every required task inside it is fully satisfied and the roadmap-level definition of done is met.
- Partial, provisional, or narrow implementations stay `[ ]` with a note like `(partial)` or `(blocked)`.
- If a roadmap is too large to finish in one round, it must be re-cut into smaller roadmaps before implementation starts. It should not be "partially completed and counted as done".
- Demo-grade shortcuts do not count as roadmap completion.

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

## Roadmap 6: Real Conversation Core

Status: `[x] complete`

### Goal

Make DopeAgent able to serve real single-turn AI replies over daemon APIs once a provider is configured.

This roadmap closes the daemon-side slice only:

- one user query in
- one assistant reply out
- one real configured provider
- one stable chat contract
- no client delivery in this roadmap

### Tasks

#### 1. Real Provider Configuration Model

Scope:

- extend daemon config with provider settings
- define default provider and default model behavior
- define redaction policy for secrets in config inspection

Task definition of done:

- config supports at least one real provider registration path
- provider config can be loaded from file and env overrides
- secrets are never exposed through config inspection APIs
- invalid provider config fails clearly at startup
- tests cover load, override, redaction, and invalid config behavior

#### 2. OpenAI-Compatible Provider Integration

Scope:

- implement one real provider using an OpenAI-compatible chat/completions surface
- use OpenClaw as a reference for provider behavior and request mapping where helpful

Task definition of done:

- daemon can issue real non-echo model requests to a configured upstream provider
- auth, base URL, model, timeout, and error mapping are explicit
- non-stream and stream execution both work through the existing dispatch plane
- retry behavior remains compatible with current dispatcher rules
- tests cover request mapping, auth failure, upstream failure, and streamed success

#### 3. Minimal Conversation API Contract

Scope:

- add a single-turn conversation contract above raw LLM dispatch
- keep daemon stateless with respect to conversation history

Task definition of done:

- daemon exposes a minimal chat query route and a streaming variant
- request shape is query-first, not full runtime/run-step oriented
- response shape is schema-backed and stable enough for UI clients
- implementation is explicit that the daemon does not retain multi-turn conversation state
- tests cover success, provider error, auth error, and streaming completion

#### 4. End-To-End Verification And Operator Docs

Scope:

- add real-provider smoke coverage
- document operator setup and failure modes

Task definition of done:

- there is a documented setup path from empty config to first successful reply
- there is a documented failure path for bad key, bad endpoint, and missing model
- end-to-end verification proves one real configured provider can serve both HTTP and streaming chat paths
- rollback or disable path is documented if provider integration must be turned off

### Roadmap Definition Of Done

- daemon can talk to at least one real configured provider
- the chat contract is explicit, schema-backed, and independent from future memory/context design
- one operator can reach a successful single-turn reply through daemon HTTP APIs alone
- operator setup, error handling, and verification are documented well enough to support client development

### Explicitly Out Of Scope

- Web UI delivery
- TUI delivery
- multi-turn conversation state in daemon
- memory or context engineering
- tool use orchestration during chat
- provider marketplace or multi-provider routing policy

## Roadmap 7: Minimal Chat Clients

Status: `[x] complete`

### Goal

Make the configured daemon usable from both a minimal Web UI and a minimal TUI for single-turn chat.

This roadmap depends on Roadmap 6 being complete first.

### Tasks

#### 1. Shared Client Contract Adoption

Scope:

- use the daemon chat contract as the only conversation surface for both clients

Task definition of done:

- Web and TUI both use the same daemon chat routes
- neither client introduces hidden provider-specific request logic
- neither client depends on daemon-side multi-turn state
- smoke verification proves contract parity across both clients

#### 2. Web Chat Surface

Scope:

- add a minimal Web UI for single-turn chat

Task definition of done:

- operator can enter a query and receive a real assistant response
- loading, error, and retry states are visible
- provider configuration assumptions are documented in the UI or operator docs
- the Web UI does not depend on memory or context subsystems
- tests or smoke verification cover the basic chat path

#### 3. TUI Chat Surface

Scope:

- add a minimal TUI for single-turn chat

Task definition of done:

- operator can enter a query and receive a real assistant response from terminal
- loading, error, and retry states are visible
- TUI uses the same daemon chat contract as the Web UI
- the TUI does not carry hidden multi-turn state in daemon
- tests or smoke verification cover the basic TUI chat path

#### 4. Cross-Client Verification

Scope:

- verify operator setup and failure behavior across both client surfaces

Task definition of done:

- one configured daemon can serve both Web and TUI without client-specific backend switches
- auth failure, provider failure, and streaming failure are visible from both clients
- operator docs cover how to start each client against the daemon

### Roadmap Definition Of Done

- Web UI and TUI can both issue a single-turn query and render the reply
- both clients rely on the same daemon chat contract
- client behavior is documented and smoke-verified

### Explicitly Out Of Scope

- multi-turn conversation state
- memory or context engineering
- tool use orchestration during chat
- rich conversation UX beyond minimal operator flow

## Roadmap 8: Ingress Routing Closure

Status: `[x] complete`

### Goal

Close the remaining ingress and routing gap so external connector input can route into sessions and create session-bound runs without relying on local-only fallback semantics.

### Tasks

#### 1. Route-Aware Run Creation

Scope:

- allow `POST /v1/runs` to resolve a session from explicit route input

Task definition of done:

- `POST /v1/runs` accepts either `sessionId`, explicit `route`, or the existing local fallback
- ambiguous requests are rejected consistently
- route-driven runs bind to the resolved session and persist correctly
- API and contract tests cover explicit route-based run creation

#### 2. Connector Ingress Contract

Scope:

- add an explicit connector ingress API for inbound messages

Task definition of done:

- daemon exposes a connector ingress route for inbound messages
- ingress resolves or creates the correct session from connector/channel identity
- ingress can optionally create a run bound to that session
- connector status gating is explicit and tested
- request and response shapes are schema-backed

#### 3. Ingress Event And Recovery Closure

Scope:

- make ingress behavior durable and restart-safe

Task definition of done:

- session and run state created through ingress are persisted
- ingress emits explicit connector/session/run events
- restart recovery restores ingress-created session and run state
- tests cover persistence and restart behavior for ingress-created state

#### 4. Contract And Documentation Closure

Scope:

- close schema, event, and operator documentation for ingress routing

Task definition of done:

- request/response schemas exist for ingress routes
- event schemas exist for ingress-specific events
- contract tests validate ingress responses and events
- operator docs explain how connector ingress maps to session/run state

### Roadmap Definition Of Done

- inbound connector traffic has a first-class daemon API
- runs can be bound to sessions from explicit route data, not only local fallback
- ingress-created session and run state are durable and restart-safe
- routing behavior is documented, schema-backed, and fully tested

### Explicitly Out Of Scope

- real external connector transport implementations
- message queueing or async ingress buffering
- automatic reply generation from inbound connector traffic

## Recommended Order

1. Roadmap 1: Runtime Closure
2. Roadmap 2: Supervision Plane
3. Roadmap 3: LLM Dispatch Plane
4. Roadmap 4: Operator Trust And Security
5. Roadmap 5: Contract Hardening And Ship Readiness
6. Roadmap 6: Real Conversation Core
7. Roadmap 7: Minimal Chat Clients
8. Roadmap 8: Ingress Routing Closure
