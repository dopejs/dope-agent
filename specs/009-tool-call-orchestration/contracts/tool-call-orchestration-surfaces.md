# Contract Surfaces: Tool-Call Orchestration

## Goal

Add goal-driven workflow planning and execution on top of the existing daemon-owned
runtime tool-call plane without creating a second execution boundary.

## HTTP API Surfaces

### New Workflow Planning And Inspection Routes

- `POST /v1/runs/{runId}/workflows`
- `GET /v1/runs/{runId}/workflows`
- `GET /v1/runs/{runId}/workflows/{workflowId}`
- `POST /v1/runs/{runId}/workflows/{workflowId}/start`
- `POST /v1/runs/{runId}/workflows/{workflowId}/cancel`

Request and response requirements:

- workflow planning input is goal-driven:
  - request may use the run goal by default
  - request may optionally accept a workflow-goal override when the operator wants a
    different goal than the stored run goal
- `POST /v1/runs/{runId}/workflows` persists a workflow resource in `planned` or
  `planning_failed` state before any workflow step executes
- workflow detail exposes:
  - top-level workflow status
  - plan summary and failure summary
  - planned workflow steps
  - dependency edges
  - handoff summaries
  - expected approval mode per step
  - runtime linkage for steps that have started
  - retry counts and interruption truth
- `start` transitions a `planned` workflow into execution on the existing runtime plane
- `cancel` surfaces explicit cancellation truth for both top-level workflow state and the
  active workflow step

Schema surfaces:

- add `schemas/api/create-workflow.request.schema.json`
- add `schemas/api/workflow-resource.schema.json`
- add `schemas/api/workflow-list.response.schema.json`
- add `schemas/api/workflow-step-resource.schema.json`
- add `schemas/api/workflow-dependency-resource.schema.json`
- add `schemas/api/workflow-handoff-resource.schema.json`

### Existing Run And Runtime Surfaces Extended

- `GET /v1/runs`
- `GET /v1/runs/{runId}`
- existing step and tool-call routes under `/v1/runs/{runId}/steps/...`

Additive requirements:

- run resources may add summary workflow linkage fields such as `activeWorkflowId` or
  `workflowCount` if needed for operator navigation, but existing run semantics remain
  valid
- step resources gain additive workflow linkage:
  - `workflowId`
  - `workflowStepId`
  - `attempt`
- tool-call resources gain additive workflow linkage:
  - `workflowId`
  - `workflowStepId`
  - `attempt`
- existing single-step tool-call routes remain valid without workflow creation

Schema surfaces:

- update `schemas/api/run-resource.schema.json`
- update `schemas/api/step-resource.schema.json`
- update `schemas/api/tool-call-resource.schema.json`
- update `schemas/api/tool-call-list.response.schema.json` if linkage fields are
  represented there through the referenced resource

## Event And History Surfaces

New workflow event families:

- `workflow.planned`
- `workflow.started`
- `workflow.status_changed`
- `workflow.step_status_changed`

Event payload requirements:

- `workflowId`
- `runId`
- top-level or step-level status
- `workflowStepId` when applicable
- `consumerKind`, `consumerId`, and `toolName` for step-status events
- `runtimeStepId` and `toolCallId` when execution has started
- `attempt` for retried step transitions
- `approvalModeExpected` and actual blocked or denied reason when relevant
- `failureClass` or interruption reason for terminal or interrupted outcomes

Schema surfaces:

- add `schemas/events/workflow-planned.event.schema.json`
- add `schemas/events/workflow-started.event.schema.json`
- add `schemas/events/workflow-status-changed.event.schema.json`
- add `schemas/events/workflow-step-status-changed.event.schema.json`

Truthfulness rules:

- planning failure must be distinguishable from execution failure
- blocked, cancelled, partial-failed, and interrupted workflow states must be
  distinguishable from each other
- step retries must be visible as bounded repeated attempts on one workflow step, not as
  hidden background behavior
- daemon restart interruption must be visible through workflow status rather than inferred
  indirectly from missing events

## Persistence Surfaces

Persistence remains additive to the existing runtime store:

- add a `workflows` table for top-level workflow resources
- add a `workflow_steps` table for planned step records
- add a `workflow_dependencies` table for explicit dependency edges
- add a `workflow_handoffs` table for operator-visible cross-step handoff truth
- add nullable workflow linkage columns to runtime `steps` and `tool_calls` if needed for
  efficient reverse lookup and operator inspection

Persistence rules:

- workflow rows are environment-scoped inside the same daemon-owned SQLite store used by
  runs, steps, tool calls, approvals, and events
- workflow interruption on restart updates workflow and active workflow-step state without
  altering completed historical runtime steps or tool calls
- the store must not create a second execution ledger separate from the current runtime
  plane

## Documentation Surfaces

Docs updated with the implementation:

- `docs/harness/harness-architecture.md`
- `docs/runtime/daemon-roadmaps.md`
- operator-facing runtime or harness docs covering workflow lifecycle, approval
  expectations, mixed-family guarantees, retry semantics, cancellation behavior, and
  restart interruption truth

## Truthfulness Constraints

- workflow planning must be inspectable before execution begins
- workflow execution must stay on the existing runtime step and tool-call plane
- approval remains step-scoped; no workflow-level approval shortcut is introduced
- frozen workflow plans may retry a step within a bounded budget but may not automatically
  replan remaining steps in phase 24
- mixed MCP, skill, and local-tool workflows must not introduce bypass paths around
  existing sandbox, approval, or redaction controls
- existing run, step, and tool-call consumers must remain backward compatible if they
  never use the new workflow routes

## Implemented Closure

- Implemented routes:
  - `POST /v1/runs/{runId}/workflows`
  - `GET /v1/runs/{runId}/workflows`
  - `GET /v1/runs/{runId}/workflows/{workflowId}`
  - `POST /v1/runs/{runId}/workflows/{workflowId}/start`
  - `POST /v1/runs/{runId}/workflows/{workflowId}/cancel`
- Implemented additive schema surfaces:
  - `schemas/api/create-workflow.request.schema.json`
  - `schemas/api/workflow-resource.schema.json`
  - `schemas/api/workflow-list.response.schema.json`
  - `schemas/api/workflow-step-resource.schema.json`
  - `schemas/api/workflow-dependency-resource.schema.json`
  - `schemas/api/workflow-handoff-resource.schema.json`
  - additive workflow linkage on run, step, and tool-call resource schemas
- Implemented additive event surfaces:
  - `workflow.planned`
  - `workflow.started`
  - `workflow.status_changed`
  - `workflow.step_status_changed`
- Verified on 2026-04-21 with a repo-owned MCP stdio helper plus executable skills in
  `KURA_ENV=test`, including blocked mixed-workflow truth before MCP allowlisting and a
  successful mixed MCP+skill workflow after allowlisting.
