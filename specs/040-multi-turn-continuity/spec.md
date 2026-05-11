# Feature Specification: Non-Knowledge Multi-Turn Continuity

**Feature Branch**: `040-multi-turn-continuity`  
**Created**: 2026-05-11  
**Status**: Draft  
**Input**: User description: "$speckit-specify 结合 docs/specs/040-non-knowledge-multi-turn-continuity.md 完成 phase 55 的工作"

**Upstream authority**: `docs/specs/040-non-knowledge-multi-turn-continuity.md` is the authoritative upstream document for this work (Roadmap 55). This specification translates that document into testable scenarios, requirements, and success criteria. Where the upstream document and this spec disagree, the upstream document wins and this spec must be updated.

**Dependency**: Roadmap 54 (`specs/039-thread-session-lifecycle/spec.md`) provides daemon-owned thread and session lifecycle, reset, source linkage, permissions, and operator inspection foundations. This phase adds bounded recent-turn continuity on top of that lifecycle truth.

## Clarifications

### Session 2026-05-11

- Q: Which permission gates thread-level continuity reset in this phase? → A: Reset continuity uses the existing lifecycle mutation permission: `connectors.manage`.
- Q: Which directly linked runtime artifact content may be included as continuity input? → A: Include only user-visible, redacted artifact excerpts tied to included turns.
- Q: What latency target should bounded continuity assembly satisfy for the default window? → A: p95 continuity assembly under 500 ms for the default window.
- Q: What ordering rule should determine continuity inclusion when messages are concurrent or replayed? → A: Order by daemon acceptance sequence, with source timestamp retained as evidence.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Continue Recent Thread Context (Priority: P1)

As a user in an active thread, I can ask a follow-up question that depends on the immediate prior exchange without restating the prior message, and the assistant uses only bounded recent turns from the current thread segment to respond.

**Why this priority**: Recent-turn continuity is the primary user value of this phase. Without it, thread lifecycle exists but active conversations still behave like isolated single-turn requests.

**Independent Test**: Can be tested by starting an active thread, asking an initial question, then asking a follow-up that depends on the prior turn and verifying the response uses the immediate prior exchange while excluding unrelated threads, older reset segments, and memory or knowledge-plane sources.

**Acceptance Scenarios**:

1. **Given** an active thread has one prior user message and assistant response in its current segment, **When** the same user asks a follow-up that refers to "that", "it", or "the previous answer", **Then** the assistant response reflects the immediate prior exchange without requiring the user to restate it.
2. **Given** an active thread has more eligible turns than the continuity limit, **When** the user asks a follow-up, **Then** only the most recent eligible turns within the deterministic limit are included.
3. **Given** another thread for the same tenant contains related content, **When** the user asks a follow-up in the current thread, **Then** the response is based only on eligible recent turns from the current thread segment.
4. **Given** memory, semantic search, knowledge graph, summaries, or long-term preferences contain related content, **When** the user asks a follow-up covered by this phase, **Then** those sources are not used as continuity input.

---

### User Story 2 - Reset Continuity Explicitly (Priority: P1)

As a user, I can reset a thread so prior turns stop influencing future responses while the same thread remains inspectable as historical evidence.

**Why this priority**: Reset is the user-visible safety boundary that keeps bounded continuity from becoming hidden memory. Users and operators need a deterministic way to start fresh.

**Independent Test**: Can be tested by creating a thread with several prior turns, resetting it, asking a follow-up that would require pre-reset context, and verifying the assistant does not use pre-reset turns while authorized inspection still shows historical evidence and the reset boundary.

**Acceptance Scenarios**:

1. **Given** an active thread has eligible recent turns, **When** an authorized user resets the thread, **Then** future continuity starts after the reset boundary and pre-reset turns are excluded from inclusion.
2. **Given** a reset thread has historical turns before the reset boundary, **When** an authorized user inspects the thread, **Then** the historical turns remain visible as evidence according to permission and retention rules but are marked excluded from future continuity.
3. **Given** a reset occurs while new work is being accepted, **When** the next assistant response is prepared, **Then** the response uses a single auditable reset boundary and does not mix pre-reset and post-reset turns.

---

### User Story 3 - Inspect Included Continuity Evidence (Priority: P1)

As an operator, I can inspect which recent turns and runtime artifacts were considered for a response, why each was included or excluded, and which deterministic limit or reset boundary applied.

**Why this priority**: Continuity changes assistant behavior. Operators need product evidence to explain responses without scraping logs, guessing prompt assembly, or exposing unsafe raw provider payloads.

