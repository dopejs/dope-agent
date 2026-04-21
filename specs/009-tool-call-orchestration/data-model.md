# Data Model: Tool-Call Orchestration

## Entities

### Workflow Resource

- Purpose: Operator-visible orchestration record for one goal-driven workflow planned and
  executed inside a run.
- Fields:
  - `workflowId`
  - `runId`
  - `goal`
  - `status`: `planning`, `planning_failed`, `planned`, `running`, `blocked`,
    `completed`, `partial_failed`, `failed`, `cancelled`, or `interrupted`
  - `planSummary`: short explanation of selected tool sequence or dependency graph
  - `failureSummary`: short operator-facing terminal reason when not successful
  - `createdAt`
  - `updatedAt`
  - `startedAt`
  - `completedAt`
  - `interruptedAt`
  - `steps[]`
  - `dependencies[]`
  - `handoffs[]`
- Validation rules:
  - a workflow belongs to exactly one existing run
  - a workflow must reach `planned` before `running`
  - once `running`, the workflow plan is immutable except for additive execution-state
    fields such as attempt counts, linked runtime IDs, timestamps, and terminal reasons
  - `interrupted` is only valid when the daemon restarts before the workflow reaches a
    terminal completed, failed, partial-failed, or cancelled state

### Workflow Step Record

- Purpose: One planned unit of orchestration work that resolves to a concrete runtime step
  when execution starts.
- Fields:
  - `workflowStepId`
  - `workflowId`
  - `title`
  - `position`
  - `consumerKind`: `local_tool`, `skill`, or `mcp_tool`
  - `consumerId`
  - `toolName`
  - `status`: `planned`, `waiting_dependency`, `ready`, `blocked`, `running`,
    `completed`, `failed`, `cancelled`, `interrupted`, or `skipped`
  - `selectionRationale`
  - `approvalModeExpected`: `allow`, `ask`, or `deny`
  - `dependencyIds[]`
  - `runtimeStepId`
  - `activeToolCallId`
  - `attemptCount`
  - `maxAttempts`
  - `lastFailureClass`
  - `sideEffectsVisible`
  - `outputSummary`
  - `createdAt`
  - `updatedAt`
- Validation rules:
  - `position` is unique within a workflow
  - `runtimeStepId` must be empty before execution and non-empty once the step starts
  - `attemptCount` starts at `0` and must never exceed `maxAttempts`
  - `approvalModeExpected` is derived from the selected consumer contract and is preview
    truth only; concrete approval outcome still belongs to the executed runtime step and
    tool call
  - `sideEffectsVisible` becomes true once at least one linked tool call produces an
    externally visible or persisted effect that should remain visible even if the workflow
    later partially fails

### Workflow Dependency Edge

- Purpose: Explicit relationship that constrains when one workflow step may start and what
  terminal condition it depends on.
- Fields:
  - `dependencyId`
  - `workflowId`
  - `fromWorkflowStepId`
  - `toWorkflowStepId`
  - `dependencyType`: `success`, `failure`, or `completion`
  - `reason`
- Validation rules:
  - dependency edges must remain acyclic inside one workflow
  - every referenced step must belong to the same workflow
  - phase 24 only supports explicit bounded dependencies; no dynamically generated edges
    may appear after execution begins

### Workflow Handoff Record

- Purpose: Operator-visible summary of the data or state passed from one workflow step to
  another.
- Fields:
  - `handoffId`
  - `workflowId`
  - `fromWorkflowStepId`
  - `toWorkflowStepId`
  - `status`: `pending`, `available`, `consumed`, or `invalid`
  - `payloadSummary`
  - `sourcePath`
  - `consumedAt`
  - `invalidReason`
- Validation rules:
  - every handoff must reference an existing dependency edge
  - `payloadSummary` must remain redacted or summarized if the underlying step output
    contains secret-bearing or operator-sensitive material
  - `invalid` handoffs must explain why downstream execution could not safely use the
    produced output

### Runtime Linkage Record

- Purpose: Stable cross-reference between workflow inspection and concrete runtime
  execution truth.
- Fields:
  - `workflowId`
  - `workflowStepId`
  - `runtimeStepId`
  - `toolCallId`
  - `attempt`
  - `invocationKind`
  - `createdAt`
- Validation rules:
  - every executing workflow step must create at least one runtime linkage record
  - retries create a new linkage record for the new tool call attempt while preserving the
    same `workflowStepId`
  - linkage records never exist without the corresponding runtime step and tool call

## State Transitions

### Workflow Lifecycle

- `planning` -> `planned` when the daemon successfully selects steps, dependencies,
  handoff intent, and approval expectations
- `planning` -> `planning_failed` when no valid inspectable workflow plan can be produced
- `planned` -> `running` when the operator starts the workflow
- `running` -> `blocked` when the next eligible step is prevented by approval denial,
  policy block, or consumer unavailability
- `running` -> `partial_failed` when at least one completed step has visible side effects
  and a later step reaches terminal failure
- `running` -> `failed` when execution reaches terminal failure without prior visible
  side-effecting completion
- `running` -> `completed` when all non-skipped steps complete successfully
- `planned` or `running` -> `cancelled` when the operator cancels the workflow
- `running` -> `interrupted` when daemon restart occurs before the workflow reaches a
  terminal state

### Workflow Step Lifecycle

- `planned` -> `waiting_dependency` when the workflow starts but prerequisite steps are
  not yet satisfied
- `planned` or `waiting_dependency` -> `ready` when dependencies are satisfied and the
  step is eligible to run
- `ready` -> `blocked` when the step cannot begin due to approval denial, policy block,
  or consumer unavailability
- `ready` -> `running` when the daemon creates the linked runtime step and starts the
  first tool call attempt
- `running` -> `ready` when the step fails in a retriable way and the bounded retry budget
  allows another attempt
- `running` -> `completed` when a linked tool call succeeds
- `running` -> `failed` when the terminal failure is non-retriable or retry budget is
  exhausted
- `running` -> `cancelled` when operator cancellation stops the active attempt
- `running` -> `interrupted` when daemon restart interrupts the active attempt
- `planned` or `waiting_dependency` -> `skipped` when a bounded branch condition makes the
  step inapplicable for the frozen workflow plan

## Derived Views

- `GET /v1/runs/{runId}/workflows` lists workflow resources with top-level status and
  summary fields suitable for operator inspection.
- `GET /v1/runs/{runId}/workflows/{workflowId}` expands steps, dependencies, handoffs,
  retry counts, and runtime linkage so operators can inspect the frozen plan and current
  execution state in one resource.
- existing runtime step and tool-call resources gain additive `workflowId`,
  `workflowStepId`, and `attempt` linkage so operators can pivot from workflow state to
  concrete execution history without ambiguous joins.
- workflow status-change and step-status-change events, plus current workflow resources,
  reconstruct planning, retry, block, partial-failure, cancellation, and restart
  interruption behavior without raw log access.

## Implementation Notes

- The implemented workflow resource is environment-scoped in the daemon SQLite store and
  loaded through additive workflow tables rather than overloading `runs`, `steps`, or
  `tool_calls`.
- The current phase keeps planning and execution control in `daemon/internal/api/workflows.go`
  while storing durable workflow truth in `daemon/internal/orchestration/types.go` and
  `daemon/internal/store/store.go`.
- Restart recovery marks active workflows and active workflow steps `interrupted`, but
  leaves completed runtime steps and tool calls unchanged for audit continuity.
