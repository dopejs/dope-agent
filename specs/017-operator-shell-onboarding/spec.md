# Feature Specification: Operator Shell And Onboarding

**Feature Branch**: `[017-operator-shell-onboarding]`  
**Created**: 2026-04-23  
**Status**: Draft  
**Input**: User description: "结合 docs/specs/017-operator-shell-and-onboarding.md 完成 phase 32 的工作"

## Clarifications

### Session 2026-04-23

- Q: “首个有用动作”在 phase 32 中应如何定义？ → A: 从 shell 发起一次受限的测试查询或测试运行，并立即在 shell 内看到结果与相关状态。
- Q: approval inbox 在 phase 32 中是否必须支持直接处理审批？ → A: 必须支持直接 approve/reject，并显示处理结果。
- Q: onboarding 的完成标准是否要求所有 readiness 项都完成？ → A: 只要求完成所选首个有用动作所需的最小 readiness 集合；其他项可作为后续 setup 保留。
- Q: phase 32 是否要求 shell 内置 test/live 环境切换？ → A: 不要求；shell 只需明确显示当前环境并保持严格环境隔离，环境切换可留给 shell 外入口或后续阶段。
- Q: phase 32 是否要求同时交付 web 和 TUI 两个等价 operator shell？ → A: 不要求；phase 32 只要求一个 primary operator shell，默认是 web，TUI 可保留为后续工作。

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Complete First-Run Setup (Priority: P1)

As a user setting up the personal agent for the first time, I need a guided operator shell
that shows what is configured, what is missing, and what I should do next so I can reach
the first useful outcome without reading raw daemon routes or source code.

**Why this priority**: Phase 32 is not complete if the product still assumes a developer
operator who can discover setup gaps through raw APIs, logs, or repository knowledge. The
minimum usable slice is a truthful onboarding path from initial shell entry to one
successful first-use action.

**Independent Test**: Start from a fresh test environment, open the operator shell,
follow the onboarding guidance to connect required access, confirm readiness status, and
complete one first-use action without using raw API calls or direct datastore access.

**Acceptance Scenarios**:

1. **Given** a user opens the product in a fresh environment, **When** the operator shell
   loads for the first time, **Then** it presents onboarding progress, missing setup
   items, and a clear next step rather than a blank or developer-only screen.
2. **Given** some required setup is complete and some is still missing, **When** the user
   returns to the shell, **Then** the onboarding state reflects the current daemon truth
   and resumes from the next unfinished operator task.
3. **Given** the shell identifies that a connector, authentication step, or capability is
   not ready, **When** the user inspects that item, **Then** the product explains why it
   is blocked and what action is needed before the first useful action can succeed.
4. **Given** some readiness items are unrelated to the selected first useful action,
   **When** the operator completes the minimum prerequisites for that action, **Then**
   onboarding can be considered complete while the unrelated items remain visible as
   optional follow-up setup.
5. **Given** onboarding prerequisites are satisfied, **When** the user triggers the first
   useful action from the shell as a bounded test query or test run, **Then** the product
   records the outcome immediately, shows related status in the same shell, and keeps the
   operator in place for follow-up inspection.

---

### User Story 2 - Inspect Approvals, Background Work, And Outcomes (Priority: P2)

As an operator, I need one shell that surfaces approvals, schedules, workflows, delivery
outcomes, and recent history so I can understand what the agent is doing without piecing
it together from multiple raw endpoints.

**Why this priority**: The upstream roadmap defines approvals, schedules, workflows, and
delivery inspection as core operator-control work rather than optional polish. Without a
coherent shell, the personal agent remains operationally fragmented and hard to trust.

**Independent Test**: Create representative pending approvals, scheduled work, workflow
history, and delivery outcomes in a test environment, then confirm the shell exposes the
current state and cross-linked context for each operator surface without requiring raw API
navigation.

**Acceptance Scenarios**:

1. **Given** one or more approval decisions are pending, **When** the operator opens the
   approval inbox, **Then** the shell shows who requested each action, what resource or
   side effect is affected, why approval is needed, and whether the request is still
   pending or already resolved.
2. **Given** an approval request is pending, **When** the operator approves or rejects it
   from the shell, **Then** the shell records the decision result and updates the related
   blocked work or approval state without requiring a raw route handoff or navigation to a
   daemon endpoint outside the shell.
3. **Given** schedules, workflows, and deliveries exist for the current environment,
   **When** the operator inspects them from the shell, **Then** the product shows their
   current status, recent transitions, and enough linked context to understand how one
   outcome relates to the others.