**Independent Test**: Can be tested by producing responses with no prior turns, within-limit turns, over-limit turns, reset-excluded turns, redacted turns, and channel-originated turns, then verifying authorized inspection shows stable references, inclusion reasons, exclusion reasons, limits applied, and redacted-safe previews.

**Acceptance Scenarios**:

1. **Given** a response used recent-turn continuity, **When** an authorized operator inspects its runtime evidence, **Then** the operator sees the included turn references, source summaries, ordering, reset segment, and inclusion policy applied.
2. **Given** eligible-looking turns were not included because of count limits, reset boundaries, permission boundaries, redaction failure, source mismatch, unsupported source, or retention expiry, **When** an authorized operator inspects the evidence, **Then** those exclusions are visible with stable reason classifications.
3. **Given** continuity evidence contains secrets, raw provider payloads, disallowed message bodies, or cross-tenant identifiers, **When** it is displayed, **Then** unsafe detail is redacted or suppressed while preserving enough stable metadata to debug inclusion behavior.

---

### User Story 4 - Preserve Continuity Across Supported Surfaces (Priority: P2)

As a user, I get the same bounded continuity behavior when using Web, TUI, or supported channel paths that carry daemon-owned thread identity.

**Why this priority**: Recent-turn continuity must be daemon-owned product behavior rather than a client-specific convenience. Supported clients and channel connectors should not drift into different context rules.

**Independent Test**: Can be tested by creating equivalent active threads through Web, TUI, and supported channel paths, asking follow-up questions, and verifying the same thread identity, reset boundary, inclusion limits, and operator evidence apply.

**Acceptance Scenarios**:

1. **Given** Web and TUI interactions attach to daemon-owned thread identity, **When** a user asks follow-ups on each surface, **Then** both surfaces apply the same bounded continuity rules.
2. **Given** a supported channel path carries tenant, connector, source account, and source conversation identity, **When** a user sends follow-up messages in that source conversation, **Then** accepted messages attach to the current thread segment and use the same bounded continuity rules.
3. **Given** a source cannot provide a valid thread identity or continuation is blocked by lifecycle state, **When** a message is processed, **Then** continuity is not silently inferred and the outcome is inspectable.

---

### User Story 5 - Keep Continuity Separate From Knowledge (Priority: P2)

As a tenant user or support operator, I can rely on this phase as recent-thread continuity only, without it creating memory, semantic summaries, knowledge retrieval, or long-term personalization.

**Why this priority**: This phase is the last non-knowledge continuity layer before future knowledge-plane work. Its scope must remain bounded and auditable so later context engineering can extend it without changing its safety guarantees.

**Independent Test**: Can be tested by configuring related historical content outside the eligible recent turns, then verifying responses, evidence, reset behavior, and retention behavior never claim or demonstrate memory recall, semantic retrieval, summarization, or preference learning from this phase.

**Acceptance Scenarios**:

1. **Given** old thread history, unrelated threads, or tenant knowledge contain relevant content, **When** a response is prepared by this phase, **Then** only eligible recent turns and directly linked runtime artifacts from the current thread segment can affect continuity.
2. **Given** a conversation contains repeated preferences or facts, **When** future unrelated threads begin, **Then** this phase does not personalize responses using those prior turns.
3. **Given** continuity records reach their normal inspection retention limit, **When** authorized users inspect or continue a thread, **Then** expired records are no longer available for normal inspection or continuity inclusion unless covered by an authorized longer tenant retention policy.

### Edge Cases

