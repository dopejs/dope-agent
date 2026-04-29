# Feature Specification: Live Validation And Side-Effect Replay

**Feature Branch**: `025-live-validation-replay`  
**Created**: 2026-04-29  
**Status**: Draft  
**Input**: User description: "$speckit-specify 结合 docs/specs/025-live-validation-and-side-effect-replay.md 完成 phase 40 的工作"

**Upstream authority**: `docs/specs/025-live-validation-and-side-effect-replay.md` is the authoritative design document for this work (Roadmap 40). This specification translates that design into testable scenarios, requirements, and success criteria. Where the upstream document and this spec disagree, the upstream document wins and this spec must be updated.

## Clarifications

### Session 2026-04-29

- Q: When a tenant or global kill switch is enabled during a running live validation, should it only block new starts or also affect in-flight work? → A: Kill switches block new starts and abort pending or future side effects in running attempts; already-submitted side effects move to completed, failed, or operator-action-needed evidence.
- Q: Who can resolve operator-action-needed reconciliation states after ambiguous external commits? → A: Only tenant owners/admins or users with an explicit reconciliation permission can resolve operator-action-needed states.
- Q: What approval granularity is required for live validation side-effect scope? → A: Scope-level approval is allowed for read-only and idempotent classes, but non-idempotent mutations require per-action approval.
- Q: How should live validation handle replay candidates that mix supported and unsupported tool classes? → A: Unsupported classes block only their own steps; supported steps may run if the operator explicitly excludes unsupported work from scope.
- Q: How long should live-validation attempts, side-effect ledger entries, and reconciliation evidence be retained? → A: Retain indefinitely unless an explicit operator retention policy is applied later.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Run Permissioned Live Validation (Priority: P1)

As an operator, I need to select an eligible replay candidate, define the side-effect scope, and request live validation only after explicit permission, quota, kill-switch, and fresh approval checks pass so that I can validate changes against real systems without uncontrolled external effects.

**Why this priority**: Roadmap 40 exists to safely cross the boundary that Roadmap 33 intentionally kept non-live by default. If operators cannot start live validation through a gated and auditable path, side-effect replay remains unsafe for production use.

**Independent Test**: Use a replay candidate that includes supported read-only and side-effecting work. Confirm that live validation cannot start without the required tenant permission, quota allowance, enabled kill switches, declared side-effect scope, and the required fresh approvals for the relevant safety classes; then confirm an authorized and approved request starts with durable evidence.

**Acceptance Scenarios**:

1. **Given** an operator lacks the `live_validation.execute` permission, **When** the operator requests live validation, **Then** the request is denied before any live validation work or external side effect begins.
2. **Given** a tenant has exhausted live-validation allowance or quota state is unavailable for hosted enforcement, **When** an authorized operator requests live validation, **Then** the request is denied before validation starts and the denial is inspectable.
3. **Given** the tenant or global live-validation kill switch is active, **When** an authorized operator requests live validation, **Then** no new live validation starts and the blocked reason identifies the applicable kill switch.
4. **Given** live validation work is in scope, **When** the operator requests approval, **Then** read-only and idempotent classes may be approved at the declared scope level, non-idempotent mutations require per-action approval, and no stale replay, source-run, or prior validation approval can be reused.
5. **Given** all gates pass and required approvals are granted, **When** the operator starts live validation, **Then** the validation attempt records the approved scope, source replay candidate, tenant, operator, quota decision, and initial side-effect evidence before external mutation can proceed.

---

### User Story 2 - Replay Only Explicitly Supported Tool Classes (Priority: P1)

As an engineer responsible for replay safety, I need every replayable tool-call class to have an explicit safety classification, approval rule, idempotency expectation, retry policy, ambiguous-commit behavior, compensation guidance, ledger requirements, and test case so unsupported or unsafe work cannot silently enter live replay.

**Why this priority**: Live validation is dangerous when support is implicit. A complete replay support matrix is the planning gate that prevents missing tool classes from being treated as safe by default.

**Independent Test**: Review the replay support matrix before executor work starts and verify that every tool-call class reachable from replay candidates has an explicit row. Confirm that missing rows, unsupported classes, and non-idempotent classes without safe confirmation cannot be automatically live-replayed.