4. **Given** a background task completed successfully, **When** the operator inspects its
   recent history, **Then** the shell shows that success as operator-visible truth rather
   than requiring log reconstruction.
5. **Given** the current environment has no pending approvals or recent background work,
   **When** the operator opens those surfaces, **Then** the shell provides truthful empty
   states and does not imply hidden or stale activity.

---

### User Story 3 - Diagnose Why Work Did Not Run Or Deliver (Priority: P3)

As an operator, I need health and diagnostic views that explain why a connector,
scheduled task, workflow, or delivery path failed so I can decide the next corrective
action without reading internal logs first.

**Why this priority**: A personal agent that cannot explain blocked or failed background
work is not trustworthy. Phase 32 must make setup and operations debuggable for normal
operators, not only for developers.

**Independent Test**: Exercise representative failures such as missing setup, degraded
integration readiness, approval-blocked work, schedule failure, and delivery failure, and
verify the shell exposes the failure class, current impact, and next step from one
operator-facing surface.

**Acceptance Scenarios**:

1. **Given** a connector or capability is misconfigured or unavailable, **When** the
   operator opens health and readiness details, **Then** the shell distinguishes missing
   configuration from degraded runtime health and explains the operator action needed.
2. **Given** a scheduled task did not run, **When** the operator inspects that task from
   the shell, **Then** the product shows whether it was paused, blocked, failed at
   launch, or otherwise prevented from running.
3. **Given** a workflow started but did not complete or deliver the expected outcome,
   **When** the operator inspects the workflow and delivery surfaces, **Then** the shell
   preserves workflow truth and delivery truth separately so the operator can identify
   where the failure occurred.
4. **Given** background work is waiting on approval, **When** the operator inspects the
   blocked item, **Then** the shell explains that approval is the current blocker rather
   than presenting the work as silently stalled or failed.
5. **Given** an activity or diagnostic item depends on authoritative schedule, workflow,
   delivery, approval, or computer-use detail, **When** the operator opens that item,
   **Then** the shell exposes the linked detail inside the shell rather than requiring raw
   route navigation.

### Edge Cases

- If onboarding is partially complete and the daemon restarts, the shell resumes from
  durable readiness truth instead of resetting the operator to an empty first-run state.
- If the current environment is healthy but a specific connector or capability is not, the
  shell distinguishes environment health from item-specific readiness.
- If an approval request is resolved outside the current shell session, the inbox reflects
  the updated status rather than leaving the operator with stale pending state.
- If approval handling fails after the operator submits an approve or reject decision, the
  shell preserves the approval request, shows the failure outcome, and does not present
  the request as resolved.
- If a schedule, workflow, or delivery record no longer has complete upstream context, the
  shell preserves the surviving truth and marks missing linkage explicitly rather than
  inventing a complete history.
- If there is no recent operator history, the shell shows an explicit empty state and a
  next step instead of implying that history failed to load.
- If the operator switches between test and live environments, the shell does not merge
  onboarding state, approvals, or execution history across environments.
- If the operator enters the shell from a different environment, the shell makes the
  current environment explicit and loads only that environment's truth rather than trying
  to merge or hot-switch state inside the same operator session.
- If the first useful action fails because of a missing prerequisite, the shell returns the
  operator to the blocking readiness item with a clear explanation.
- If some integrations or capabilities remain unconfigured after onboarding is complete for
  the selected first useful action, the shell keeps them visible as optional follow-up
  setup rather than treating onboarding as incomplete.
- If background work remains blocked on approval, the shell preserves both the blocked work
  and the pending approval record so operators can inspect the causal chain.
- If other client surfaces exist but do not yet implement the full operator shell, phase 32
  still treats the primary web shell as the completion surface and does not claim parity
  across every client.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The product MUST provide a coherent operator shell that is the primary
  surface for onboarding, approvals, readiness, inspection, and diagnostics for the
  personal-agent product.
- **FR-001a**: For phase 32, exactly one primary operator shell is required to claim
  completion, and that primary shell is the web surface; other client surfaces may remain
  partial or follow later without blocking this phase.
- **FR-002**: The operator shell MUST provide a first-run onboarding path that shows setup
  progress, unresolved prerequisites, and the next operator action needed to reach a
  successful first-use outcome.
- **FR-002a**: Onboarding completion for phase 32 MUST be defined by the minimum readiness
  set required for the selected first useful action, rather than requiring every known
  integration, connector, or capability to be configured before the shell is usable.
