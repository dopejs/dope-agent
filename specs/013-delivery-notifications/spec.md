# Feature Specification: Delivery And Notifications

**Feature Branch**: `[013-delivery-notifications]`  
**Created**: 2026-04-22  
**Status**: Draft  
**Input**: User description: "结合 docs/specs/013-delivery-and-notifications.md 完成 phase 28 的工作"

## Clarifications

### Session 2026-04-22

- Q: How should one result route when multiple delivery targets are active in the same environment? → A: Route each result to exactly one preferred delivery target in its environment.
- Q: How much retry history should operators be able to inspect for delivery? → A: Preserve each delivery attempt and its outcome in operator-visible history.
- Q: Which results may be grouped into digests when summary delivery is configured? → A: Routine successes may be digested, but failures and urgent results must deliver immediately.
- Q: After retries are exhausted for the chosen target, should the same result automatically fail over to a secondary target? → A: No automatic failover; keep the result bound to its chosen target and record terminal delivery failure there.
- Q: At what scope should delivery preferences apply? → A: Use user-level defaults with optional integration-specific overrides.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Receive Background Results Reliably (Priority: P1)

As a user, I need scheduled work, workflows, and integration-backed actions to send me
their result or failure summary even when I did not start them from an active chat
session, so the agent remains useful when it works in the background.

**Why this priority**: Roadmap 28 only closes if the daemon can return background work
to the user through a durable delivery plane. Without routed background delivery,
schedules and autonomous work remain operationally incomplete.

**Independent Test**: Configure at least one active delivery target, run a scheduled or
workflow-originated task without an active foreground session, and confirm the user
receives a success or failure result through the configured target.

**Acceptance Scenarios**:

1. **Given** a scheduled task finishes successfully and the user has an active delivery
   target, **When** the task completes without an active foreground session, **Then** the
   user receives a routed result notification through the configured delivery target.
2. **Given** a workflow fails after starting in the background, **When** the failure is
   finalized, **Then** the user receives a failure summary through the configured
   delivery target without needing to poll operator surfaces manually.
3. **Given** multiple eligible delivery targets exist for the same user in one
   environment, **When** background work completes, **Then** the system routes the result
   to exactly one preferred delivery target according to the user's active delivery
   preference rather than relying on the original request channel.

---

### User Story 2 - Inspect Delivery Truth Separately From Execution Truth (Priority: P2)

As an operator, I need to tell whether work succeeded, retried, was suppressed, or
failed to deliver so I can troubleshoot notification problems without confusing them
with execution failures.

**Why this priority**: The roadmap explicitly requires delivery truth to remain distinct
from execution truth and from integration readiness truth. If those planes collapse into
one status, operators cannot trust the system under failure.

**Independent Test**: Trigger representative successful work and then cause delivery to
retry, suppress, or fail, and confirm the operator can inspect execution outcome and
delivery outcome separately from the same operator-visible surfaces.

**Acceptance Scenarios**:

1. **Given** background work succeeds but the configured delivery target is temporarily
   unavailable, **When** the notification attempt is processed, **Then** execution
   remains recorded as successful and delivery is recorded separately as retrying or
   failed.
2. **Given** a notification is intentionally suppressed by delivery policy or target
   configuration, **When** the result is finalized, **Then** the operator can see that
   work completed and that delivery was suppressed rather than missing.
3. **Given** an integration-backed action completes while the related integration is
   healthy, **When** delivery later fails, **Then** the operator can distinguish the
   successful action, the healthy integration state at execution time, and the later
   delivery failure as separate truths.

---

### User Story 3 - Reuse Delivery Targets And Summary Preferences (Priority: P3)

As a user or operator, I need reusable delivery targets and preferences that can be
attached to schedules, workflows, integration outcomes, and later digests, so delivery
behavior stays consistent instead of being redefined per feature.

**Why this priority**: The delivery plane must outlive one result-notification path.
Reusable targets and summary preferences are what let later calendar, mail, and reminder
features build on the same delivery contract.

**Independent Test**: Configure delivery targets and preferences once, attach them to
multiple background result sources, and confirm those sources can reuse the same routing
and summary settings without redefining delivery behavior.

**Acceptance Scenarios**:

1. **Given** a user has configured one or more delivery targets and a preferred route,
   **When** a schedule and a workflow both emit results, **Then** both can reuse the
   same user-level delivery defaults without source-specific setup unless an
   integration-specific override applies.
