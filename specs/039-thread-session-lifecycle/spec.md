# Feature Specification: Daemon-Owned Thread And Session Lifecycle

**Feature Branch**: `039-thread-session-lifecycle`  
**Created**: 2026-05-11  
**Status**: Draft  
**Input**: User description: "$speckit-specify 结合 docs/specs/039-daemon-owned-thread-and-session-lifecycle.md 完成 phase 54 的工作"

**Upstream authority**: `docs/specs/039-daemon-owned-thread-and-session-lifecycle.md` is the authoritative upstream document for this work (Roadmap 54). This specification translates that document into testable scenarios, requirements, and success criteria. Where the upstream document and this spec disagree, the upstream document wins and this spec must be updated.

**Related contract**: `docs/specs/033-channel-connector-conformance-contract.md` defines the shared channel connector conformance expectations that inbound channel messages must satisfy when they attach to daemon-owned session and thread truth.

## Clarifications

### Session 2026-05-11

- Q: When a user resets a thread, should reset preserve the thread identity or create a brand-new thread? → A: Reset keeps the same thread ID and starts a new active session segment.
- Q: Which tenant permissions gate thread/session lifecycle inspection and mutations? → A: Inspection requires `credentials.inspect`; lifecycle mutations require `connectors.manage`.
- Q: How long should lifecycle, source, and runtime projection evidence be retained by default? → A: Retain lifecycle, source, and runtime projection evidence for 90 days by default unless tenant policy requires longer.
- Q: What should archive do when a thread has active or pending runtime work? → A: Archive blocks future continuation but does not cancel active runs, approvals, workflows, replies, or deliveries.
- Q: How should inbound source conversations map to daemon-owned thread identity? → A: One current thread per tenant, connector, source account, and source conversation.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Inspect And Find Conversations (Priority: P1)

As a tenant user, I can see recent conversations for my tenant, distinguish active, reset, archived, and reopened threads, and open a detail view that explains the current session lifecycle without relying on hidden channel state or prompt files.

**Why this priority**: Thread and session lifecycle cannot be productized if users cannot find the conversation they are in, understand its current lifecycle state, or verify that continuity is owned by the daemon.

**Independent Test**: Can be tested by creating conversations from multiple sources for one tenant, then confirming authorized users can list, filter, and inspect thread/session state, source identity, recent activity, lifecycle state, and continuation eligibility without accessing logs or channel-local files.

**Acceptance Scenarios**:

1. **Given** a tenant has conversations started from chat, channel, or workflow sources, **When** an authorized user opens the conversation list, **Then** the user sees tenant-scoped threads with source, session state, lifecycle state, last activity, and available actions.
2. **Given** a user opens a thread detail view, **When** the thread has current and historical sessions, **Then** the user sees the active session, prior lifecycle transitions, source linkage, continuation eligibility, and related runtime evidence summaries.
3. **Given** a user lacks `credentials.inspect` for a tenant, **When** they request a thread list or detail view, **Then** access is denied without revealing thread existence, source identity, session state, or runtime evidence for inaccessible tenants.

---

### User Story 2 - Reset, Archive, And Reopen Threads (Priority: P1)

As a tenant user, I can explicitly reset a conversation to start fresh, archive a conversation to remove it from active work, and reopen an archived conversation when allowed, while historical runtime evidence remains inspectable.

**Why this priority**: Reset and archive are the primary user-visible lifecycle operations. They must be explicit, reversible where appropriate, and safe for support review because they affect future conversation continuity.

**Independent Test**: Can be tested by resetting, archiving, and reopening representative threads, then verifying future work follows the new lifecycle state while prior runs, workflows, approvals, deliveries, and messages remain durable and auditable.

**Acceptance Scenarios**:

1. **Given** an active thread has prior messages and runtime evidence, **When** an authorized user resets it, **Then** the same thread remains visible, future conversation work starts from a new active session segment, and prior evidence remains available as history.
2. **Given** an active or reset thread is archived, **When** new user or channel activity attempts to continue it, **Then** the system prevents unintended continuation unless the thread is explicitly reopened or a new thread is created.
3. **Given** a thread has active or pending runs, approvals, workflows, replies, or deliveries, **When** an authorized user archives it, **Then** future continuation is blocked but already accepted runtime work is not cancelled by the archive action.
4. **Given** an archived thread is reopened by an authorized user, **When** the user resumes interaction, **Then** the thread becomes eligible for future work according to its current source and session rules while preserving the archive and reopen audit trail.
5. **Given** a user lacks `connectors.manage` for a tenant, **When** they request reset, archive, or reopen, **Then** the mutation is denied without changing thread lifecycle state.
6. **Given** required lifecycle audit evidence cannot be recorded, **When** a reset, archive, or reopen action is requested, **Then** the action fails closed and leaves the thread lifecycle unchanged.

---

### User Story 3 - Trace Channel Messages To Runtime Evidence (Priority: P1)

As an operator, I can trace an inbound channel message to the daemon-owned thread/session it attached to and to the run, workflow, approval, delivery, and reply records it caused.

**Why this priority**: Operators need a durable chain of evidence from external channel messages to runtime consequences. Without this, incidents require connector-specific investigation and cannot be debugged consistently.

**Independent Test**: Can be tested by sending representative inbound connector messages, then confirming authorized operators can reconstruct the source channel, source account, thread, session, run, workflow, approval, delivery, and reply outcomes from tenant-scoped product evidence.

**Acceptance Scenarios**:

1. **Given** an inbound connector message is accepted for a known source conversation, **When** an operator inspects the thread or message evidence, **Then** the operator can trace the message to the current daemon-owned thread for that tenant, connector, source account, and source conversation, and to the session plus run or workflow it caused.
2. **Given** an inbound message is ignored, blocked, duplicated, disabled, or failed, **When** an operator inspects the relevant thread or source evidence, **Then** the routing outcome is visible without implying that assistant work was created.
3. **Given** a run requires approval or produces delivery outcomes, **When** the operator opens the linked thread detail, **Then** approval state, delivery outcomes, reply outcomes, and workflow linkage are visible as separate facts.
4. **Given** evidence contains secrets, raw provider payloads, or message bodies outside the allowed inspection boundary, **When** it is displayed to an authorized operator, **Then** unsafe detail is redacted or suppressed while preserving stable reason and linkage metadata.

---

### User Story 4 - Survive Restarts And Connector Handoffs (Priority: P2)

As an operator, I can restart the daemon or reconnect a channel connector without losing thread lifecycle state, session ownership, or source-to-runtime linkage for future messages.

**Why this priority**: Lifecycle truth must be restart-safe and daemon-owned. If connector-local state is required to resume conversations, the product remains fragile and difficult to operate.

**Independent Test**: Can be tested by creating active, reset, archived, and reopened threads, restarting the daemon, replaying connector events, and confirming the same lifecycle state and linkage rules are applied after restart.

**Acceptance Scenarios**:

1. **Given** active, reset, archived, and reopened threads exist, **When** the daemon restarts, **Then** lifecycle state, session linkage, source identity, and audit history remain intact.
2. **Given** a connector previously routed into local or transient state, **When** it processes a new inbound message after this feature is enabled, **Then** the message attaches to daemon-owned thread/session truth or fails closed with inspectable reason evidence.
3. **Given** an inbound message is replayed after restart, **When** routing evaluates the message, **Then** duplicate and continuation behavior uses daemon-owned lifecycle state rather than connector-local memory.

---

### User Story 5 - Inspect Conversation History Without Memory Recall (Priority: P3)

As a tenant user or support operator, I can inspect conversation lifecycle metadata and runtime evidence without the system treating historical thread content as memory recall, semantic summary, or automatic context-packing input.

