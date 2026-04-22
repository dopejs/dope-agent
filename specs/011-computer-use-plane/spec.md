# Feature Specification: Computer-Use Capability Plane

**Feature Branch**: `[012-computer-use-plane]`  
**Created**: 2026-04-22  
**Status**: Draft  
**Input**: User description: "$speckit-specify 结合 docs/specs/011-use-computer-capability-plane.md 完成 phase 26 的工作"

## Clarifications

### Session 2026-04-22

- Q: phase 26 的 browser-first computer-use MVP 应该支持哪些核心动作？ → A: 支持会话启动/结束、导航、返回/前进、等待、截图/快照、点击、输入、选择、下载。
- Q: phase 26 的 browser session 生命周期应该怎么界定？ → A: 同一 run/workflow 内可复用 session，但不跨 run/schedule 持久复用。
- Q: phase 26 默认哪些 browser actions 需要审批？ → A: 只有高风险动作需要审批：会改变外部状态、提交输入、触发下载、或离开当前受信页面范围的动作。
- Q: phase 26 遇到 target mismatch 时应该怎么处理？ → A: 立即失败并保留证据；必须重新检查后才能继续。
- Q: phase 26 是否支持多标签/多窗口 browser 会话？ → A: 仅支持单标签/单活动页面；新窗口或新标签视为不支持或失败。

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Inspect And Approve Browser Actions (Priority: P1)

As an operator, I need to inspect what browser page or target the agent is about to act
on before a high-risk computer-use action executes so I can approve or deny the action
with enough context to trust the outcome.

**Why this priority**: Phase 26 is primarily a safety and operator-trust problem. If the
daemon cannot show what page or target is about to change, browser automation becomes an
opaque side path rather than a production capability plane.

**Independent Test**: Start a browser-first computer-use session that reaches a
high-risk action, inspect the pending action and current page context, deny it once and
approve it once, and confirm the approved path records linked evidence history without
reading raw logs.

**Acceptance Scenarios**:

1. **Given** an operator has a computer-use step ready to perform a high-risk action,
   **When** the operator inspects the pending action, **Then** the system shows the
   current browser target, the action being requested, and enough page context to decide
   whether to allow it.
2. **Given** a pending high-risk computer-use action, **When** the operator denies it,
   **Then** the action does not execute and the denial remains visible as an explicit
   operator-visible outcome.
3. **Given** a pending high-risk computer-use action, **When** the operator approves it,
   **Then** the system executes the action through the normal runtime plane and records
   linked session and evidence history for the outcome.

---

### User Story 2 - Run Computer Use Inside Normal Workflows (Priority: P2)

As an operator, I need computer-use steps to run inside the existing run and workflow
surfaces so browser automation can be orchestrated beside other capabilities without
creating a hidden executor.

**Why this priority**: The roadmap is only closed if computer use remains subordinate to
the current runtime truth, approval boundaries, and audit model instead of becoming a
special-case local tool path.

**Independent Test**: Execute at least one `DOPE_ENV=test` workflow that combines a
computer-use step with another capability family, then confirm every computer-use action
appears as normal runtime truth with session linkage and operator-visible evidence.

**Acceptance Scenarios**:

1. **Given** a workflow includes a browser-first computer-use step and at least one other
   capability step, **When** the workflow is planned and executed, **Then** the
   computer-use step remains part of the normal workflow truth rather than an out-of-band
   local action.
2. **Given** a computer-use step completes during a normal run or workflow, **When** the
   operator inspects the resulting runtime records, **Then** the system links the step
   and tool-call truth to the relevant browser session, page context, and evidence
   artifacts.
3. **Given** a schedule or operator-launched run triggers work that includes
   computer-use behavior, **When** the work executes, **Then** the system preserves the
   same policy, approval, and audit semantics that apply to other runtime capabilities.

---

### User Story 3 - Diagnose Outcomes With Evidence (Priority: P3)

As an operator, I need screenshots, page snapshots, downloads, and distinct failure
reasons so I can understand what happened during computer use after the fact.

**Why this priority**: Browser automation is not trustworthy if operators must infer
page state and failure class from logs alone. Artifact-backed truth is required for
debuggability and auditability.

**Independent Test**: Exercise successful navigation, policy denial, unavailable
consumer, navigation failure, and target mismatch paths, then verify that each outcome is
distinguishable and backed by operator-visible evidence where applicable.

**Acceptance Scenarios**:

1. **Given** a computer-use action succeeds and produces page evidence, **When** the
   operator inspects the result, **Then** the system exposes screenshots, page snapshots,
   downloads, or equivalent evidence as first-class artifacts linked to the action.