- A thread has no prior eligible turns; the next response must proceed without continuity and evidence must show an empty inclusion set.
- A thread has exactly the maximum number of eligible turns; all eligible turns may be included in daemon acceptance sequence order.
- A thread has more than the maximum number of eligible turns; older eligible turns by daemon acceptance sequence must be excluded with an over-limit reason.
- A thread has turns before and after a reset boundary; only post-reset eligible turns may be included.
- A thread is archived, reopened, and continued; inclusion must follow the current active segment and preserve archive/reopen evidence separately.
- A prior turn contains a failed, cancelled, blocked, or policy-denied runtime outcome; inclusion evidence must distinguish user-visible conversation text from runtime artifacts that are not eligible for continuity.
- A prior turn references a tool result, approval, workflow, reply, delivery, or artifact; only user-visible, safely redacted artifact excerpts tied to included turns may be considered as continuity input, and other directly linked artifacts may appear only as evidence references where allowed.
- A prior turn or artifact cannot be safely redacted; unsafe content must be excluded or previewed only as a safe classification.
- Two messages arrive concurrently in the same thread and both could become the next recent turn; ordering must converge to one daemon acceptance sequence while retaining source timestamps as evidence.
- A connector retries or replays a source message; duplicate turns must not be counted twice for continuity.
- A daemon restart occurs after a turn is accepted but before its response evidence is fully recorded.
- Existing or legacy sessions have incomplete turn ordering or source linkage; they must not be silently included as continuity without enough evidence to prove eligibility.
- Permission changes occur between response creation and later operator inspection.
- The continuity inclusion window reaches content, count, age, or retention boundaries.
- Live connector evidence contains provider payloads, secrets, message bodies outside the allowed inspection boundary, or cross-tenant identifiers.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide bounded recent-turn continuity for active daemon-owned threads and sessions.
- **FR-002**: The system MUST persist accepted user and assistant turns that are eligible for continuity with tenant, thread, session segment, source, daemon acceptance sequence, source timestamp where available, actor role, response relationship, and runtime evidence references.
- **FR-003**: The system MUST record enough source linkage for chat and supported channel paths to distinguish current-thread continuity from unrelated tenant, connector, account, conversation, or source-message activity.
- **FR-004**: The default continuity window MUST include no more than the 12 most recent eligible prior turns from the current active thread segment, excluding the current user input.
- **FR-005**: The default continuity window MUST exclude turns older than 30 days from normal inclusion unless an authorized tenant policy explicitly allows a shorter or longer active-continuity window.
- **FR-006**: Continuity inclusion MUST be deterministic for the same thread state, reset boundary, permission boundary, retention state, and inclusion policy.
- **FR-007**: Continuity inclusion MUST preserve daemon acceptance sequence order from oldest included turn to newest included turn, and source timestamps MUST remain available as evidence where provided.
- **FR-008**: Continuity inclusion MUST only use turns and directly linked runtime artifacts from the current thread segment after the most recent reset boundary.
- **FR-008a**: Directly linked runtime artifact content MUST be eligible for continuity input only as user-visible, safely redacted excerpts tied to included turns.
- **FR-008b**: Runtime artifacts that are not user-visible, not safely redacted, not directly tied to an included turn, or outside the inspection boundary MUST NOT contribute content to continuity input.
- **FR-009**: Reset MUST exclude all pre-reset turns and artifacts from future continuity while preserving them as historical evidence according to lifecycle, permission, redaction, and retention rules.
- **FR-009a**: Thread-level continuity reset MUST require the existing lifecycle mutation permission, `connectors.manage`.
- **FR-010**: Archived threads MUST NOT accept unintended continuity until explicitly reopened or replaced by a new eligible thread according to lifecycle rules.
- **FR-011**: Reopened threads MUST use only the eligible turns in the current continuation segment and MUST preserve archive/reopen evidence separately from continuity evidence.
- **FR-012**: The system MUST expose an operator-visible continuity preview for each response that uses or evaluates recent-turn continuity.
- **FR-013**: Continuity preview MUST show included turn references, source summaries, ordering, reset segment, inclusion policy, and directly linked runtime artifact references where available and allowed.
- **FR-014**: Continuity preview MUST show exclusion reasons for prior turns that were not included because of count limit, age limit, reset boundary, retention expiry, permission denial, source mismatch, unsupported source, duplicate event, incomplete evidence, redaction failure, or lifecycle block.
- **FR-015**: Continuity preview and evidence MUST be tenant-scoped and permission-gated according to the thread/session inspection rules from Roadmap 54.
- **FR-016**: Unauthorized users MUST be denied continuity inspection without revealing inaccessible thread existence, source identity, included content, excluded content, or runtime evidence.
- **FR-017**: The system MUST redact or suppress secrets, raw provider payloads, disallowed message bodies, unsafe channel metadata, and cross-tenant identifiers before exposing continuity previews or evidence.
- **FR-018**: If safe redaction cannot be guaranteed for a turn or artifact, the system MUST exclude unsafe detail from continuity preview, record a redaction-failure reason, and continue with only safe metadata where allowed.
- **FR-019**: Chat, Web, TUI, and supported channel paths that carry daemon-owned thread identity MUST apply the same continuity inclusion rules.
- **FR-020**: Source paths that cannot prove valid thread identity MUST NOT infer continuity from message text, user identity alone, channel-local state, prompt files, client state, or prior provider context.
- **FR-021**: Accepted channel messages for the same current tenant, connector, source account, and source conversation MUST attach to the daemon-owned current thread segment unless lifecycle state blocks continuation.
- **FR-022**: Duplicate, replayed, ignored, blocked, disabled, unsupported, or failed source messages MUST NOT create duplicate or misleading continuity turns.
- **FR-023**: Continuity turn records, daemon acceptance sequence, inclusion decisions, reset boundaries, exclusion reasons, and preview evidence MUST survive daemon restart.
- **FR-024**: Restart recovery MUST preserve enough evidence for operators to determine whether a turn was included, excluded, pending, duplicated, or unsafe for continuity.
- **FR-025**: Legacy or partial thread/session evidence MUST remain inspectable where allowed but MUST NOT be included in continuity unless ordering, tenant, source, reset segment, and redaction eligibility can be proven.
- **FR-026**: This phase MUST NOT create memory writes, memory recall, semantic retrieval, knowledge graph behavior, summaries, autonomous context packing, long-term preference learning, or cross-thread personalization.
- **FR-027**: Continuity documentation and operator-facing evidence MUST make the difference between bounded recent-turn continuity and knowledge-plane memory explicit.
- **FR-028**: Continuity evidence MUST follow the Roadmap 54 default 90-day inspection retention period unless an authorized tenant retention policy requires longer retention.
- **FR-029**: Expired continuity evidence MUST NOT be available for normal continuity inclusion or normal inspection after its retention limit.
- **FR-030**: The system MUST provide testable failure classifications for continuity unavailable, no eligible turns, lifecycle blocked, reset boundary, source mismatch, permission denied, redaction failure, retention expired, duplicate source event, and incomplete evidence.
- **FR-031**: Existing single-turn behavior MUST remain available when no valid thread identity or no eligible recent turns exist.
- **FR-032**: Bounded continuity assembly for the default continuity window MUST meet a p95 latency target under 500 ms in the verification environment.

