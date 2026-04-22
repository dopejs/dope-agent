# Feature Specification: Scheduled Tasks Wakeups

**Feature Branch**: `[011-scheduled-tasks-wakeups]`  
**Created**: 2026-04-22  
**Status**: Draft  
**Input**: User description: "$speckit-specify 结合 docs/specs/010-scheduled-tasks-and-wakeups.md 完成 phase 25 的工作"

## Clarifications

### Session 2026-04-22

- Q: daemon 重启后遇到 overdue schedules，phase 25 应该采用哪种 catch-up 语义？ → A: 仅补发最近一次 overdue trigger，其余记为可见 missed/skipped history。
- Q: recurring schedule 的下一次触发到来时，如果上一次启动的 execution 还没结束，phase 25 应该采用什么 overlap 语义？ → A: 不并发重入；若前一次 execution 仍在运行，则这次触发记为可见 skipped/blocked history。
- Q: recurring schedule 连续出现 dispatch-side failure 时，phase 25 的 retry/backoff 语义应该是什么？ → A: 对 dispatch-side failure 做有界自动重试和 backoff，并暴露 retry budget、next retry time、以及最终 exhausted 状态。
- Q: recurring schedule 的时间语义在 phase 25 应该怎么定义？ → A: 每个 recurring schedule 绑定显式 IANA timezone，并按该 timezone 计算下一次 fire time；operator-visible surfaces 同时展示原始 timezone 和归一化时间。
- Q: schedule 的 launch target 在 phase 25 应该采用哪种绑定语义？ → A: 持久化稳定 target reference，dispatch 时按当前可见定义解析；若 target 不存在或不再可执行，则记为 dispatch-side failure。

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Schedule Future Work (Priority: P1)

An operator creates a one-time scheduled task that will launch a normal run or workflow later, then inspects it before the trigger fires.

**Why this priority**: Without durable future work, the agent is still reactive. This is the minimum slice that makes the daemon behave like a personal agent instead of a chat-only runtime.

**Independent Test**: Can be fully tested by creating a one-time scheduled task, verifying that it remains pending before its due time, and confirming that it launches exactly one normal run or workflow when due.

**Acceptance Scenarios**:

1. **Given** an authenticated operator and an existing runnable goal or workflow plan, **When** the operator creates a one-time scheduled task for a future time, **Then** the system stores an inspectable schedule resource with its trigger time, launch target, current state, and no execution yet started.
2. **Given** a stored one-time scheduled task that has not yet reached its trigger time, **When** the operator inspects it, **Then** the system shows that the task is still pending and has not created any runtime execution records.
3. **Given** a stored one-time scheduled task at or past its trigger time, **When** the daemon dispatches it, **Then** the system creates a normal run or workflow execution and links that execution back to the schedule history.

---

### User Story 2 - Manage Recurring Schedules (Priority: P2)

An operator creates a recurring schedule, pauses it, resumes it, and reviews the history of previous dispatches and failures without losing visibility into what will happen next.

**Why this priority**: A personal agent becomes useful when routines can be delegated. Recurring schedules make the trigger plane durable and operational rather than a one-shot helper.

**Independent Test**: Can be fully tested by creating a recurring schedule, observing at least one dispatch, pausing it before the next due time, confirming no dispatch happens while paused, then resuming it and confirming future dispatch resumes with preserved history.

**Acceptance Scenarios**:

1. **Given** an authenticated operator, **When** the operator creates a recurring schedule, **Then** the system stores the recurrence rule, next due time, current state, and empty execution history.
2. **Given** an active recurring schedule, **When** the operator pauses it before the next trigger time, **Then** the schedule remains visible, keeps its prior history, and does not dispatch while paused.
3. **Given** a paused recurring schedule, **When** the operator resumes it, **Then** the system calculates a new next due time, preserves prior history, and allows future dispatches to continue.

---

### User Story 3 - Understand Trigger Failures (Priority: P3)

An operator can distinguish whether a scheduled task failed to dispatch, launched but failed during execution, was cancelled, or was skipped because it was paused or otherwise ineligible.

**Why this priority**: Background automation cannot be trusted if operators have to reconstruct failures from raw logs. Failure truth must be explicit and operationally useful.

