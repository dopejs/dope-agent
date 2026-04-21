# Research: Tool-Call Orchestration

## Decisions

### Decision: Introduce first-class workflow resources nested under runs rather than overloading existing run or step records

- Rationale: The current runtime model exposes `Run`, `Step`, and `ToolCall` as concrete
  execution truth. Roadmap 24 adds a new planning and coordination layer that needs
  workflow-level rationale, dependency edges, retry policy, handoff visibility, and
  interruption truth. A distinct workflow resource under a run keeps planning and
  execution semantics explicit without mutating run or step meanings into something less
  auditable.
- Alternatives considered:
  - Reuse `Run` as the workflow plan and store orchestration metadata directly on run
    fields.
    - Rejected because existing run state is too coarse and would blur runtime lifecycle
      with workflow-specific planning and interruption semantics.
  - Reuse `Step` as both workflow node and concrete execution unit.
    - Rejected because one workflow step can require retries, handoffs, approval preview,
      and dependency state before any concrete runtime step exists.

### Decision: Compile each workflow step onto the existing runtime plane as one runtime step with one or more tool-call attempts

- Rationale: The spec requires every concrete action to preserve the same policy,
  approval, sandbox, provenance, and audit guarantees as an individual tool call today.
  The safest design is for orchestration to create a normal runtime step for each
  workflow step that begins execution, then reuse existing tool-call create, running,
  complete, fail, deny, and cancel semantics for attempts under that step.
- Alternatives considered:
  - Execute workflow steps inside a new orchestrator-specific executor without runtime
    step or tool-call records.
    - Rejected because it would create a hidden execution boundary and bypass current
      provenance and approval truth.
  - Create a fresh runtime step for every retry attempt.
    - Rejected because it makes operator history harder to follow and turns retry into
      structural plan churn rather than bounded execution of the same workflow step.

### Decision: Planning should be goal-driven, synchronous, and inspectable before workflow execution begins

- Rationale: Clarification established that operators provide a workflow goal and the
  daemon produces a plan. The plan should be materialized as a persisted workflow record
  before any execution starts so operators can inspect selected tools, dependency order,
  handoff intent, and expected approvals without reading raw logs or racing an active run.
- Alternatives considered:
  - Generate a plan lazily on the first execution request and hide the pre-execution
    representation.
    - Rejected because it weakens inspectability and makes planning failure look like
      execution failure.
  - Support operator-authored explicit workflow definitions as the primary phase-24
    interface.
    - Rejected because the clarified spec makes goal-driven planning the canonical entry
      point for this roadmap.

### Decision: Phase 24 graph support should stay sequential-first with explicit bounded dependencies and no mid-run replanning

- Rationale: The clarified scope requires sequential workflows plus a bounded set of
  explicit dependency or branch relationships, not a general graph planner. The plan
  should therefore store dependency edges and optional branch outcomes explicitly, freeze
  once execution starts, and allow bounded step-level retries only.
- Alternatives considered:
  - Implement a general DAG planner with dynamic branch generation and automatic
    replanning.
    - Rejected because it expands scope, complicates debugging, and weakens auditability
      for the first orchestration slice.
  - Restrict phase 24 to strict linear execution only.
    - Rejected because the roadmap and clarified spec both require limited graph-shaped
      workflows.

### Decision: Approval remains attached to concrete workflow steps, while planning exposes approval expectations only

- Rationale: The current tool-call plane already owns approval enforcement, including
  pending and rejected outcomes tied to concrete resources. Phase 24 should therefore
  preview expected approval mode per planned workflow step, but actual approval requests,
  denials, and rejections remain step-scoped at execution time.
- Alternatives considered:
  - Add a single workflow-level approval gate that authorizes every later step.
    - Rejected because it would create a bypass around existing step-level policy truth.
  - Ignore approvals during planning and expose them only when execution blocks.
    - Rejected because the roadmap requires operator-visible rationale and policy
      expectations before execution.

### Decision: Restart should preserve workflow audit state but interrupt any in-flight workflow execution in phase 24

- Rationale: Clarification fixed restart behavior: completed workflow state and completed
  step truth survive restart, but active workflows become explicitly interrupted instead of
  auto-resuming. This keeps restart behavior restart-safe and auditable without expanding
  the phase into orchestration checkpoint replay and automatic resumption.
- Alternatives considered:
  - Automatically resume in-flight workflows after daemon restart.
    - Rejected because it requires a more complex checkpoint, idempotency, and side-effect
      model than phase 24 needs.
  - Drop in-flight workflow state on restart and keep only underlying runtime records.
    - Rejected because it would lose the workflow-level audit truth the roadmap requires.

### Decision: Mixed-workflow handoffs should be stored as explicit structured records rather than implicit prompt text

- Rationale: The roadmap requires cross-tool data flow and handoff to remain
  operator-visible. Each dependency that passes data from one workflow step to another
  should therefore record a small structured handoff summary and source/target linkage so
  inspection and debugging do not depend on reconstructing prompt text or raw logs.
- Alternatives considered:
  - Rely on the runtime step output payload only.
    - Rejected because later consumers and operators need a direct handoff relationship,
      not just two disconnected step records.
  - Store only free-form rationale text on the workflow.
    - Rejected because it is too weak for testability and contract-backed inspection.

## Implementation Notes

- New workflow routes should remain nested under runs so orchestration is clearly a layer
  on top of existing runtime sessions rather than a second top-level execution system.
- Runtime `Step` and `ToolCall` resources should gain additive workflow linkage fields so
  operators can move between workflow inspection and concrete execution history.
- Persistence should use additive workflow tables plus nullable linkage on runtime records
  where that improves queryability; the design should not repurpose the `tool_calls` table
  as a workflow table.
- Rate limiting and scheduler fairness for many concurrent workflows are intentionally out
  of scope for phase 24 and can be deferred unless implementation uncovers a concrete
  correctness issue.

## Implemented Result

- The phase landed with workflow orchestration logic in
  `daemon/internal/api/workflows.go`, additive workflow/resource types in
  `daemon/internal/orchestration/types.go`, and additive persistence in
  `daemon/internal/store/store.go`.
- SQLite schema version `11` adds `workflows`, `workflow_steps`,
  `workflow_dependencies`, and `workflow_handoffs` plus nullable workflow linkage on
  runtime `steps` and `tool_calls`.
- Restart recovery in `daemon/internal/app/app.go` now marks in-flight workflows
  `interrupted` per environment scope without mutating completed historical steps or tool
  calls.
