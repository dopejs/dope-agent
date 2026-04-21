# Feature Specification: Tool-Call Orchestration

**Feature Branch**: `[010-tool-call-orchestration]`  
**Created**: 2026-04-21  
**Status**: Draft  
**Input**: User description: "结合 docs/specs/009-tool-call-orchestration.md 完成 phase 24 的工作"

## Clarifications

### Session 2026-04-21

- Q: phase 24 必须支持哪种 workflow 规划入口？ → A: 支持 operator 提交目标，由 daemon 生成并暴露可检查的 workflow plan。
- Q: phase 24 的 workflow 结构范围是什么？ → A: 支持顺序 workflow，并支持有限的显式分支/依赖关系。
- Q: phase 24 的 approval 语义应该落在哪一层？ → A: 维持逐 step 审批；workflow plan 只提前展示各 step 的审批预期。
- Q: step 失败后的 orchestration 行为是什么？ → A: plan 一旦确认执行即冻结；只允许 step 级有界重试，不自动重规划。
- Q: daemon 重启时，正在运行的 workflow 应该怎么处理？ → A: daemon 重启后保留 workflow 状态与已完成记录，但未完成 workflow 终止为可见中断状态。

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Inspect Workflow Planning Truth (Priority: P1)

As an operator, I need the daemon to show why it planned a workflow in a particular
order or graph so I can verify that the selected tool sequence matches policy,
approvals, and expected execution intent before or during execution.

**Why this priority**: If planning rationale is hidden, operators cannot safely trust
multi-step execution, especially when different tool families and approval boundaries are
involved.

**Independent Test**: Start a workflow that requires more than one step, inspect the
daemon-visible workflow record, and confirm the selected steps, ordering, rationale, and
approval expectations can be understood without reading raw logs.

**Acceptance Scenarios**:

1. **Given** an operator requests a workflow that requires multiple tool steps, **When**
   the operator submits a workflow goal and the daemon creates a workflow plan, **Then**
   the operator can inspect the selected steps, ordering or graph relationships, and
   rationale for each step before execution proceeds.
2. **Given** a workflow includes steps with different approval or policy requirements,
   **When** the operator inspects the workflow plan, **Then** each step shows the policy
   or approval expectations that still apply at execution time.
3. **Given** a workflow cannot produce a valid plan, **When** planning ends, **Then** the
   daemon records an explicit planning failure instead of silently attempting ad hoc
   execution.

---

### User Story 2 - Execute A Controlled Multi-Step Workflow (Priority: P2)

As an operator, I need the daemon to execute a multi-step workflow through the existing
runtime tool-call plane so ordered execution, retries, and cancellation do not create an
unmanaged execution path.

**Why this priority**: The phase is only complete if orchestration remains subordinate to
the current runtime truth instead of creating a second execution mechanism that weakens
policy, provenance, or audit guarantees.

**Independent Test**: Run a workflow with multiple steps, including at least one retry,
one cancellation path, or one partial-failure path, and confirm every executed step is
recorded as normal runtime work with explicit state transitions.

**Acceptance Scenarios**:

1. **Given** a valid workflow plan exists, **When** the daemon executes it, **Then**
   every concrete step runs through the existing runtime tool-call plane with normal
   policy, approval, sandbox, and provenance handling.
2. **Given** one workflow step fails after one or more earlier steps already produced
   visible side effects, **When** the workflow stops or degrades, **Then** the daemon
   preserves earlier step truth and records the workflow as partially failed.
3. **Given** a workflow step is retriable, **When** the daemon retries it, **Then** the
   retry attempts and final outcome remain visible in operator-facing workflow history.
4. **Given** an operator cancels a workflow while it is running, **When** cancellation is
   processed, **Then** the daemon records which steps completed, which step was active,
   and what state the workflow ended in.
5. **Given** the daemon restarts while a workflow is still running, **When** the operator
   inspects the workflow after restart, **Then** the daemon preserves completed step
   truth and marks the unfinished workflow as explicitly interrupted rather than silently
   resuming it.

---