**Independent Test**: Can be fully tested by exercising one schedule that dispatches successfully, one that encounters a dispatch-side failure, and one whose launched execution fails, then verifying that the resulting states and history remain distinct.

**Acceptance Scenarios**:

1. **Given** a scheduled task whose trigger time arrives but whose launch target cannot be dispatched, **When** the system records the outcome, **Then** the schedule history marks the attempt as a trigger or dispatch failure without inventing a runtime execution that never started.
2. **Given** a scheduled task that successfully launches a run or workflow and that downstream execution later fails, **When** the operator inspects schedule history, **Then** the history shows that dispatch succeeded and the linked execution failed separately.
3. **Given** a scheduled task that is cancelled or paused before dispatch, **When** the due time passes, **Then** the system does not launch work and preserves a visible reason for why no dispatch occurred.

### Edge Cases

- What happens when the daemon restarts while one or more schedules are overdue?
- What happens when multiple schedules are due at nearly the same time?
- What happens when a recurring schedule was paused during one or more expected trigger times?
- What happens when a launch target referenced by a schedule is no longer valid at dispatch time?
- What happens when a recurring schedule experiences repeated dispatch failures?
- After daemon restart, only the most recent overdue trigger is eligible for catch-up dispatch; older missed intervals remain visible as missed or skipped history rather than being replayed.
- If a recurring schedule reaches its next due time while its prior launched execution is still running, the new trigger is not dispatched concurrently and must remain visible as skipped or blocked history.
- Repeated dispatch-side failures use bounded automatic retry with backoff, and operators can inspect retry budget consumption, next retry time, and exhausted outcomes.
- Recurring schedules are evaluated against an explicit IANA timezone stored on the schedule rather than relying on the daemon host timezone.
- Schedule launch targets are stored as stable references and resolved at dispatch time; if the referenced target is no longer present or executable, the attempt is recorded as a dispatch-side failure rather than using a stale embedded snapshot.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST expose first-class schedule resources for one-time and recurring scheduled tasks.
- **FR-002**: The system MUST allow an operator to create, inspect, list, pause, resume, and cancel schedule resources.
- **FR-003**: Each schedule resource MUST include its launch target, trigger definition, current state, next due time when applicable, and dispatch history.
- **FR-003a**: Each recurring schedule resource MUST include its explicit IANA timezone, and operator-visible surfaces MUST show both the stored timezone and the derived next due time.
- **FR-003b**: Each schedule resource MUST persist a stable launch target reference rather than an opaque embedded execution snapshot, and operator-visible surfaces MUST expose that reference.
- **FR-004**: The system MUST launch scheduled work by creating normal runs or workflows through the existing runtime plane rather than a separate hidden executor.
- **FR-005**: The system MUST preserve a durable linkage between a schedule dispatch attempt and any run or workflow created from that dispatch.
- **FR-006**: The system MUST distinguish at least these outcomes in schedule state or history: pending, dispatched successfully, paused, cancelled, trigger or dispatch failed, and launched execution failed.
- **FR-007**: The system MUST preserve schedule resources and their prior dispatch history across daemon restart within the same environment.
- **FR-008**: When the daemon restarts and finds overdue schedules, the system MUST dispatch at most the most recent overdue trigger and MUST record older missed intervals as visible missed or skipped history rather than silently replaying an unbounded backlog.
- **FR-009**: The system MUST keep schedule routes, persisted state, and audit history environment-scoped.
- **FR-010**: Schedule execution MUST NOT bypass existing run, workflow, step, tool-call, approval, or policy semantics.
- **FR-011**: The system MUST record dispatch-side failures separately from downstream runtime or workflow failures.
- **FR-011a**: If a schedule's referenced launch target cannot be resolved or is no longer executable at dispatch time, the system MUST record that outcome as a dispatch-side failure without inventing a downstream execution.
- **FR-012**: The system MUST support retry or backoff handling for repeated dispatch-side failures and make that handling operator-visible.
- **FR-012a**: For dispatch-side failures, phase 25 MUST use bounded automatic retry with backoff and MUST expose retry budget consumption, next retry time, and exhausted state to operators.
- **FR-013**: The system MUST allow operators to inspect the next planned fire time for active schedules and the most recent outcome for previously attempted dispatches.
- **FR-013a**: Recurring schedule evaluation MUST use the schedule's stored IANA timezone rather than implicit daemon-local timezone semantics.
- **FR-014**: The system MUST produce operator-visible audit signals for schedule creation, state transitions, and dispatch attempts.
- **FR-015**: The system MUST NOT dispatch overlapping concurrent executions for the same schedule in phase 25; if a trigger arrives while a prior linked execution is still active, the new trigger MUST be recorded as visible skipped or blocked history.