### Key Entities *(include if feature involves data)*

- **Continuity Turn**: A persisted user or assistant turn that may be eligible for future recent-thread continuity, including tenant, thread, session segment, source, daemon acceptance sequence, source timestamp where available, actor role, response relationship, and runtime evidence references.
- **Continuity Window**: The deterministic set of most recent eligible prior turns considered for one response, ordered by daemon acceptance sequence and bounded by count, age, reset segment, permission, retention, source identity, and redaction rules.
- **Reset Boundary**: The lifecycle point after which future continuity begins fresh within the same thread; turns before the boundary remain historical evidence but are excluded from future inclusion.
- **Continuity Preview**: Operator-visible evidence explaining included turn references, excluded turn references, source summaries, order, policy limits, reset segment, and safe runtime artifact references.
- **Runtime Artifact Reference**: A stable reference from a continuity turn to directly related runs, workflows, approvals, tool outcomes, replies, deliveries, files, or artifacts. Artifact content may contribute to continuity only as a user-visible, safely redacted excerpt tied to an included turn; otherwise the artifact remains reference-only evidence.
- **Continuity Exclusion Reason**: A stable classification explaining why a prior turn or artifact was not included, such as over limit, too old, reset boundary, lifecycle block, source mismatch, permission denied, redaction failure, duplicate event, incomplete evidence, or retention expired.
- **Unsupported Continuity Source**: A user, client, connector, or workflow source that cannot prove daemon-owned thread identity and therefore cannot receive implicit recent-turn continuity.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Adds bounded continuity records, inclusion decisions, and operator-visible continuity evidence to daemon-owned thread/session behavior. Existing single-turn chat and channel behavior must remain compatible when callers lack valid thread identity or when no eligible recent turns exist.
- **Migration / Rollback**: Rollout can begin with continuity disabled or read-only evidence collection, then enable inclusion for verified thread sources. Legacy sessions may be labeled as partial evidence and excluded from inclusion until eligibility is provable. Rollback disables continuity inclusion while preserving already-recorded turns, reset boundaries, and evidence for authorized inspection.
- **Verification Strategy**: Required validation includes recent follow-up behavior, over-limit exclusion, reset exclusion, archive/reopen boundaries, Web/TUI/channel parity, duplicate source handling, restart recovery, legacy partial evidence exclusion, permission denial, redaction failure, retention expiry, default-window latency, and explicit non-use of memory, semantic retrieval, summaries, or cross-thread personalization.
- **Observability Impact**: Operators must gain continuity previews, included and excluded turn references, deterministic policy details, reset-boundary evidence, source mismatch reasons, duplicate detection evidence, redaction failures, retention expiry reasons, and restart recovery classifications without relying on raw logs or hidden prompt assembly.
- **Environment & Secrets**: Development and automated verification must default to the repository test environment. Live connectors and production tenants must not be touched by default. Any live connector validation must use explicitly approved tenant-owned test accounts and must not expose secrets, tokens, raw provider payloads, disallowed message bodies, or cross-tenant identifiers.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of primary follow-up tests in active threads show the assistant can use the immediate prior exchange without requiring the user to restate it.
- **SC-002**: 100% of over-limit continuity tests include no more than 12 eligible prior turns and expose deterministic exclusion reasons for older eligible turns.
- **SC-003**: 100% of age-boundary tests exclude turns older than the active continuity window unless an authorized tenant policy allows otherwise.
- **SC-004**: 100% of reset tests prove pre-reset turns and artifacts do not affect the next response while remaining inspectable as historical evidence where allowed.
- **SC-005**: 100% of archive and reopen tests prove continuity is blocked while archived and resumes only from the eligible current continuation segment after reopen.
- **SC-006**: 100% of Web, TUI, and supported channel parity tests apply the same inclusion limits, reset boundaries, and evidence classifications for equivalent thread states.
- **SC-007**: 100% of source identity tests prevent continuity when valid daemon-owned thread identity cannot be proven.
- **SC-008**: 100% of duplicate, replayed, ignored, blocked, disabled, unsupported, and failed source-message tests avoid duplicate or misleading continuity turns.
- **SC-008a**: 100% of concurrent and replayed message tests apply daemon acceptance sequence as continuity order while preserving source timestamp evidence where available.
- **SC-009**: 100% of inspected responses in the verification dataset expose included turn references, exclusion reasons, reset segment, and inclusion policy to authorized operators.
- **SC-010**: Authorized operators can determine why a representative response did or did not use recent-turn continuity within 5 minutes using product evidence.
- **SC-011**: 100% of permission-denial tests prevent unauthorized continuity inspection without exposing inaccessible thread existence, source identity, content, or runtime evidence.
- **SC-012**: Redaction validation finds zero exposed secrets, raw provider payloads, disallowed message bodies, unsafe channel metadata, or cross-tenant identifiers in continuity previews, tests, logs, fixtures, and audit output.
- **SC-013**: 100% of restart recovery tests preserve accepted continuity turns, daemon acceptance sequence, inclusion decisions, reset boundaries, and preview evidence for active, reset, archived, and reopened threads.
- **SC-014**: 100% of legacy or partial-evidence cases remain inspectable where allowed but are excluded from continuity unless eligibility can be proven.
- **SC-015**: Verification confirms this phase creates no memory writes, memory recall, semantic retrieval, knowledge graph behavior, summaries, autonomous context packing, long-term preference learning, or cross-thread personalization in 100% of covered flows.
- **SC-016**: 100% of retention tests prove continuity evidence expires from normal inclusion and normal inspection after the applicable retention limit.
- **SC-017**: 100% of runtime artifact tests prove only user-visible, safely redacted excerpts tied to included turns can contribute content to continuity input.
- **SC-018**: Performance validation shows p95 continuity assembly completes under 500 ms for the default continuity window in the verification environment.