- **FR-003**: Onboarding progress MUST be derived from current daemon truth and remain
  resumable across sessions and daemon restart within the same environment.
- **FR-004**: The shell MUST expose readiness for required authentication, integrations,
  and capabilities in a way that distinguishes ready, missing configuration, degraded, and
  blocked states.
- **FR-005**: When a readiness item is not actionable, the shell MUST explain the blocking
  condition and the operator-visible next step rather than only reporting a generic
  failure.
- **FR-006**: Once onboarding prerequisites are satisfied, the shell MUST allow the
  operator to initiate one bounded first useful action, defined for phase 32 as a test
  query or test run, and inspect its outcome from the same operator surface.
- **FR-007**: The shell MUST provide an approval inbox that exposes pending and resolved
  approvals with request context, affected resource or side effect, timestamps, requester,
  and current resolution state.
- **FR-007a**: The approval inbox MUST allow the operator to approve or reject a pending
  approval directly from the shell and inspect the resulting decision state without using
  a raw daemon route.
- **FR-008**: Operators MUST be able to inspect schedules, workflows, deliveries, and
  recent background outcomes from the shell without reconstructing truth from raw daemon
  routes.
- **FR-009**: The shell MUST preserve linkage between related operator truths so an
  operator can move from a schedule to the related workflow or delivery outcome, and from a
  blocked execution to its approval or readiness cause.
- **FR-010**: Operator-visible history in the shell MUST be derived from daemon-owned
  records rather than client-invented summaries that can diverge from persisted truth.
- **FR-011**: The shell MUST provide health and diagnostic views for integrations,
  schedules, workflows, deliveries, and computer-use paths that identify whether the item
  is healthy, blocked, degraded, interrupted, failed, or awaiting approval.
- **FR-012**: Diagnostic surfaces MUST preserve separate truth for readiness state,
  execution state, approval state, and delivery state so operators can identify which
  plane failed.
- **FR-013**: The shell MUST provide truthful empty states for approvals, background work,
  and history when no relevant records exist, and MUST NOT imply missing data is hidden
  activity.
- **FR-014**: All operator shell surfaces MUST remain environment-scoped so test and live
  environments do not share onboarding progress, approvals, readiness, or execution
  history.
- **FR-014a**: For phase 32, the shell MUST make the current environment explicit on entry
  and during inspection, but it does not need to provide in-shell test/live environment
  switching to claim completion.
- **FR-015**: The operator shell MUST not require raw daemon route usage for basic setup,
  readiness inspection, approval inspection, schedule inspection, workflow inspection,
  delivery inspection, or failure diagnosis.
- **FR-015a**: When authoritative detail data is needed for those flows, the shell MAY
  reuse the existing daemon routes behind SDK-backed detail panels or sheets, but it MUST
  NOT require the operator to navigate to raw daemon endpoints to complete the phase 32
  workflow.
- **FR-016**: When a first useful action, background task, or delivery fails, the shell
  MUST preserve the failure outcome and point the operator back to the blocking readiness,
  approval, execution, or delivery truth instead of presenting an ambiguous generic error.
- **FR-017**: Phase 32 MUST stay scoped to single-operator onboarding and control. It MUST
  NOT require a marketing site, a collaborative multi-user admin console, or unrelated
  design-system expansion to claim completion.

### Key Entities *(include if feature involves data)*

- **Onboarding Progress Record**: The operator-visible summary of first-run setup status,
  including completed steps, unresolved prerequisites, blocking reasons, the minimum
  readiness set for the selected first useful action, and the next recommended action.
- **Readiness Item**: A specific authentication, integration, or capability dependency that
  the operator must inspect, configure, or verify before the product can perform expected
  personal-agent work.
- **Approval Inbox Item**: An operator-facing record of a pending or resolved approval
  request, including requester, requested action, affected resource or side effect, reason,
  timestamps, current decision status, and any operator-entered resolution outcome.
- **Operator Activity Record**: A durable summary of recent schedules, workflows,
  deliveries, and related background outcomes for one explicit environment that can be
  inspected from the shell without reassembling raw route responses.
- **Diagnostic Finding**: The operator-facing explanation of a blocked, degraded,
  interrupted, or failed state, including which operational plane is affected and what
  corrective action is required next.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Additive API, schema, event, config, and client-surface
  changes are expected for onboarding progress, readiness projections, approval inbox
  inspection, operator history, and diagnostic views. Existing daemon truth surfaces
  remain backward compatible.