**Acceptance Scenarios**:

1. **Given** implementation planning is complete, **When** the replay support matrix is reviewed, **Then** it includes every tool-call class reachable from replay candidates and covers the required safety, approval, idempotency, retry, ambiguous-commit, compensation, ledger, and test fields.
2. **Given** a replay candidate references a tool-call class not present in the matrix, **When** live validation readiness is evaluated, **Then** that class is treated as unsupported and reported to the operator instead of being replayed.
3. **Given** a replay candidate contains both supported and unsupported tool-call classes, **When** the operator explicitly excludes unsupported work from the live-validation scope, **Then** eligible supported steps may proceed while unsupported steps remain blocked and visible.
4. **Given** a tool-call class is classified as non-idempotent mutation, **When** validation reaches that action, **Then** automatic retry is disabled unless durable evidence proves the prior attempt did not commit.
5. **Given** a downstream system can support correlation or idempotency, **When** a live side-effect attempt is prepared, **Then** the attempt includes a stable correlation or idempotency key in the validation evidence.
6. **Given** a downstream system cannot support idempotency or reconciliation evidence, **When** planning classifies the tool-call class, **Then** live replay is disabled for that class or requires explicit manual confirmation before any retry.

---

### User Story 3 - Inspect Side-Effect Ledger And Outcome Comparison (Priority: P1)

As an engineer or operator, I need to inspect a durable side-effect ledger and compare original outcomes with live validation outcomes so that I can prove which actions were attempted, skipped, completed, failed, aborted, denied, or require operator reconciliation.

**Why this priority**: Live validation cannot be trusted without durable evidence. The ledger is the audit boundary that explains what happened, what did not happen, and what requires human follow-up.

**Independent Test**: Run live validation against supported fake integration scenarios covering completed, failed, skipped, denied, aborted, timeout-after-submit, restart-after-submit, duplicate retry, ambiguous commit, and manual reconciliation paths. Confirm the ledger and comparison view expose the correct outcome for each path without relying on raw logs.

**Acceptance Scenarios**:

1. **Given** live validation prepares a side-effecting action, **When** the action is attempted, skipped, completed, failed, aborted, denied, or requires operator action, **Then** the ledger records the outcome with links to the validation attempt, source replay evidence, tool-call class, integration or runtime context, actor, approval, and correlation evidence where available.
2. **Given** a side-effect attempt times out, loses connection, receives an unknown provider response, or the daemon restarts after submit, **When** commit status cannot be proven, **Then** the ledger records an ambiguous-commit operator-action-needed state and automatic retry stops.
3. **Given** a live validation attempt is aborted, **When** the operator inspects the result, **Then** the ledger distinguishes work that was never attempted, work safely skipped, work completed before abort, and work that requires reconciliation.
4. **Given** live validation completes or stops, **When** the operator compares original and replayed outcomes, **Then** the comparison identifies matched outcomes, observed differences, unsupported replay, denied side effects, ambiguous commits, and required operator action.
5. **Given** an ambiguous commit requires reconciliation, **When** a user without tenant owner/admin authority or explicit reconciliation permission attempts to resolve it, **Then** resolution is denied while the evidence remains inspectable.
6. **Given** validation history is inspected after restart, **When** prior attempts are loaded, **Then** side-effect ledger entries and outcome comparisons remain available for audit.

---

### User Story 4 - Disable Live Validation During Operational Risk (Priority: P2)

As a tenant owner or authorized operator, I need a hard kill switch at tenant and global scope so that live validation can be stopped during incidents or operational freezes without disabling non-live replay history.

**Why this priority**: Side-effect replay touches real systems. Tenant owners and operators need a simple, reliable containment mechanism before incidents occur.

**Independent Test**: Enable tenant and global kill switches independently, attempt new live validation, and confirm starts are blocked while existing non-live replay and historical validation inspection remain available. Enable a kill switch while validation is running and confirm pending or future side effects are aborted while already-submitted side effects are recorded as completed, failed, or operator-action-needed. Confirm disabling the switch restores eligibility only after normal permission, quota, readiness, and approval gates pass.

