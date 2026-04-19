# Runtime Architecture

## Architecture Boundary

The runtime is the agent core. It should not be collapsed into prompt templates or tool wrappers.

The runtime is responsible for:

- run lifecycle
- step execution
- checkpoint and resume
- memory recall and candidate capture
- tool execution coordination
- policy enforcement
- observation and replay

## Core Objects

### Run

A top-level execution instance.

Suggested fields:

- `run_id`
- `entrypoint`
- `user_id`
- `workspace_id`
- `goal`
- `status`
- `created_at`
- `updated_at`

### Thread

A logical conversation or workstream inside a run or across runs.

### Step

An atomic unit of execution.

Suggested statuses:

- `queued`
- `planning`
- `calling_model`
- `executing_tool`
- `waiting_input`
- `blocked`
- `completed`
- `failed`
- `cancelled`

### Event

An append-only record of what happened.

Examples:

- user input received
- memory recalled
- model response generated
- tool call requested
- tool call finished
- policy denied
- checkpoint saved

### Checkpoint

A recoverable runtime snapshot.

Should include enough information to resume without reconstructing state from loose summaries.

### Artifact

Structured outputs such as files, patches, reports, command outputs, or attachments.

### PolicyDecision

A structured decision made by the policy layer.

Examples:

- allow tool execution
- deny escalation
- suppress long-term memory write
- require confirmation for external side effects

## Runtime Subsystems

### Orchestrator

Accepts tasks, creates runs, and coordinates execution across local agents or specialized workers.

### Execution Engine

Runs the step state machine and owns retries, timeouts, cancellation, and checkpointing.

### Memory Service Adapter

Provides structured recall and candidate-write access to the memory plane.

### Tool Gateway

Owns tool schemas, permissions, idempotency keys, execution envelopes, and structured output capture.

For the current high-risk tool-call path, the gateway also carries declaration-backed consumer provenance and policy-record linkage even before generic local-tool subprocess execution is migrated onto sandbox.

### Planner

Can use model output to propose plans, but execution should still be gated by runtime policy.

### Replay And Observability Layer

Provides event stream inspection, timeline reconstruction, failure triage, and debugging support.

## Recommended Execution Model

- event-driven
- append-first
- checkpointable
- recoverable after partial failure
- explicit about human input waits and permission boundaries

## Invariants

- Step order within a run is reconstructable.
- Tool calls are idempotent or duplicate-detectable.
- A run can resume from the most recent checkpoint.
- Model output alone is not treated as durable state.
- Policy decisions are observable and attributable.
- Current sandbox-backed and approval-gated local consumers preserve consumer kind, consumer instance, and redacted secret-resolution outcomes in durable records.