2. **Given** a digest or summary window is configured for background results, **When**
   multiple routine-success results are produced during that window, **Then** the system
   can deliver a summary through the configured delivery target instead of requiring one
   immediate message per result.
3. **Given** a delivery target is disabled or changed for one environment, **When**
   later background work emits results in another environment, **Then** the system keeps
   delivery routing environment-scoped and does not reuse the disabled target implicitly.

### Edge Cases

- If work completes successfully after all delivery targets for that environment are
  disabled or unavailable, the system records delivery as unsent, suppressed, or failed
  explicitly rather than silently dropping the result.
- If delivery to a target times out or is rejected after the result is finalized, the
  system preserves the execution outcome and records retry or failure history separately.
- If delivery retries multiple times before succeeding or failing, operators can inspect
  each attempt rather than only a collapsed final state.
- If retries are exhausted for the chosen target, the system records terminal delivery
  failure on that target rather than automatically rerouting the same result to a
  secondary target.
- If the same background result is replayed or retried, the system avoids presenting
  duplicate user-visible notifications as independent successful outcomes without
  inspectable justification.
- If a workflow emits both urgent failures and routine summaries, the system applies the
  configured delivery preference for each category, delivering failures and urgent
  results immediately without collapsing them into one undifferentiated message stream.
- If a digest window closes with no eligible results, the system does not invent an
  empty success notification unless the configured summary policy explicitly requires it.
- If a delivery target is removed after a schedule or workflow is created but before it
  finishes, the system uses current environment-scoped delivery truth and records why the
  result could not be delivered.
- If multiple delivery targets are configured for the same environment, each background
  result still selects exactly one preferred target rather than broadcasting by default.
- If an integration-specific delivery override exists, the system applies that override
  only to outcomes from that integration and leaves the user's default routing rules in
  place for other background work.
- If integration-backed work succeeds while delivery later fails, operators can still
  inspect successful execution truth and delivery failure truth independently.
- Phase 28 does not add a mobile push stack, social feed design, or marketing messaging;
  requests for those behaviors remain out of scope.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST support delivery targets that are separate from the
  immediate request channel so background work can notify users without an active
  foreground conversation.
- **FR-002**: The system MUST allow scheduled work, workflow-originated work, and
  integration-backed outcomes to emit routed result notifications through the shared
  delivery plane.
- **FR-003**: The system MUST preserve delivery outcome truth separately from execution
  outcome truth so successful work, failed work, and failed delivery are not conflated.
- **FR-004**: The system MUST project delivery outcomes using explicit operator-visible
  states that distinguish successful delivery, retry in progress, intentional
  suppression, and terminal failure.
- **FR-005**: The system MUST preserve delivery truth separately from integration
  readiness truth so a healthy or unhealthy integration does not overwrite notification
  outcome history.
- **FR-006**: Users or operators MUST be able to configure and inspect active delivery
  targets and preferences per environment.
- **FR-006c**: Delivery preferences MUST support user-level defaults with optional
  integration-specific overrides so integration-backed outcomes can use specialized
  routing without redefining the base delivery model.
- **FR-006a**: When multiple delivery targets are active in the same environment, each
  routed result MUST select exactly one preferred delivery target rather than fan out by
  default.
- **FR-006b**: Once a result is bound to its chosen delivery target, exhausted retries
  MUST end in terminal failure on that target rather than automatic failover to another
  target unless a later phase adds an explicit policy for that behavior.
- **FR-007**: Delivery targets MUST be reusable across schedules, workflows,
  integration-triggered outcomes, and later summary or digest flows without redefining
  the delivery resource model for each source.
- **FR-008**: The system MUST support retry behavior for transient delivery problems and
  MUST expose operator-visible per-attempt history and current retry state to operators.
- **FR-009**: The system MUST support intentional delivery suppression and MUST surface
  the suppression reason so operators can distinguish policy or preference decisions from
  failures.
- **FR-010**: The system MUST retain durable operator-visible history for routed result
  delivery attempts and final outcomes within the active environment.
- **FR-011**: The system MUST support summary or digest delivery scaffolding so multiple
  routine-success background outcomes can be grouped into a user-visible summary when
  configured, while failures and urgent results continue to deliver immediately.