### User Story 3 - Coordinate Mixed Tool Families Safely (Priority: P3)

As an operator, I need one workflow to combine MCP tools, local tools, and executable
skills without losing policy, provenance, or handoff visibility so mixed workflows remain
safe to inspect and debug.

**Why this priority**: Real workflows require handoffs between different tool families,
and that is the main risk point where policy or audit truth can be bypassed if the design
is weak.

**Independent Test**: Run at least one workflow in `DOPE_ENV=test` that combines at
least two consumer families, inspect the recorded handoffs between steps, and verify the
workflow does not use any out-of-band execution path.

**Acceptance Scenarios**:

1. **Given** a workflow needs tools from at least two consumer families, **When** the
   daemon plans and executes it, **Then** the workflow can combine those families without
   bypassing existing sandbox or approval controls.
2. **Given** one workflow step produces data needed by a later step from another
   consumer family, **When** the later step runs, **Then** the operator can inspect the
   handoff relationship between the producing and consuming steps.
3. **Given** a mixed workflow step becomes blocked by approval denial, sandbox policy, or
   consumer unavailability, **When** the workflow is inspected, **Then** the operator can
   distinguish blocked execution from ordinary tool failure.

### Edge Cases

- What happens when a workflow plan is valid at creation time but a later step becomes
  unavailable before execution reaches it?
- How does the daemon represent a workflow where some side effects are already visible and
  a later step fails permanently?
- What happens when operator approval is denied for one step in an otherwise valid
  workflow?
- How does cancellation behave when the active step cannot stop immediately?
- How does the daemon expose an in-flight workflow that is interrupted by daemon restart?
- What happens when data produced by one step is insufficient or invalid for a later step
  from a different consumer family?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST support a first-class orchestration model for multi-step
  tool workflows with ordered or graph-shaped execution.
- **FR-001a**: The system MUST accept an operator-provided workflow goal as orchestration
  input and generate an operator-visible workflow plan before executing workflow steps.
- **FR-001b**: Phase 24 orchestration MUST support sequential workflows plus a bounded
  set of explicit dependency or branch relationships; a fully general graph planner is
  out of scope for this phase.
- **FR-002**: The system MUST make workflow planning decisions explicit, inspectable, and
  auditable before and during execution.
- **FR-003**: The system MUST execute each concrete workflow step on the existing runtime
  tool-call plane rather than introducing a separate execution boundary.
- **FR-004**: Each orchestrated step MUST preserve the same policy, approval, sandbox,
  provenance, and audit guarantees it would have as an individual runtime tool call.
- **FR-004a**: Workflow planning MUST expose the expected approval requirement for each
  step, but approval enforcement MUST remain at the concrete step execution boundary.
- **FR-005**: The system MUST record workflow-level status that distinguishes planning
  failure, running, completed, cancelled, blocked, and partially failed outcomes.
- **FR-006**: The system MUST preserve operator-visible truth for already completed or
  side-effecting steps even when later workflow steps fail, retry, or are cancelled.
- **FR-007**: The system MUST surface retry attempts, cancellation activity, and
  partial-failure semantics in operator-visible workflow history.
- **FR-007a**: Once execution begins, the workflow plan MUST remain fixed for that
  workflow run; phase 24 MAY perform bounded retries for a step but MUST NOT replan the
  remaining workflow automatically.
- **FR-008**: The system MUST support workflows that span at least two of the following
  consumer families: MCP tools, local tools, and executable skills.
- **FR-009**: The system MUST keep data handoff and dependency relationships between
  workflow steps inspectable to operators.
- **FR-010**: The system MUST preserve existing single-step tool-call behavior so
  operators can continue to run non-orchestrated tool calls without workflow-specific
  regression.
- **FR-011**: The system MUST make approval-denied, policy-blocked, and
  consumer-unavailable workflow steps distinguishable from ordinary execution failure.
- **FR-012**: The system MUST keep orchestration state and audit truth environment-scoped
  so test and production histories do not leak across environments.