**Acceptance Scenarios**:

1. **Given** a tenant kill switch is active, **When** any user requests live validation for that tenant, **Then** the request is denied before validation starts and the tenant-scoped block is visible.
2. **Given** a global kill switch is active, **When** any user requests live validation for any tenant, **Then** no new live validation starts and the global block is visible.
3. **Given** a kill switch blocks live validation, **When** operators inspect prior validation history or run non-live replay, **Then** those read-only or non-live paths remain available unless separately restricted.
4. **Given** a kill switch is enabled while live validation is running, **When** the attempt has pending or future side effects, **Then** those side effects are aborted and no new side-effect attempt begins while already-submitted side effects resolve to completed, failed, or operator-action-needed evidence.
5. **Given** a kill switch is deactivated, **When** an operator requests live validation again, **Then** the request still must pass permission, quota, readiness, and fresh approval gates before any side effect occurs.

### Edge Cases

- A live validation request is made for a replay candidate that has no replay support matrix coverage for one or more reachable tool-call classes.
- A replay candidate contains both supported and unsupported tool-call classes; supported steps may run only after unsupported work is explicitly excluded from scope.
- The operator approves only part of the requested side-effect scope.
- The operator grants scope-level approval for read-only or idempotent classes but withholds one or more required per-action approvals for non-idempotent mutations.
- Quota is available when readiness is checked but exhausted before live validation starts.
- Tenant or global kill switch changes state while an attempt is pending approval or running; pending and future side effects abort while already-submitted side effects require completed, failed, or operator-action-needed evidence.
- The daemon restarts after a side-effect attempt is submitted but before the outcome is recorded.
- The downstream system returns a timeout, connection loss, duplicate response, partial success, unknown response, or conflicting reconciliation evidence.
- A non-idempotent side effect has already committed but the original response was lost.
- A user can inspect an ambiguous commit but lacks tenant owner/admin authority or explicit reconciliation permission to resolve it.
- A side-effecting tool class lacks durable compensation or manual confirmation guidance.
- An operator aborts validation while read-only steps have completed and one or more side-effecting steps are pending approval.
- A validation attempt includes both local runtime tool calls and external integration operations with different safety classes.
- A live validation request targets a local-first unlimited environment rather than a hosted tenant.
- Prior side-effect ledger entries are present after rollback or downgrade and must remain auditable even if new live validation is disabled.
- An unsupported provider, sandbox operation, or connector operation is encountered during replay.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Live validation MUST be an explicit validation mode selected by an authorized operator and MUST NOT become the default replay behavior.
- **FR-002**: The system MUST require the `live_validation.execute` tenant permission before starting live validation.
- **FR-003**: The system MUST require a successful quota decision before live validation starts for hosted tenants.
- **FR-004**: The system MUST deny live validation before any live work begins when quota state cannot be safely determined for a hosted tenant.
- **FR-005**: The system MUST provide tenant-scoped and global kill switches that prevent new live validation attempts from starting.
- **FR-006**: Kill switches MUST NOT remove or hide historical live-validation evidence, side-effect ledger records, or non-live replay history.
- **FR-006a**: When a tenant-scoped or global kill switch is enabled during a running live validation attempt, the system MUST abort pending and future side effects while preserving already-submitted side effects as completed, failed, or operator-action-needed evidence.
- **FR-007**: The system MUST require fresh operator approval before any live replay action that needs approval under its replay support matrix row is attempted.
- **FR-007a**: Scope-level approval MAY cover read-only and idempotent replay classes when the approved scope is explicit; non-idempotent mutation replay MUST require per-action approval.
- **FR-008**: Fresh approvals MUST be tied to the current validation attempt, approved scope or action, actor, tenant, replay candidate, and safety class, and MUST NOT reuse approvals from the original work, prior replay, or prior validation attempt.
- **FR-009**: The system MUST record a durable live validation attempt that links to the source replay candidate, tenant, operator, requested scope, permission decision, quota decision, kill-switch decision, approval status, and terminal outcome.
- **FR-010**: The system MUST maintain a replay support matrix before executor work starts.
- **FR-011**: The replay support matrix MUST include every tool-call class reachable from replay candidates.
- **FR-012**: Each replay support matrix row MUST include tool class, safety class, required permission, approval requirement, idempotency or correlation expectation, retry policy, ambiguous-commit behavior, compensation guidance, required ledger events, and a proving test case.
- **FR-013**: The initial replay support matrix MUST classify read-only daemon inspection calls, runtime local tool calls, MCP tool calls, integration probe read operations, integration mutation probes, calendar event create/update/cancel, mail draft create/update, mail send/reply/forward, reminder lifecycle mutations, delivery dispatch attempts, connector message sends, and unsupported provider or sandbox operations that cannot be safely replayed.
- **FR-014**: No tool-call class may default to live replay support; missing matrix rows MUST be treated as unsupported.
- **FR-015**: Unsupported tool-call classes MUST be reported as unsupported validation states and MUST NOT be silently skipped, simulated as successful, or live-replayed.
- **FR-015a**: If a replay candidate includes both supported and unsupported tool-call classes, unsupported classes MUST block only their own steps; supported steps MAY proceed only when the operator explicitly excludes unsupported work from the live-validation scope.
- **FR-016**: Every replayable tool-call class MUST be classified as read-only, idempotent mutation, non-idempotent mutation, or unsupported for live replay.
- **FR-017**: The system MUST attach a stable correlation or idempotency key to external side-effect attempts whenever the downstream system can support it.
- **FR-018**: The system MUST record attempted, skipped, completed, failed, aborted, denied, and operator-action-needed side-effect outcomes in a dedicated ledger.
- **FR-019**: Side-effect ledger entries MUST link to validation attempt evidence, original replay evidence, tool-call class, runtime or integration context, approval evidence, actor, tenant, timestamp, outcome, and correlation or idempotency evidence where available.
- **FR-020**: Side-effect ledger entries MUST be durable before or atomically with external mutation attempts where feasible.
- **FR-021**: The system MUST support abort semantics that stop pending and future work, preserve already-recorded outcomes, and distinguish unattempted, skipped, completed, failed, aborted, denied, and reconciliation-needed work.
- **FR-022**: The system MUST support bounded retry semantics for replay classes where retry is declared safe by the replay support matrix.
- **FR-023**: The system MUST NOT automatically retry non-idempotent side effects after timeout, connection loss, unknown provider response, or daemon restart unless durable ledger evidence proves the prior attempt did not commit.
- **FR-024**: When external commit status cannot be proven, the system MUST stop automatic retry and expose an ambiguous-commit or equivalent operator-action-needed state.
- **FR-025**: The system MUST record compensation availability, manual confirmation requirements, or unsupported compensation for each side-effecting replay class.
- **FR-026**: The system MUST provide operator-visible reconciliation guidance for ambiguous commits, non-idempotent side effects, and side effects without automatic compensation.
- **FR-026a**: Resolving operator-action-needed reconciliation states MUST require tenant owner/admin authority or an explicit reconciliation permission; users without that authority may inspect permitted evidence but MUST NOT resolve the state.
- **FR-027**: The system MUST compare original replay evidence and live validation outcomes so operators can inspect matched outcomes, observed differences, unsupported replay, denied side effects, ambiguous commits, and required operator action.
- **FR-028**: Live validation history, side-effect ledger records, and outcome comparisons MUST remain inspectable after restart.
- **FR-028a**: Live-validation attempts, side-effect ledger entries, reconciliation decisions, and comparison evidence MUST be retained indefinitely unless an operator later applies an explicit retention policy.
- **FR-029**: The feature MUST include tests for permission denial, quota denial, kill-switch denial, approval-required behavior, completed side effects, failed side effects, skipped side effects, denied side effects, aborted attempts, timeout-after-submit, daemon restart after submit, duplicate retry, ambiguous commit, manual reconciliation, unsupported classes, non-idempotent retry prevention, matrix completeness, and comparison output.
- **FR-030**: The feature MUST include fake-backend validation coverage for live side-effect behavior and opt-in real-account smoke notes for operators who explicitly choose to validate against live external systems.
- **FR-031**: Contract validation MUST cover live validation attempt, side-effect ledger, replay support matrix, validation denial, abort, retry, ambiguous-commit, and comparison evidence shapes.
- **FR-032**: Autonomous optimization based on live validation results, broad memory or self-improvement loops, silent background live replay, and replay support for unsupported tool classes beyond explicit unsupported reporting are out of scope for this phase.