## Assumptions

- Roadmap 54 provides tenant-scoped thread identity, session segments, lifecycle state, reset/archive/reopen behavior, permissions, source linkage, runtime evidence projections, and the 90-day default inspection retention model.
- Thread/session lifecycle inspection remains permission-gated by the Roadmap 54 inspection boundary; this phase does not define a new tenant access model.
- The default recent-turn inclusion policy is 12 prior turns and 30 active-continuity days unless an authorized tenant policy narrows or extends the active window.
- "Recent turns" means explicit user and assistant turns in the current thread segment, not summaries, embeddings, memory records, provider-side retained context, or client-local history.
- Runtime artifact content is eligible only as user-visible, safely redacted excerpts directly linked to included turns; all other runtime artifacts remain reference-only evidence where allowed.
- Reset is the only user-facing continuity reset boundary in this phase; it preserves thread identity and historical evidence while excluding pre-reset turns from future continuity.
- Supported channel continuity requires daemon-owned thread identity from tenant, connector, source account, and source conversation linkage.
- Existing Web and TUI clients may need user-visible thread identity support before they can fully demonstrate multi-turn continuity, but single-turn operation must continue when thread identity is absent.
- Automated verification uses fake or test-environment channel evidence by default. Live connector validation is optional unless a later release-readiness gate explicitly requires approved safe accounts.