**Why this priority**: Roadmap 54 provides conversation continuity metadata, not memory. The product must not blur lifecycle inspection with future knowledge-recall or context-management features.

**Independent Test**: Can be tested by creating historical conversations with messages and runtime evidence, then confirming inspection and lifecycle actions expose only authorized lifecycle and evidence views and do not create memory recall, semantic summaries, or autonomous pruning behavior.

**Acceptance Scenarios**:

1. **Given** a thread has historical messages and runtime evidence, **When** an authorized user inspects it, **Then** the user sees lifecycle and evidence metadata without any claim that the history has become long-term memory.
2. **Given** a user resets or archives a thread, **When** future assistant work begins, **Then** historical evidence remains inspectable but is not automatically injected as recalled memory or semantic summary by this phase.
3. **Given** a thread has old activity, **When** lifecycle inspection runs, **Then** this phase does not autonomously prune, summarize, or repack conversation context.

### Edge Cases

- A tenant has no conversations; the conversation list must show an empty state without exposing other tenants or suggesting destructive lifecycle actions.
- A tenant has more threads than fit on one page; authorized users must be able to reach all tenant threads across deterministic paginated results.
- A thread receives new activity while a user pages through or inspects the list; ordering and pagination must avoid missing or duplicated entries.
- A channel message arrives with no known source mapping, stale source mapping, disabled connector, unsupported source, or inaccessible tenant binding.
- Two inbound messages for the same source conversation arrive concurrently and attempt to create or continue a thread.
- A tenant, connector, source account, and source conversation combination already has a current thread; later accepted inbound messages must attach to that current thread unless archive blocks continuation or a lifecycle action changes eligibility.
- A reset, archive, reopen, connector routing decision, run start, approval update, or delivery update happens while another lifecycle action is in progress.
- A reset is requested for an already reset or archived thread.
- An archive is requested for a thread with an active run, pending approval, in-flight workflow, or pending delivery.
- A reopen is requested for a thread whose source channel, account binding, connector, or tenant access is no longer valid.
- Runtime evidence exists without a complete channel source, such as shell-originated work, scheduled work, workflow-originated work, or legacy session data.
- Historical runtime evidence includes records created before daemon-owned thread truth existed.
- A daemon restart occurs after accepting a message but before run, workflow, approval, reply, or delivery evidence is fully recorded.
- A connector retries or replays an event after restart, reset, archive, or reopen.
- Required audit evidence cannot be recorded for a lifecycle mutation.
- Redaction confidence is insufficient for source identity, channel metadata, provider payloads, message bodies, or runtime evidence details.
- Lifecycle, source, or runtime projection evidence reaches its default retention limit; it must expire from normal inspection after 90 days unless covered by an authorized longer tenant retention policy.
- Permission changes occur while a user has a thread list, detail, or support evidence view open.
- Existing clients continue to use session behavior that predates first-class thread lifecycle.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST treat tenant-scoped threads and sessions as first-class daemon-owned product resources.
- **FR-002**: Authorized users MUST be able to list tenant threads with lifecycle state, source type, source identity summary, active session summary, last activity, and available lifecycle actions.
- **FR-002a**: Thread lists MUST support pagination and deterministic ordering so authorized users can reach every tenant thread without missing or duplicated entries.
- **FR-003**: Authorized users MUST be able to inspect thread detail, including current lifecycle state, current session, prior sessions, source linkage, related runs, related workflows, approvals, replies, delivery outcomes, and lifecycle audit history.
- **FR-004**: Thread and session reads and lifecycle mutations MUST be tenant-scoped and permission-gated, and unauthorized attempts MUST be denied without revealing inaccessible thread existence, source identity, or runtime evidence.
- **FR-004a**: Thread/session lifecycle list, detail, source linkage, and runtime evidence inspection MUST require `credentials.inspect`.
- **FR-004b**: Thread/session lifecycle reset, archive, and reopen mutations MUST require `connectors.manage`.
- **FR-005**: The system MUST represent lifecycle states for active, reset, archived, and reopened threads, including enough transition history to explain how the current state was reached.
- **FR-006**: Authorized users MUST be able to reset an active or reopened thread explicitly.
- **FR-007**: Reset MUST start future conversation work from a fresh lifecycle segment and MUST NOT delete, rewrite, or hide historical messages, sessions, runs, workflows, approvals, replies, deliveries, or audit evidence.
- **FR-007a**: Reset MUST preserve the thread identity and create a new active session segment for future work.
- **FR-008**: Authorized users MUST be able to archive a thread explicitly.
- **FR-009**: Archived threads MUST remain inspectable by authorized users and support operators.
- **FR-010**: Archived threads MUST NOT accept unintended continuation from users, channel connectors, scheduled work, workflows, or retries unless the thread is explicitly reopened or a new thread is created.
- **FR-010a**: Archive MUST NOT cancel active or pending runs, approvals, workflows, replies, or deliveries that were already accepted before the archive action.
- **FR-011**: Authorized users MUST be able to reopen an archived thread when its tenant, source, connector, and session rules still allow continuation.
- **FR-012**: Reopen MUST preserve archive and reopen evidence and MUST NOT erase or collapse prior lifecycle history.
- **FR-013**: Reset, archive, and reopen actions MUST record tenant, actor, action, prior state, resulting state, timestamp, reason where provided, and affected source/session references as audit evidence.
- **FR-014**: Lifecycle mutations MUST fail closed and leave thread state unchanged when required audit evidence cannot be recorded.
- **FR-015**: Lifecycle mutations for the same thread MUST converge to one auditable state when concurrent reset, archive, reopen, routing, run, workflow, approval, or delivery updates occur.
- **FR-016**: Inbound channel connector messages MUST attach to daemon-owned thread/session truth rather than connector-local conversation state.
- **FR-017**: Channel source linkage MUST include tenant, connector, source account, source conversation, source message, routing outcome, and redacted source identity sufficient for authorized tracing.
- **FR-017a**: The system MUST maintain at most one current thread for each tenant, connector, source account, and source conversation combination.
- **FR-017b**: Accepted inbound connector messages for an existing tenant, connector, source account, and source conversation combination MUST attach to that current thread unless lifecycle state blocks continuation or explicitly creates a new eligible current thread.
- **FR-018**: Accepted inbound messages MUST link the source message to the daemon-owned thread/session and to the run or workflow work they caused.
- **FR-019**: Ignored, blocked, duplicate, disabled, unsupported, and failed inbound messages MUST produce inspectable source-to-routing evidence without implying that assistant work was created.
- **FR-020**: Thread detail MUST present assistant execution outcomes, foreground reply outcomes, background delivery outcomes, workflow outcomes, and approval outcomes as separate facts.
- **FR-021**: Thread/session lifecycle state, source linkage, routing decisions, and runtime projections MUST survive daemon restart.
- **FR-022**: Restart recovery MUST preserve in-progress lifecycle and runtime linkage well enough for operators to determine whether a message, run, workflow, approval, reply, or delivery needs retry, suppression, or manual review.
- **FR-023**: Existing session routing behavior MUST remain compatible unless a caller opts into new lifecycle views or actions.
- **FR-024**: Legacy or pre-existing sessions without complete thread linkage MUST remain inspectable as partial or migrated lifecycle evidence rather than being silently discarded.
- **FR-025**: The system MUST expose thread/session lifecycle behavior through the product surfaces needed by users, operators, and client integrations for list, detail, reset, archive, reopen, source tracing, and runtime evidence inspection.
- **FR-026**: Thread/session lifecycle MUST NOT add memory recall, context packing, semantic summaries, autonomous conversation pruning, or memory-driven routing.
- **FR-027**: Historical thread evidence exposed by this phase MUST be treated as inspection and audit evidence, not as automatically recalled assistant memory.
- **FR-028**: Source and runtime evidence MUST redact or suppress secrets, raw provider payloads, message bodies where not allowed, cross-tenant identifiers, and unsafe provider details before user or support exposure.
- **FR-029**: If lifecycle, source, or runtime evidence cannot be confidently redacted, the system MUST suppress unsafe detail, emit redaction-failure audit evidence, and show only a generic safe classification.
- **FR-030**: Thread/session lifecycle documentation and operator views MUST make the distinction between lifecycle continuity metadata and memory recall explicit.
- **FR-031**: Lifecycle, source, and runtime projection evidence MUST use a 90-day default retention period unless an authorized tenant retention policy requires longer retention.