- **Migration / Rollback**: Rollout is additive and should introduce operator-shell
  projections without removing existing raw inspection routes. Rollback is a revert of the
  new shell surfaces and projections while preserving any already-recorded approval,
  workflow, schedule, delivery, or onboarding audit truth.
- **Verification Strategy**: Required validation includes client coverage for critical
  onboarding, approval, readiness, history, and diagnostics flows; API-to-UI contract
  checks where shell surfaces project daemon truth; restart validation for resumable
  onboarding and durable operator history; representative failure-path coverage for
  readiness, approval-blocked execution, schedule failure, workflow failure, and delivery
  failure; timing checks against the stated local operator-shell response and refresh
  targets; and one manual onboarding acceptance path in `DOPE_ENV=test`.
- **Observability Impact**: Operators must be able to inspect onboarding progress, missing
  prerequisites, readiness state, approval status, background execution outcomes, delivery
  outcomes, and diagnostic findings as explicit operator-visible truth with enough context
  to identify the failing plane and next action.
- **Environment & Secrets**: Validation defaults to `DOPE_ENV=test`. Live connectors and
  privileged actions remain explicit operator choices, and the shell must avoid exposing
  secret material while still making readiness and failure causes understandable.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In manual `DOPE_ENV=test` validation, a first-time operator can complete the
  minimum onboarding path and reach one successful first-use outcome in under 10 minutes
  without using raw daemon routes or source-code inspection.
- **SC-001b**: In manual `DOPE_ENV=test` validation, onboarding can complete after the
  minimum readiness set for the selected first useful action is satisfied, while unrelated
  setup items remain visible as optional follow-up work.
- **SC-001a**: In manual `DOPE_ENV=test` validation, the first useful action completes as
  a bounded test query or test run and returns visible result and status feedback in the
  same shell session without requiring the operator to inspect logs.
- **SC-002**: In representative operator-shell validation, 100% of exercised pending
  approvals, schedules, workflows, and delivery outcomes are visible from shell surfaces
  with their current status and shell-resident linked inspection context.
- **SC-002a**: In representative operator-shell validation, a pending approval can be
  approved or rejected from the shell and the updated decision state is visible without
  using a raw daemon route.
- **SC-003**: In representative failure validation, operators can identify whether a
  blocked or failed case is caused by missing setup, degraded readiness, pending approval,
  execution failure, or delivery failure from shell surfaces alone.
- **SC-004**: After daemon restart in test validation, previously completed onboarding
  steps, approval records, and recent operator activity remain inspectable and do not
  reset to a misleading first-run state.
- **SC-005**: In manual troubleshooting validation, an operator can determine why a
  scheduled or background action did not run or did not deliver in under 5 minutes without
  reading raw logs first.
- **SC-006**: In empty-state validation, the shell reports no pending approvals and no
  recent background work truthfully, without presenting stale records as current activity.
- **SC-006a**: In environment-scope validation, the shell always shows the active
  environment explicitly and never mixes test and live onboarding, approval, or execution
  truth in one view.
- **SC-006b**: In release-readiness validation for phase 32, the primary web shell covers
  onboarding, approval handling, readiness inspection, operator history, and diagnostics
  without requiring equivalent TUI parity.

## Assumptions

- Phase 32 is a single-operator product surface and does not introduce collaborative
  administration, multi-user policy management, or shared team dashboards.
- The shell consumes and projects daemon-owned truth rather than replacing or redefining
  the underlying schedule, workflow, approval, delivery, or capability contracts.
- The first useful action is a bounded operator-facing action in the same environment that
  demonstrates the product is usable after onboarding; for phase 32, that action is a
  test query or test run that returns visible result and status feedback in the same shell
  without requiring broad product expansion beyond the shell and existing daemon
  capabilities.
- Onboarding does not require every possible integration, connector, or capability to be
  ready; it only requires the minimum readiness set needed for the selected first useful
  action, with remaining items preserved as follow-up setup.
- Test and live environments remain operationally distinct, and phase 32 does not merge
  their setup state, history, or approvals into one combined operator view; switching
  environments may happen outside the shell entry point or in a later roadmap phase.
- Live connectors, remote access patterns, and richer onboarding polish may evolve later,
  but phase 32 only needs the minimum trustworthy shell required to configure, inspect, and
  debug the personal-agent product.
- Existing non-web client surfaces may continue to exist, but phase 32 does not require
  them to provide equivalent operator-shell coverage before planning can proceed.