- **FR-012a**: If the daemon restarts during workflow execution, the system MUST preserve
  workflow state and completed-step history but mark the unfinished workflow as
  interrupted rather than automatically resuming it in phase 24.
- **FR-013**: The system MUST allow operators to verify at least one mixed workflow
  end-to-end using the daemon-owned runtime plane in `DOPE_ENV=test`.

### Key Entities *(include if feature involves data)*

- **Workflow Plan**: The operator-visible description of workflow intent, selected steps,
  ordering or graph relationships, rationale, and approval or policy expectations.
- **Workflow Step Record**: The operator-visible record for one step in a workflow,
  including consumer family, execution state, retry history, approval outcome, and
  provenance link to runtime execution truth.
- **Workflow Execution Record**: The aggregate runtime history for a workflow, including
  lifecycle status, partial-failure truth, cancellation state, and completed side effects.
- **Step Handoff Record**: The operator-visible description of how output or state from
  one workflow step is used by a later step.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Runtime history, event, schema, and operator-inspection
  surfaces gain additive workflow-planning, step-state, and handoff visibility. Existing
  single-step tool-call behavior remains valid.
- **Migration / Rollback**: Rollback is a revert of additive orchestration resources,
  workflow metadata, and operator surfaces while preserving the current single-step
  runtime plane. No parallel execution plane may be introduced that would require a
  separate rollback path.
- **Verification Strategy**: Run targeted daemon tests for planning truth, ordered or
  graph execution, retries, cancellation, partial failure, and mixed-family workflows; run
  contract coverage for workflow-visible runtime and event surfaces; manually validate at
  least one mixed workflow in `DOPE_ENV=test`.
- **Observability Impact**: Operator-visible surfaces must explain workflow rationale,
  step ordering or graph relationships, approval and policy status per step, retry and
  cancellation activity, partial failure, and cross-family handoff truth.
- **Environment & Secrets**: Validation stays in `DOPE_ENV=test` by default. Mixed
  workflows must continue to respect existing secret-redaction, approval, and sandbox
  controls for every concrete step.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can explain why a workflow was planned in a particular sequence or
  graph within 5 minutes using daemon-visible inspection only.
- **SC-002**: At least one workflow spanning at least two consumer families completes
  end-to-end in `DOPE_ENV=test` without any out-of-band execution path.
- **SC-003**: 100% of validated retry, cancellation, and partial-failure scenarios remain
  reconstructable from operator-visible workflow history and events.
- **SC-004**: 100% of validated approval-denied, policy-blocked, and
  consumer-unavailable step outcomes are distinguishable from ordinary execution failure.
- **SC-005**: Existing single-step tool-call flows continue to pass their in-scope
  validation coverage with no workflow-specific operator input required.

## Assumptions

- Roadmap 19 sandbox execution and Roadmap 21 MCP runtime foundations are already closed
  and remain the base execution guarantees for this phase.
- Existing single-step runtime tool calls, approval handling, and audit history remain
  authoritative and must not be replaced by orchestration-specific side paths.
- Workflow approval remains step-scoped; phase 24 does not replace existing concrete
  step approval with a single workflow-level approval gate.
- Phase 24 requires one production-grade orchestration slice, not an open-ended
  autonomous planner or memory-driven agent loop.
- Workflow planning is goal-driven: the operator provides the workflow goal, and the
  daemon generates the inspectable workflow plan rather than requiring a fully explicit
  step-by-step workflow definition as input.
- The first orchestration slice is sequential-first and only needs limited explicit
  branch or dependency relationships rather than a fully general graph planner.
- Workflow execution is plan-frozen once started; bounded step retries are allowed, but
  automatic mid-run replanning is out of scope for phase 24.
- A workflow may expose partial side effects; automatic rollback of external side effects
  is out of scope unless an underlying tool already supports it.
- The first validation target is operator-driven execution in `DOPE_ENV=test`; production
  rollout behavior can remain gated behind the same existing policy and approval controls.
- Daemon restart preserves workflow audit truth and completed-step records, but automatic
  resume of interrupted in-flight workflows is out of scope for phase 24.
