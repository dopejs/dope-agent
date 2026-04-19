# P0 Daemon Task Registry

## Purpose

This document is the audit registry for P0 daemon tasks.

The execution unit is defined in [15-daemon-roadmaps.md](/Users/John/Code/agent-os/docs/runtime/daemon-roadmaps.md). A roadmap contains multiple tasks and is only complete when those tasks are fully complete.

This file exists to:

- track task status
- make incomplete work visible
- prevent narrow implementations from being counted as done
- support mid-roadmap audit without weakening the roadmap completion standard

## Status Rules

- `[x]` means the full task definition of done is satisfied
- `[ ]` means the task is not complete
- use a short suffix like `(partial)`, `(in_progress)`, or `(blocked)` when needed
- a task stays `[ ] (partial)` if any required API, persistence, recovery, eventing, contract, or testing boundary is still open
- task progress is informational only; a roadmap is still incomplete until every in-scope task is fully closed

## Current Overall Read

The daemon now has a closed P0 daemon foundation across runtime, supervision, model dispatch, trust boundary, and ship-readiness contracts.

Current completion should be read as:

- the original five daemon roadmaps are closed
- `Roadmap 6: Real Conversation Core` is now closed
- `Roadmap 7: Minimal Chat Clients` is now closed
- `Roadmap 8: Ingress Routing Closure` is now closed
- `Roadmap 9: Provider Identity And Profiles` is now closed
- `Roadmap 10: Managed Coding Providers` is now closed
- `Roadmap 11: First IM Channel Loop` is now closed
- `Roadmap 12: Channel Reply Progression` is now closed
- `Roadmap 13: Provider Streaming Timeout Semantics` is now closed
- `Roadmap 14: Test Environment Workflow` is now closed
- `Roadmap 15: Skill Registry And Prompt Support` is now complete
- `Roadmap 16: Sandbox Execution Plane` is now planned

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

- [x] Provider abstraction
- [x] Usage accounting
- [x] Retry and timeout policy
- [x] Streaming integration

## Roadmap 4: Operator Trust And Security

- [x] Local-first auth and pairing
- [x] Approval enforcement integration
- [x] Approval durability

## Roadmap 5: Contract Hardening And Ship Readiness

- [x] Schema enforcement pipeline
- [x] Migration and versioning plan
- [x] Restart and recovery coverage expansion
- [x] Release contract review

## Roadmap 6: Real Conversation Core

- [x] Real provider configuration model
- [x] OpenAI-compatible provider integration
- [x] Minimal conversation API contract
- [x] End-to-end verification and operator docs

## Roadmap 7: Minimal Chat Clients

- [x] Shared client contract adoption
- [x] Web chat surface
- [x] TUI chat surface
- [x] Cross-client verification

## Roadmap 8: Ingress Routing Closure

- [x] Route-aware run creation
- [x] Connector ingress contract
- [x] Ingress event and recovery closure
- [x] Contract and documentation closure

## Roadmap 9: Provider Identity And Profiles

- [x] Provider profile resource model
- [x] Provider inventory and introspection
- [x] Provider preflight and health checks
- [x] Provider resolution and override policy
- [x] Provider contracts and operator docs

## Roadmap 10: Managed Coding Providers

- [x] Managed provider auth surface
- [x] Claude managed provider
- [x] Codex or ChatGPT managed provider
- [x] Model catalog and compatibility metadata

## Roadmap 11: First IM Channel Loop

- [x] IM connector runtime contract
- [x] Discord connector implementation
- [x] Inbound-to-reply execution loop
- [x] IM operator docs and end-to-end verification

## Roadmap 12: Channel Reply Progression

- [x] Channel capability and degradation model
- [x] Streaming-capable reply progression in daemon runtime
- [x] Discord thinking and streaming UX
- [x] Operator docs and contract closure

## Roadmap 13: Provider Streaming Timeout Semantics

- [x] Streaming timeout contract
- [x] OpenAI-compatible SSE transport refactor
- [x] Dispatch and partial-result semantics
- [x] Channel fallback and operator visibility

## Roadmap 14: Test Environment Workflow

- [x] Test-vs-production environment defaults
- [x] Quick-start repository entry points
- [x] Project-level test environment skill
- [x] Documentation closure

## Roadmap 15: Skill Registry And Prompt Support

- [x] Skill source and overlay discovery
- [x] Skill registry and inspection API
- [x] Explicit skill support in chat
- [x] Operator documentation and verification

## Roadmap 16: Sandbox Execution Plane

- [ ] Sandbox contract and policy model
- [ ] Sandbox control plane APIs
- [ ] Subprocess sandbox runner
- [ ] Isolation controls and audit trail

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
- [x] Add data migration and versioning plan

### Sessions And Routing Foundation

- [x] Define session resource model
- [x] Implement direct and group session isolation
- [x] Implement session list/get/reset routes
- [x] Implement inbound routing model
- [x] Bind runs to sessions where needed

### Policy Foundation

- [x] Define policy engine contract
- [x] Add approval model
- [x] Add policy decision events
- [x] Add local-first auth and pairing model
- [x] Approval durability
- [x] Approval enforcement integration

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
- [x] Create auth and pairing schemas
- [x] Create llm dispatch schemas
- [x] Add schema enforcement and generation pipeline
- [x] Add runtime unit tests
- [x] Add API unit tests
- [x] Add event bus unit tests
- [x] Add persistence tests
- [x] Add checkpoint tests
- [x] Add config loading tests
- [x] Add integration tests across daemon restart
- [x] Add auth and approval restart tests
- [x] Add llm dispatch tests

## Working Rule

When work completes:

- update the task in this registry
- update the roadmap status in [15-daemon-roadmaps.md](/Users/John/Code/agent-os/docs/runtime/daemon-roadmaps.md) if the task affects roadmap closure
- do not mark a task or roadmap complete until its full boundary is actually closed