### Key Entities *(include if feature involves data)*

- **Schedule**: A durable operator-managed trigger resource that defines when future work should be launched, which stable launch target reference should be resolved at dispatch time, what state it is in, and what its next due time is.
- **Schedule Trigger**: The timing definition for a schedule, including whether it is one-time or recurring, the timezone used for recurring evaluation when applicable, and the next due moment derived from that definition.
- **Schedule Dispatch Attempt**: A historical record of an individual trigger evaluation or launch attempt, including outcome, timestamps, failure reason when relevant, and any linked runtime execution.
- **Missed Or Skipped Interval Record**: A visible history record representing an expected trigger time that was not dispatched because it fell outside the bounded catch-up policy or because the schedule was otherwise ineligible.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Additive API, schema, event, and storage surface changes for schedule resources, schedule history, and schedule-linked runtime projections.
- **Migration / Rollback**: Additive persistence for schedules and dispatch history is required. Rollback is a revert of schedule-specific resources and routes while preserving already-created runs and workflows as historical execution truth.
- **Verification Strategy**: Required validation includes targeted scheduler tests, API and contract coverage for schedule resources and events, restart recovery coverage for persisted schedules including bounded overdue catch-up behavior, and one manual `DOPE_ENV=test` recurring schedule verification.
- **Observability Impact**: Operator-visible events, logs, and history must show schedule creation, pause or resume, cancellation, dispatch attempts, dispatch failures, successful launches, and links to downstream execution truth.
- **Environment & Secrets**: Schedule behavior must remain isolated per environment. Test-environment verification must not require production connectors or production secrets unless explicitly configured for a live integration scenario.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can create and inspect a one-time scheduled task in under 2 minutes without using raw database access or log inspection.
- **SC-002**: In manual `DOPE_ENV=test` verification, a created one-time schedule launches exactly one normal run or workflow at its due time and exposes that linkage through operator-visible surfaces.
- **SC-003**: In automated verification, paused recurring schedules produce zero unintended dispatches while paused across repeated due-time checks.
- **SC-004**: In automated verification, 100% of exercised schedule outcomes are distinguishable as dispatch failure, downstream execution failure, successful dispatch, paused, or cancelled without consulting raw logs.
- **SC-004a**: In automated verification, repeated dispatch-side failures expose retry budget progression, next retry time, and a terminal exhausted outcome through operator-visible schedule surfaces.
- **SC-005**: After daemon restart in test verification, previously created future schedules remain inspectable and preserve their history, while overdue schedules dispatch at most the most recent overdue trigger and preserve older missed intervals as visible missed or skipped history.

## Assumptions

- Operators are scheduling work that already has a valid run or workflow target shape available through the existing runtime or orchestration surfaces.
- Phase 25 focuses on durable daemon-owned schedules, not on natural-language routine planning or cross-device notification infrastructure.
- Bounded catch-up means only the most recent overdue trigger is eligible for dispatch after restart; older overdue intervals remain visible as missed or skipped history.
- Phase 25 uses a non-reentrant schedule model: a recurring schedule does not launch a new execution while its previous linked execution is still active, and the overlapping trigger is surfaced as skipped or blocked history.
- Dispatch-side retry behavior is bounded and operator-visible; retry exhaustion is represented explicitly rather than hidden behind repeated silent failures.
- Recurring schedules carry an explicit IANA timezone and are evaluated against that timezone instead of daemon-host local time.
- Schedule launch targets are resolved from stored stable references at dispatch time, so future dispatches reflect the currently visible target definition and fail explicitly if that target is no longer valid.
- The first slice may start from operator-created schedules before expanding to webhook or external-trigger sources in later roadmaps.