### Key Entities *(include if feature involves data)*

- **Thread**: A tenant-owned conversation container with lifecycle state, source linkage, current session, lifecycle transition history, and runtime evidence projections.
- **Session**: A bounded unit of conversation continuity within a thread, including current status, source relationship, and links to runs or workflow work produced during that session. Reset creates a new active session segment while preserving the thread identity.
- **Thread Lifecycle State**: The current product state of a thread, such as active, reset, archived, or reopened, with transition history and audit evidence.
- **Lifecycle Action**: A user-initiated reset, archive, or reopen request with actor, tenant, prior state, resulting state, reason where provided, and audit outcome.
- **Source Linkage**: The relationship between a thread/session and its origin, including channel connector, source account, source conversation, source message, shell/chat source, scheduled source, or workflow source. For inbound channel connectors, the tenant, connector, source account, and source conversation identify the current thread for continuation.
- **Connector Message Evidence**: Redacted evidence for how an inbound connector message was routed, including accepted, ignored, blocked, duplicate, disabled, unsupported, or failed outcomes.
- **Runtime Projection**: The thread-visible summary of related runs, workflows, approvals, replies, deliveries, and outcomes without merging those facts into one ambiguous status.
- **Lifecycle Audit Record**: Tenant-scoped evidence of lifecycle inspection, reset, archive, reopen, permission denial, redaction failure, audit-write failure, or restart recovery outcome.
- **Legacy Session Evidence**: Partial or migrated session history created before first-class daemon-owned thread truth was available.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Adds first-class thread/session lifecycle, source linkage, and runtime evidence projections across tenant product surfaces and client integration contracts. Existing session routing must remain compatible, and channel connector conformance meanings remain authoritative for inbound routing outcomes.
- **Migration / Rollback**: Existing sessions and channel-local conversation state may need projection into daemon-owned thread truth. Rollout can expose lifecycle views and actions only after thread truth is durable for the tenant. Rollback hides new lifecycle actions and routes new channel activity through the prior compatible behavior while preserving already-created thread, session, lifecycle, and audit evidence for authorized inspection.
- **Verification Strategy**: Required validation includes list/detail behavior, tenant permission denials, reset, archive, reopen, concurrent lifecycle mutation handling, audit-write failure behavior, connector message attachment, source-to-runtime tracing, restart recovery, legacy session projection, redaction, client contract coverage, operator shell coverage, and connector regression proving inbound messages attach to daemon-owned thread truth.
- **Observability Impact**: Operators must gain thread lifecycle transitions, source routing outcomes, runtime linkage, restart recovery status, permission denials, audit-write failures, and redaction failures as product evidence without relying on raw logs or connector-local state.
- **Environment & Secrets**: Development and automated verification must default to the repository test environment. Live connectors and production tenants must not be touched by default. Any live connector walkthrough must use explicitly approved tenant-owned test accounts, redact provider evidence, and avoid exposing secrets, tokens, raw provider payloads, or disallowed message bodies.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Authorized users can find and open any recent tenant thread represented in the verification dataset within 2 minutes using the conversation list and detail surfaces.
- **SC-002**: 100% of thread list pagination tests return deterministic results with no missing or duplicated threads across pages.
- **SC-003**: 100% of reset tests prove future conversation work starts from a fresh lifecycle segment while historical runtime evidence remains inspectable.
- **SC-004**: 100% of archive tests prove archived threads remain inspectable and do not accept unintended continuation until explicitly reopened or replaced by a new thread.
- **SC-004a**: 100% of archive-with-active-work tests prove archive blocks future continuation without cancelling already accepted runs, approvals, workflows, replies, or deliveries.
- **SC-005**: 100% of reopen tests preserve archive/reopen evidence and restore continuation eligibility only when tenant, source, connector, and session rules allow it.
- **SC-006**: 100% of lifecycle mutation tests with required audit-write failure deny the mutation without changing thread state.
- **SC-007**: 100% of concurrent lifecycle mutation tests converge to one auditable thread state.
- **SC-008**: 100% of accepted inbound connector message tests link the source message to a daemon-owned thread/session and to the run or workflow work it caused.
- **SC-008a**: 100% of source-continuation tests attach later accepted inbound messages for the same tenant, connector, source account, and source conversation to the same current thread unless lifecycle state blocks continuation or a lifecycle action creates a new eligible current thread.
- **SC-009**: 100% of ignored, blocked, duplicate, disabled, unsupported, and failed inbound message tests produce inspectable routing evidence without creating misleading assistant work evidence.
- **SC-010**: Authorized operators can trace a representative channel incident from source message to thread, session, run or workflow, approval, reply, and delivery facts within 5 minutes.
- **SC-011**: 100% of restart recovery tests preserve thread lifecycle state, source linkage, and runtime projections for active, reset, archived, and reopened threads.
- **SC-012**: 100% of legacy or partial-session cases in the verification dataset remain inspectable with clear partial-evidence status rather than disappearing or being misclassified.
- **SC-013**: 100% of list, detail, source linkage, and runtime evidence inspection attempts without `credentials.inspect` are denied without exposing inaccessible tenant thread existence, source identity, or runtime evidence.
- **SC-013a**: 100% of reset, archive, and reopen mutation attempts without `connectors.manage` are denied without changing thread lifecycle state.
- **SC-014**: Redaction validation finds zero exposed secrets, raw provider payloads, disallowed message bodies, cross-tenant identifiers, or unsafe provider details in user-facing, support-facing, test, log, fixture, and audit output.
- **SC-015**: Verification confirms this phase adds no memory recall, semantic summary generation, autonomous pruning, or automatic context-packing behavior in 100% of lifecycle inspection, reset, archive, and reopen flows.
- **SC-016**: 100% of lifecycle, source, and runtime projection evidence in retention tests expires from normal inspection after 90 days unless covered by an authorized longer tenant retention policy.