- **FR-012**: Existing foreground chat reply behavior MUST remain valid and MUST NOT be
  redefined as the only delivery mechanism for background work.
- **FR-013**: Delivery configuration and outcome inspection MUST remain operator-visible
  without requiring raw connector logs or source-specific debugging surfaces.
- **FR-014**: Phase 28 MUST stay focused on the shared delivery plane and MUST NOT depend
  on mobile push infrastructure, social feed design, or domain-specific calendar, mail,
  or reminder behavior to claim completion.

### Key Entities *(include if feature involves data)*

- **Delivery Target**: A durable destination that can receive routed results, alerts, or
  summaries for a user within one environment, independent of the channel that started
  the work.
- **Delivery Preference**: The environment-scoped rules that determine which delivery
  targets are active, which single target is preferred for a given result class, when
  summaries, immediate delivery, or suppressions apply, and whether an integration uses
  the user default or an integration-specific override.
- **Delivery Outcome Record**: The operator-visible history describing whether a routed
  result was delivered, retried, suppressed, or failed, including each delivery attempt,
  the reason for that attempt's outcome, the bound delivery target, and the final state.
- **Summary Window**: A user-visible grouping period that can collect multiple eligible
  routine-success background outcomes into one delivered digest or summary while urgent
  and failed outcomes continue to deliver immediately.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Additive API, schema, event, config, and storage surface
  changes are expected for delivery targets, delivery preferences, routed result history,
  and summary scaffolding. Existing foreground reply behavior remains backward
  compatible.
- **Migration / Rollback**: Additive delivery resources and history are required.
  Rollback is a revert of delivery-target, routing, and projection changes while
  preserving already-recorded delivery history as read-only audit truth where needed.
- **Verification Strategy**: Required validation includes targeted coverage for delivery
  target configuration, background result routing, retry and suppression behavior,
  separation of execution truth from delivery truth, durable history across restart,
  contract coverage for delivery-facing resources or events, and one manual
  `DOPE_ENV=test` notification or summary flow.
- **Observability Impact**: Operators must be able to inspect active delivery targets,
  environment-scoped preferences, retry state, suppression reasons, failed-delivery
  causes, and the relationship between execution outcome and delivery outcome without
  depending on raw channel logs.
- **Environment & Secrets**: Work defaults to `DOPE_ENV=test`. Delivery behavior must
  remain environment-scoped. Live user-facing connectors are optional for initial
  validation, and any credentials used for delivery targets remain operator-owned and
  redacted from operator-visible history.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In manual validation, a user can receive a success or failure result from a
  representative background task through an active delivery target in under 2 minutes
  from final task completion.
- **SC-002**: In automated verification, 100% of tested cases where execution succeeds
  but delivery retries, suppresses, or fails preserve execution truth and delivery truth
  as separate inspectable outcomes.
- **SC-003**: In automated and manual validation combined, at least one scheduled task,
  one workflow-originated task, and one integration-backed outcome can each route a
  result through the shared delivery plane without requiring an active chat request.
- **SC-004**: An operator can determine the active delivery target, last delivery
  outcome, per-attempt delivery history, and whether a missing user notification was
  caused by suppression, retry, or terminal failure in under 2 minutes using
  operator-visible surfaces only.
- **SC-005**: When summary delivery is configured, multiple eligible background outcomes
  produced within one summary window can be delivered as one user-visible digest with no
  tested case of those routine-success results being lost from operator-visible delivery
  history, while failures and urgent results still bypass the digest path.

## Assumptions

- Phase 28 establishes a shared delivery plane and does not by itself close domain-level
  calendar, mail, reminder, or mobile-product behavior.
- Existing foreground reply paths remain valid, but background work needs a separate,
  reusable delivery model.
- One operator-managed environment remains the default scope; multi-user tenancy is out
  of scope for this phase.
- Delivery targets and preferences are environment-scoped and can differ between test and
  later live environments.
- User-level defaults remain the baseline preference model, with optional integration-
  specific overrides for delivery-sensitive sources.
- Summary or digest behavior initially focuses on grouping routed results and does not
  require a generalized feed or campaign system.
- Failures and urgent results are treated as immediate-delivery classes even when a
  summary window is enabled for routine successes.
- Delivery failures may occur after successful execution, and that separation is a core
  product requirement rather than an error case to hide.