### Key Entities

- **Live Validation Attempt**: A tenant-scoped validation run requested for a replay candidate, including requested scope, actor, permission decision, quota decision, kill-switch decision, approvals, terminal status, and links to produced evidence.
- **Replay Candidate**: A previously captured replay source or fixture that may be evaluated for live validation readiness while retaining source provenance and safety boundaries.
- **Side-Effect Scope**: The operator-declared set of read-only and side-effecting actions allowed for the validation attempt, including any partial approvals or exclusions.
- **Fresh Approval**: A current approval decision for a specific validation attempt and approved scope or action, separate from approvals recorded on original work or earlier replay attempts. Scope-level approval can cover read-only and idempotent replay classes, while non-idempotent mutations require per-action approval.
- **Replay Support Matrix Row**: A required planning and verification entry describing whether a tool-call class can be live-replayed and what safety, approval, idempotency, retry, ambiguous-commit, compensation, ledger, and test obligations apply.
- **Tool-Call Safety Class**: The declared live replay classification for a tool-call class: read-only, idempotent mutation, non-idempotent mutation, or unsupported.
- **Side-Effect Ledger Entry**: Durable audit evidence for a live replay action that was attempted, skipped, completed, failed, aborted, denied, or marked operator-action-needed.
- **Correlation Or Idempotency Evidence**: Stable identity used to relate an external side-effect attempt to downstream execution or to prevent duplicate mutation where the downstream system supports it.
- **Ambiguous Commit**: A state where the system cannot prove whether an external side effect committed, requiring automatic retry to stop and operator reconciliation to begin.
- **Reconciliation Resolution**: An authorized decision that closes an operator-action-needed state after ambiguous external commit evidence is reviewed.
- **Compensation Guidance**: The declared recovery path for a side-effecting replay class, including automatic compensation, manual confirmation, or unsupported compensation.
- **Live Validation Kill Switch**: A tenant-scoped or global control that prevents new live validation attempts from starting while preserving historical evidence and non-live replay inspection.
- **Live Validation Comparison**: The operator-visible comparison between original replay evidence and live validation results, including matched outcomes, observed differences, unsupported replay, denials, ambiguous commits, and required operator action.
- **Live Validation Retention Policy**: The retention rule for live-validation attempts, side-effect ledger entries, reconciliation decisions, and comparison evidence. The default is indefinite retention unless an operator later applies an explicit policy.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: This phase adds live validation, side-effect ledger, replay support matrix, kill-switch, approval, denial, abort, retry, ambiguous-commit, reconciliation, and comparison surfaces. Non-live replay remains the default and must remain backward compatible with Roadmap 33 behavior.
- **Migration / Rollback**: Rollout should be additive and disabled-by-default for live side effects until permission, quota, matrix, approval, ledger, and kill-switch gates are verified. Rollback must disable new live validation starts while preserving historical validation attempts, side-effect ledger entries, and comparison evidence for audit.
- **Verification Strategy**: Requires unit, integration, restart, fake-backend, contract, and matrix-completeness validation for permission and quota gates, approvals, kill switches, side-effect ledger outcomes, abort and retry behavior, ambiguous commits, non-idempotent retry prevention, unsupported classes, reconciliation guidance, and original-versus-live comparison.
- **Observability Impact**: Operators need durable, structured evidence for every live validation gate and side-effect outcome, including permission denial, quota denial, kill-switch denial, approval state, attempted/skipped/completed/failed/aborted/denied/operator-action-needed ledger entries, ambiguous commits, reconciliation guidance, reconciliation-resolution authority, retention policy, and comparison results. Operator documentation must explain how to interpret and reconcile ambiguous or non-idempotent outcomes.
- **Environment & Secrets**: Development and automated verification must use the test environment by default. Fake backends are required for side-effect safety coverage. Live connectors and real accounts are optional smoke validation only, must be explicitly opted into by operators, and must not be required for normal acceptance.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of live validation start attempts in verification either pass all required permission, quota, kill-switch, readiness, and approval gates before live work begins or are denied before any external side effect is attempted.
- **SC-001a**: 100% of approval-granularity tests allow scope-level approval for read-only and idempotent replay classes while requiring per-action approval before non-idempotent mutation replay.
- **SC-002**: 100% of side-effecting replay attempts exercised in verification produce a side-effect ledger entry for attempted, skipped, completed, failed, aborted, denied, or operator-action-needed outcomes as applicable.
- **SC-003**: 100% of replay candidate tool-call classes exercised in verification have an explicit replay support matrix row; missing rows are reported as unsupported and never live-replayed.
- **SC-003a**: 100% of mixed supported/unsupported candidate tests allow supported steps to run only after unsupported work is explicitly excluded from scope, and unsupported steps are never live-replayed.
- **SC-004**: 100% of non-idempotent timeout, connection-loss, unknown-response, and restart-after-submit tests stop automatic retry unless ledger evidence proves the prior attempt did not commit.
- **SC-005**: 100% of ambiguous external commit tests expose an operator-action-needed state with reconciliation guidance within the validation evidence.
- **SC-005a**: 100% of reconciliation-resolution tests deny users without tenant owner/admin authority or explicit reconciliation permission while allowing authorized resolution to be recorded with audit evidence.
- **SC-006**: 100% of tenant and global kill-switch tests prevent new live validation starts while preserving historical validation and non-live replay inspection.
- **SC-006a**: 100% of running-attempt kill-switch tests abort pending and future side effects while recording already-submitted side effects as completed, failed, or operator-action-needed evidence.
- **SC-007**: 100% of abort tests distinguish unattempted, skipped, completed, failed, aborted, denied, and reconciliation-needed work in the validation evidence.
- **SC-008**: 100% of live validation comparisons exercised in verification identify matched outcomes, observed differences, unsupported replay, denied side effects, ambiguous commits, or required operator action without raw log inspection.
- **SC-009**: Live validation attempts, side-effect ledger entries, and comparison evidence remain inspectable after restart for 100% of restart persistence scenarios exercised during release verification.
- **SC-009a**: 100% of retention checks confirm live-validation attempts, side-effect ledger entries, reconciliation decisions, and comparison evidence remain available by default unless an explicit operator retention policy is applied.
- **SC-010**: Contract validation passes for live validation attempt, side-effect ledger, replay support matrix, validation denial, abort, retry, ambiguous-commit, and comparison evidence shapes.
- **SC-011**: Operator smoke verification can explain one successful side-effect replay, one denied live validation request, one unsupported tool class, and one ambiguous-commit reconciliation path from structured evidence in under 20 minutes.
- **SC-012**: No automated verification path requires real external accounts; optional real-account smoke validation is documented as explicit opt-in and records the operator-selected side-effect scope.

## Assumptions

- Roadmap 33 evaluation and replay harness, Roadmap 34 tenant identity and access foundation, Roadmap 38 billing quotas and usage accounting, and Roadmap 39 production install, upgrade, backup, and soak capabilities are complete enough to provide replay candidates, tenant permissions, quota gates, and operational verification boundaries.
- Non-live replay remains the default path for evaluation, and this phase adds a separate explicit live validation mode rather than changing existing replay semantics.
- Operators are the primary users who start live validation and inspect outcomes; tenant owners or authorized operators can manage live-validation kill-switch behavior according to existing tenant administration policy.
- The first version prioritizes controlled validation of supported tool-call classes and explicit unsupported reporting over broad replay coverage.
- Fake backends are sufficient for required automated side-effect validation; real external accounts are only used for optional, operator-approved smoke checks.
- Some downstream systems cannot prove idempotency or reconciliation; those classes are either unsupported for live replay or require manual confirmation before retry.
- Historical ledger and comparison evidence should remain available for audit even if new live validation is later disabled or rolled back, and is retained indefinitely by default unless an explicit operator retention policy is applied later.