## Assumptions

- Roadmaps 25, 28, and 48 provide the scheduled work, delivery, notification, and channel connector conformance behavior needed to trace source messages to runtime outcomes.
- Phase 54 uses existing tenant identity and permission boundaries; it does not define a new tenant access model.
- Reset and archive are explicit product actions and must be auditable. They are not implicit side effects of inactivity, failures, or context size.
- Archived threads are hidden from default active conversation continuation but remain available to authorized inspection.
- Archive is a thread continuation control, not a cancellation mechanism for runtime work already accepted before the archive action.
- Reopen is allowed only when current tenant, source, connector, and session rules can safely continue the thread.
- Inbound channel continuation is keyed by tenant, connector, source account, and source conversation; source message identity is evidence for a specific inbound event, not the thread identity.
- Conversation continuity metadata is separate from memory recall. Historical evidence may be inspected but is not automatically recalled as assistant memory by this phase.
- Existing sessions may contain incomplete linkage and should be projected or labeled as partial evidence instead of requiring perfect historical reconstruction.
- Lifecycle, source, and runtime projection evidence follows the project-standard 90-day default retention model unless an authorized tenant policy requires longer retention.
- Default automated verification uses fake or test-environment channel evidence. Live connector validation is optional unless a later release-readiness gate explicitly requires approved safe accounts.