2. **Given** a computer-use attempt fails before completing the intended action,
   **When** the operator inspects the failure, **Then** the system distinguishes whether
   the cause was policy denial, unavailable consumer, navigation failure, or target
   mismatch.
3. **Given** a browser target changes between inspection and execution, **When** the
   action can no longer safely match the intended target, **Then** the system blocks the
   action, records target mismatch explicitly, and preserves the latest available page
   evidence for operator review.

### Edge Cases

- If the page or element no longer matches the approved target at execution time, the
  requested action fails immediately, preserves the latest evidence, and requires renewed
  inspection before any follow-up action.
- If the browser consumer becomes unavailable after a workflow or run is already planned,
  the affected action or session records an explicit unavailable-consumer or interrupted
  outcome without falling back to unmanaged local behavior.
- If daemon restart interrupts a session while an action is pending or in flight, the
  session and action remain inspectable and transition to explicit interrupted or
  unavailable states instead of silently resuming.
- If evidence capture completes only partially because navigation or download fails
  mid-step, the system preserves any successfully captured evidence, records failed
  captures as visible metadata, and keeps the action outcome distinguishable from a fully
  successful capture.
- If multiple computer-use actions reuse the same browser session within a longer
  workflow, the system preserves ordered action history, current page context, and
  trusted scope revisions within that single run or workflow boundary.
- Phase 26 supports only one active browser page per session; attempts to open or depend
  on additional tabs or windows fail explicitly rather than creating unmanaged parallel
  page state.
- If an operator requests generalized desktop automation during the browser-first phase,
  the request fails explicitly rather than being approximated through the browser surface
  or an unmanaged local-tool fallback.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST expose a first-class computer-use capability surface for
  browser-first automation rather than treating browser control as an opaque local tool.
- **FR-002**: The system MUST provide operator-visible session or equivalent lifecycle
  resources that show the current browser target, session state, and recent action
  history.
- **FR-002a**: Phase 26 browser sessions MAY be reused across multiple computer-use
  actions within the same run or workflow, but they MUST NOT be persistently reused across
  separate runs or schedule dispatches.
- **FR-003**: Computer-use executions MUST create normal runtime steps and tool calls
  with additive browser session or page linkage.
- **FR-004**: The system MUST let operators inspect the current browser target and the
  requested action before high-risk computer-use actions execute.
- **FR-005**: High-risk computer-use actions MUST preserve the existing approval,
  sandbox, provenance, and audit guarantees attached to concrete runtime actions.
- **FR-005a**: By default, high-risk computer-use actions include actions that change
  external state, submit operator-provided input, trigger downloads, or navigate beyond
  the currently trusted page scope; lower-risk read-only or in-page inspection actions do
  not require approval unless policy elevates them.
- **FR-006**: The system MUST project screenshots, page snapshots, downloads, and other
  relevant computer-use evidence as first-class operator-visible artifacts or equivalent
  outputs.
- **FR-007**: The system MUST support a controlled navigation and action model that keeps
  the current browser target inspectable before and after each action.
- **FR-007a**: Phase 26 browser-first computer use MUST support session start and end,
  navigation, back or forward navigation, waiting, screenshots or page snapshots, click,
  text input, selection, and downloads as the core operator-visible action set.
- **FR-007b**: Phase 26 browser sessions MUST support exactly one active page at a time;
  actions that require additional tabs or windows MUST fail explicitly unless a later
  phase expands the browser surface.
- **FR-008**: The system MUST allow computer-use steps to participate in existing run and
  workflow execution without bypassing policy or audit boundaries.
- **FR-009**: The system MUST distinguish at least these computer-use failure classes:
  policy denial, unavailable consumer, navigation failure, and target mismatch.
- **FR-009a**: On target mismatch, the system MUST fail the requested action immediately,
  preserve the latest available evidence, and require renewed operator-visible inspection
  before any follow-up action can proceed.
- **FR-010**: The system MUST preserve explicit action outcome history so operators can
  tell whether an action was planned, approved, denied, attempted, interrupted, or
  completed.
- **FR-011**: Completed computer-use history and evidence MUST remain inspectable across
  daemon restart within the same environment.
- **FR-012**: If the daemon restarts while a computer-use session or action is pending or
  in flight, the system MUST record an explicit interrupted or unavailable outcome rather
  than silently resuming unnoticed work.
- **FR-012a**: Restart or later work MUST NOT restore a prior browser session into a
  different run or schedule dispatch; any new work starts from a new session boundary.
- **FR-013**: Computer-use resources, artifacts, and audit history MUST remain
  environment-scoped.
- **FR-014**: Existing MCP, skill, and non-computer-use runtime execution paths MUST
  continue to behave as they do today unless the operator explicitly uses
  computer-use-capable work.
- **FR-015**: Phase 26 MUST be browser-first; requests that require generalized desktop
  automation beyond the supported browser surface MUST fail explicitly rather than falling
  back to unmanaged local behavior.

### Key Entities *(include if feature involves data)*

- **Computer-Use Session**: The operator-visible record for one browser interaction
  context, including its current target, lifecycle state, related action history, and the
  run or workflow boundary within which it may be reused.
- **Computer-Use Action**: One concrete navigation or interaction request, including the
  intended target, approval state when applicable, execution outcome, and linkage to
  runtime truth.
- **Page Evidence Artifact**: An operator-visible artifact such as a screenshot, page
  snapshot, or downloaded file that explains what a computer-use step saw or changed.
- **Target Match Context**: The inspectable description of what page element, page state,
  or browser target an action expected to operate on and whether that match remained valid
  at execution time.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Additive API, schema, event, artifact, and storage surface
  changes are required for computer-use sessions, actions, evidence artifacts, and
  runtime linkage. Existing MCP and skill execution paths remain unchanged.
- **Migration / Rollback**: Additive computer-use resource and artifact persistence is
  required. Rollback is a revert of computer-use-specific resources, routes, projections,
  and operator surfaces while preserving already-recorded runtime truth and captured
  artifacts as historical evidence.
- **Verification Strategy**: Required validation includes targeted capability and API
  tests for session lifecycle, approval gating, workflow integration, and failure-class
  handling; contract coverage for new computer-use resources or artifacts; restart
  coverage for persisted history and interrupted work; and one manual browser-based
  `DOPE_ENV=test` verification path.
- **Observability Impact**: Operator-visible surfaces must explain pending approvals,
  current browser target, action history, evidence artifacts, interruption outcomes, and
  failure-class distinctions without requiring raw-log reconstruction.
- **Environment & Secrets**: Validation stays in `DOPE_ENV=test` by default. Browser
  consumers and any downloaded or captured artifacts remain environment-scoped and must
  follow existing secret-handling and redaction rules. Production connectors or general
  desktop access are not required for this phase.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can inspect and approve or deny a pending high-risk browser
  action within 3 minutes using operator-visible surfaces only.
- **SC-002**: In validated successful browser-first runs, 100% of completed high-risk
  computer-use actions record operator-visible pre-action context, final outcome, and at
  least one linked evidence artifact without requiring raw log inspection.
- **SC-003**: At least one `DOPE_ENV=test` workflow combining computer use with another
  capability family completes end-to-end without any out-of-band execution path.
- **SC-004**: In automated verification, 100% of exercised policy denial, unavailable
  consumer, navigation failure, and target mismatch outcomes are distinguishable from one
  another through operator-visible history.
- **SC-005**: After daemon restart in test verification, previously completed
  computer-use history and artifacts remain inspectable, and any in-flight session or
  action ends in an explicit interrupted or unavailable state.

## Assumptions

- Phase 26 is browser-first and does not close generalized desktop automation, mobile
  automation, or remote desktop work.
- Phase 26 browser-first MVP includes session start and end, navigation, back or forward,
  waiting, screenshots or page snapshots, click, text input, selection, and downloads as
  the supported core action surface.
- Phase 26 supports a single active page per session and does not close multi-tab or
  multi-window browser automation.
- Browser sessions may be reused within one run or workflow for multi-step work, but they
  do not persist across separate runs or schedule-owned launches.
- The existing run and workflow surfaces remain the authoritative execution owners for
  computer-use work; this phase adds a capability plane, not a second executor.
- Screenshots, page snapshots, downloads, and similar browser evidence are first-class
  operator-visible artifacts, although not every action must emit every artifact type.
- Existing approval, policy, provenance, and sandbox semantics continue to apply at the
  concrete action boundary.
- The default approval model is risk-based: actions that change external state, submit
  input, trigger downloads, or leave the currently trusted page scope require approval,
  while lower-risk read-only or in-page inspection actions do not unless policy elevates
  them.
- Existing schedule and workflow planes may launch work that includes computer-use steps
  without requiring a separate scheduling or orchestration subsystem for this phase.
- Unsupported browser targets or unsupported desktop-style requests fail explicitly rather
  than falling back to unmanaged local tooling.
