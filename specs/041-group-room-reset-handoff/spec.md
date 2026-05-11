# Feature Specification: Group Room Reset Handoff

**Feature Branch**: `041-group-room-reset-handoff`  
**Created**: 2026-05-11  
**Status**: Draft  
**Input**: User description: "$speckit-specify 结合 docs/specs/041-group-room-reset-and-handoff-semantics.md 完成 phase 56 的工作"

**Upstream authority**: `docs/specs/041-group-room-reset-and-handoff-semantics.md` is the authoritative upstream document for this work (Roadmap 56). This specification translates that document into testable scenarios, requirements, and success criteria. Where the upstream document and this spec disagree, the upstream document wins and this spec must be updated.

**Dependency**: Roadmap 54 (`specs/039-thread-session-lifecycle/spec.md`) provides daemon-owned thread and session lifecycle truth. Roadmap 55 (`specs/040-multi-turn-continuity/spec.md`) provides bounded recent-turn continuity without memory or semantic recall. This phase defines group, room, reset, and cross-surface handoff semantics on top of those foundations.

## Clarifications

### Session 2026-05-11

- Q: Should handoff merge source and destination surfaces into one daemon thread or link separate destination threads? -> A: Handoff creates or selects a separate destination thread linked to the source by a handoff record.
- Q: What default policy controls group or room participation? -> A: Default group or room participation requires both allowlist eligibility and a qualifying mention.
- Q: What source context is available to a handoff destination? -> A: Handoff makes eligible current-segment source turns available to the destination by reference, subject to permission and redaction.
- Q: Which permission gates reset and handoff creation? -> A: `connectors.manage` gates reset and handoff creation.
- Q: How long may handoff source-turn references influence the destination? -> A: Source-turn references are available only for the first destination response after handoff.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Distinguish Direct, Group, Room, and Web Threads (Priority: P1)

As a user or operator, I can rely on every accepted conversation to carry an explicit conversation shape so direct messages, group conversations, rooms, and web-originated conversations are not confused with each other.

**Why this priority**: Group and room behavior cannot be safe or predictable unless the system first represents the conversation shape as product state instead of inferring it from message text or connector-local details.

**Independent Test**: Can be tested by creating direct-message, group, room, and web-originated conversations and verifying that each accepted thread shows the correct shape, source identity, current lifecycle segment, and isolation boundary to authorized users.

**Acceptance Scenarios**:

1. **Given** a direct-message conversation and a group conversation exist for the same tenant user, **When** each conversation sends a message, **Then** the accepted thread for each message preserves its own conversation shape and does not share group or direct-message state.
2. **Given** two rooms from the same connector have similar names or participants, **When** messages arrive in both rooms, **Then** each room maps to its own current thread boundary with stable source evidence.
3. **Given** a web-originated thread exists for the same user as a channel thread, **When** the user continues either conversation without handoff, **Then** the system treats them as separate conversations.

---

### User Story 2 - Honor Group Participation Policy (Priority: P1)

As a tenant user in a group or room, I can predict when the assistant will participate based on mention and allowlist policy, and operators can inspect the policy that controlled the routing decision.

**Why this priority**: Public and shared spaces need clear participation rules. Without policy-linked routing, the assistant can respond unexpectedly, ignore valid mentions, or cross tenant and connector boundaries.

**Independent Test**: Can be tested by sending group and room messages with allowed mentions, missing mentions, disabled participation, unsupported source identities, and permission changes, then verifying accepted, ignored, or blocked outcomes and operator-visible reasons.

**Acceptance Scenarios**:

1. **Given** a group thread is allowlist-eligible and requires mention-based participation, **When** a user sends a message that mentions the assistant and satisfies tenant policy, **Then** the message is accepted into the correct group thread.
2. **Given** the same group thread is allowlist-eligible but requires mention-based participation, **When** a user sends a message without a qualifying mention, **Then** the assistant does not participate and authorized inspection shows the missing-mention routing reason.
3. **Given** a room or group is not allowlist-eligible for assistant participation, **When** a user attempts to invoke the assistant there even with a qualifying mention, **Then** the system blocks participation without creating misleading thread continuity.

---

### User Story 3 - Reset Direct, Group, and Room Threads Without Deleting Evidence (Priority: P1)

As a user with permission to reset a conversation, I can reset a direct-message, group, or room thread so future responses start fresh while historical evidence remains available for authorized inspection.

**Why this priority**: Reset is the user-visible safety boundary for conversation state. It must be durable, auditable, and source-specific rather than a prompt instruction that can be ignored or misinterpreted.

**Independent Test**: Can be tested by creating direct-message, group, and room threads with prior turns, resetting each source-specific thread, asking a follow-up that would require pre-reset context, and verifying future continuity excludes pre-reset content while authorized inspection still shows the reset event and historical evidence.

**Acceptance Scenarios**:

1. **Given** a direct-message thread has prior eligible turns, **When** an authorized user resets that thread, **Then** future responses in that direct-message thread exclude pre-reset turns and show a reset boundary.
2. **Given** a group or room thread has prior eligible turns, **When** an authorized user resets that group or room thread, **Then** future responses in that same source start after the reset boundary without resetting unrelated direct-message or web threads.
3. **Given** a reset request is made by a user without reset permission, **When** the request is evaluated, **Then** the reset is denied without exposing inaccessible historical thread details.

---

### User Story 4 - Hand Off Conversations Across Surfaces With Traceability (Priority: P1)

As a user, I can continue a supported channel conversation in the web shell, or hand off a web shell thread to a supported channel, with visible linkage between the source and destination surfaces.

**Why this priority**: Cross-surface continuation is useful only if users and operators can understand where a conversation came from, where it moved, and why the destination was allowed to continue it.

**Independent Test**: Can be tested by handing off a channel thread to the web shell and a web-originated thread to a supported channel, then verifying the destination thread is separate from the source thread, both sides are linked by a handoff record where visible, permissions are enforced, the first destination response may reference only eligible current-segment source turns, and later destination responses do not reuse source-turn references unless another handoff occurs.

**Acceptance Scenarios**:

1. **Given** a user has a supported channel thread with eligible current-segment turns, **When** the user opens it in the web shell through handoff, **Then** the web shell shows a separate traceable destination conversation linked to the source thread and the first destination response may reference eligible source turns without copying them.
2. **Given** a web-originated thread is handed off to a supported channel, **When** the destination channel accepts the handoff, **Then** the channel conversation uses a separate destination thread, shows source linkage, can reference eligible source turns where allowed, and follows the destination channel participation policy.
3. **Given** the requester lacks lifecycle mutation permission or the destination surface lacks tenant, connector, participant, or source permission, **When** a handoff is requested, **Then** the handoff is denied and no destination conversation is silently created.

---

### User Story 5 - Inspect Reset and Handoff Events (Priority: P2)

As an operator, I can inspect reset events, participation decisions, and handoff links so I can explain conversation behavior without relying on raw logs or hidden prompt state.

**Why this priority**: Group, room, reset, and handoff behavior changes where conversations continue and what prior turns can influence them. Operators need product evidence to debug and support these outcomes.

**Independent Test**: Can be tested by producing accepted, ignored, reset, denied-reset, successful-handoff, denied-handoff, and unsupported-source cases, then verifying authorized inspection shows stable event references, source summaries, actor information, policy outcome, and safe evidence.

**Acceptance Scenarios**:

1. **Given** a reset occurs in a direct-message, group, or room thread, **When** an authorized operator inspects the thread, **Then** the reset event shows actor, source, time, affected thread boundary, and post-reset segment.
2. **Given** a handoff succeeds, **When** an authorized operator inspects either side of the handoff, **Then** the source and destination references are visible according to tenant and connector permissions.
3. **Given** a participation or handoff decision is denied, ignored, or unsupported, **When** an authorized operator inspects the event, **Then** the operator sees a stable reason classification without unsafe message bodies or inaccessible source details.

---

### User Story 6 - Preserve Non-Memory Scope (Priority: P2)

As a tenant user or support operator, I can trust that group, room, reset, and handoff behavior does not create group memory, team knowledge behavior, semantic recall, or autonomous delegation.

**Why this priority**: This phase defines routing and lifecycle semantics only. It must not blur into memory or knowledge-plane behavior before those capabilities have their own safety, permission, and operator evidence model.

**Independent Test**: Can be tested by configuring related older conversations, unrelated groups, and repeated preferences, then verifying reset, handoff, and group participation never use those materials unless they are eligible current-segment turns from an allowed source.

**Acceptance Scenarios**:

1. **Given** a group has older unrelated history or repeated preferences, **When** a user starts or hands off a conversation, **Then** the assistant does not use that history as group memory.
2. **Given** a thread is reset before handoff, **When** the destination conversation continues, **Then** pre-reset turns are not available to the destination by reference and do not influence the destination response.
3. **Given** another group or room has related content, **When** a user continues the current thread, **Then** the response does not use cross-room semantic recall or team knowledge behavior from this phase.

### Edge Cases

- A connector cannot prove whether a source conversation is a direct message, group, room, or web-originated thread; the system must avoid implicit group or handoff behavior and expose an inspectable unsupported-source outcome.
- A group and a direct-message conversation share the same participant set; the system must keep their lifecycle, reset, and handoff boundaries separate.
- A group is renamed, archived, deleted, or recreated while retaining similar participants; stable source identity, not display name, must determine continuity and reset scope.
- A user leaves or joins a group after a handoff or reset; future participation and inspection must reflect current permission while preserving historical event evidence where allowed.
- A mention is edited, deleted, duplicated, or replayed by a connector; participation must not create duplicate or misleading accepted turns.
- A reset happens during active message acceptance; the next response must use one auditable reset boundary and must not mix pre-reset and post-reset turns.
- A reset request targets the wrong conversation shape or source; the system must deny or scope the reset without affecting unrelated conversations.
- A handoff is requested from a reset, archived, unsupported, blocked, or permission-denied thread; the destination must not silently continue hidden state.
- A handoff source has eligible and ineligible turns in the current segment; only eligible, safely redacted, permission-allowed source turns may be referenced by the destination.
- A destination thread continues after its first post-handoff response; subsequent responses must use destination-thread continuity only unless another authorized handoff occurs.
- A handoff is requested more than once from the same source thread; each separate destination thread must remain traceable and not overwrite prior handoff evidence.
- A destination surface accepts a handoff but later loses permission; future continuation and inspection must honor the later permission state.
- Operator inspection is requested by a user who can see one side of a handoff but not the other; inaccessible source or destination detail must be suppressed.
- Live connector evidence contains secrets, raw provider payloads, disallowed message bodies, unsafe channel metadata, or cross-tenant identifiers; inspection must expose only safe summaries and stable classifications.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST explicitly distinguish direct-message, group, room, and web-originated thread shapes for accepted conversations.
- **FR-002**: The system MUST preserve tenant, connector, source account, source conversation, participant context, and conversation shape evidence for every supported group, room, direct-message, and web-originated thread.
- **FR-003**: Group and room thread state MUST remain isolated from direct-message, web-originated, and unrelated group or room thread state even when users, labels, or message text overlap.
- **FR-004**: Group and room participation MUST be governed by explicit mention, allowlist, and tenant permission policy.
- **FR-004a**: Default group and room participation MUST require both allowlist eligibility and a qualifying mention before assistant work is accepted.
- **FR-005**: The system MUST record accepted, ignored, blocked, denied, duplicate, unsupported, and failed group participation outcomes with stable reason classifications.
- **FR-006**: Messages that do not satisfy group or room participation policy MUST NOT create assistant turns, handoff links, or continuity evidence that implies participation occurred.
- **FR-007**: Reset MUST be treated as a durable lifecycle operation for a specific direct-message, group, room, or web-originated thread boundary.
- **FR-008**: Reset MUST preserve historical evidence according to permission, redaction, and retention rules while excluding pre-reset turns from future current-segment continuity.
- **FR-009**: Reset of one conversation shape or source MUST NOT reset unrelated direct-message, group, room, web-originated, or handoff destination threads.
- **FR-010**: Reset requests MUST require the lifecycle mutation permission, `connectors.manage`, and MUST be denied safely when the requester lacks permission.
- **FR-011**: Reset events MUST record actor, source, time, affected thread boundary, previous segment, new segment, and permission outcome for authorized inspection.
- **FR-012**: Handoff MUST create or select a separate destination thread and MUST link it traceably to the source thread by a handoff record.
- **FR-013**: Handoff linkage MUST include stable source thread reference, destination thread reference, actor, time, tenant context, source conversation shape, destination conversation shape, and permission outcome.
- **FR-013a**: Handoff MUST NOT merge source and destination surfaces into the same daemon-owned thread identity.
- **FR-014**: Handoff MUST preserve current-segment reset boundaries and MUST NOT allow pre-reset turns to affect the destination conversation.
- **FR-014a**: Handoff MAY make eligible current-segment source turns available to the separate destination thread by reference for the first destination response after handoff, subject to source permission, destination permission, redaction, retention, and continuity eligibility.
- **FR-014b**: Handoff MUST NOT copy source turns into destination thread history as new destination turns.
- **FR-014c**: After the first destination response following handoff, the destination thread MUST use its own destination-thread continuity only unless another authorized handoff occurs.
- **FR-015**: Handoff MUST NOT bypass tenant, connector, source, participant, or destination-surface permissions.
- **FR-015a**: Handoff creation MUST require the lifecycle mutation permission, `connectors.manage`, and MUST be denied safely when the requester lacks permission.
- **FR-016**: Handoff MUST be denied safely when either source or destination cannot prove required identity, permission, or lifecycle eligibility.
- **FR-017**: The destination conversation MUST follow its own participation policy after handoff, including mention and allowlist requirements for group and room destinations.
- **FR-018**: Authorized users MUST be able to inspect reset events, participation decisions, and handoff links from thread detail or equivalent operator-visible evidence.
- **FR-019**: Inspection evidence MUST show included source and destination references, safe source summaries, policy outcomes, reset boundaries, handoff status, and reason classifications.
- **FR-020**: Unauthorized inspection MUST NOT reveal inaccessible thread existence, source identity, participant identity, message content, handoff destination, reset history, or runtime evidence.
- **FR-021**: The system MUST redact or suppress secrets, raw provider payloads, disallowed message bodies, unsafe connector metadata, and cross-tenant identifiers before exposing group, reset, or handoff evidence.
- **FR-022**: If safe redaction cannot be guaranteed, the system MUST expose only safe metadata and a redaction-limited classification where allowed.
- **FR-023**: Duplicate, replayed, edited, deleted, ignored, blocked, disabled, unsupported, or failed source messages MUST NOT create duplicate or misleading group participation, reset, or handoff records.
- **FR-024**: Group, room, reset, participation, and handoff records MUST survive daemon restart with enough evidence for authorized users to determine final status.
- **FR-025**: Existing single-surface and single-turn behavior MUST remain available when no valid group, room, reset, or handoff semantics apply.
- **FR-026**: Existing supported connector routing MUST remain backward compatible unless a connector opts into the new conformance requirements for group, room, reset, or handoff behavior.
- **FR-027**: Connector conformance MUST define testable evidence for conversation shape, source identity, mention participation, duplicate detection, reset support, handoff support, and safe inspection behavior where the connector claims support.
- **FR-028**: This phase MUST NOT create group memory, team knowledge base behavior, autonomous agent-to-agent delegation, semantic retrieval, summaries, long-term personalization, or cross-room recall.
- **FR-029**: Documentation and operator-facing evidence MUST make the difference between lifecycle handoff or reset and knowledge-plane memory explicit.
- **FR-030**: Automated verification MUST cover direct-message reset, group reset, room reset, web-to-channel handoff, channel-to-web handoff, policy-denied participation, permission-denied reset, permission-denied handoff, duplicate source events, restart recovery, redaction, and explicit non-use of memory.

### Key Entities *(include if feature involves data)*

- **Conversation Shape**: The explicit product classification for a conversation, such as direct message, group, room, or web-originated thread.
- **Room Identity**: Stable source evidence that distinguishes one shared conversation space from another, independent of display name or participant overlap.
- **Participation Policy**: Tenant and source-specific rules that determine whether the assistant may participate in a group or room, requiring both allowlist eligibility and a qualifying mention by default.
- **Participation Decision**: An inspectable outcome for a group or room message, such as accepted, ignored, blocked, denied, duplicate, unsupported, or failed.
- **Reset Event**: A lifecycle event that starts a fresh current segment for a specific thread boundary while preserving historical evidence where allowed.
- **Handoff Link**: A traceable relationship between a source thread and a separate destination thread, including actor, time, source, destination, status, permission outcome, and allowed source-turn references.
- **Handoff Destination**: The separate destination thread and conversation surface that receives a handoff, can reference eligible current-segment source turns for the first destination response where allowed, and applies its own lifecycle, participation, permission, and reset rules.
- **Inspection Evidence**: Operator-visible, permission-gated evidence explaining group participation, reset, and handoff behavior using safe summaries and stable classifications.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Adds explicit conversation-shape, group participation, reset, and handoff semantics to daemon-owned thread behavior. Existing direct-message, web, and connector flows must continue to work when a connector or client does not claim group, room, reset, or handoff support.
- **Migration / Rollback**: Rollout can begin with read-only classification and inspection evidence, then enable reset and handoff actions for verified sources. Legacy conversations may remain inspectable with partial evidence but must not receive implicit group or handoff semantics unless eligibility is provable. Rollback disables new reset and handoff actions while preserving already-recorded evidence for authorized inspection.
- **Verification Strategy**: Required validation includes direct-message and group reset, room isolation, mention and allowlist routing, channel-to-web handoff, web-to-channel handoff, denied handoff, permission denial, duplicate source handling, restart recovery, redaction, connector conformance, and explicit non-use of memory or semantic continuity.
- **Observability Impact**: Operators must gain product-visible reset events, handoff links, participation decisions, source and destination summaries, policy outcomes, permission outcomes, duplicate classifications, redaction-limited classifications, and restart recovery status without relying on raw logs.
- **Environment & Secrets**: Development and automated verification must default to the repository test environment. Live connector validation is optional and must use explicitly approved tenant-owned test accounts. Secrets, tokens, raw provider payloads, disallowed message bodies, and cross-tenant identifiers must not be exposed in tests, fixtures, logs, or inspection output.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of direct-message, group, room, and web-originated test conversations expose the correct conversation shape and source isolation to authorized inspection.
- **SC-002**: 100% of group and room participation tests produce the expected accepted, ignored, blocked, denied, duplicate, unsupported, or failed outcome with a stable reason classification, including missing-mention and not-allowlisted cases.
- **SC-003**: 100% of direct-message, group, and room reset tests prove pre-reset turns do not affect the next response while historical evidence remains inspectable where allowed.
- **SC-004**: 100% of reset scoping tests prove resetting one source or conversation shape does not reset unrelated direct-message, group, room, web-originated, or handoff destination threads.
- **SC-005**: 100% of channel-to-web and web-to-channel handoff tests create traceable source and destination references visible to authorized users.
- **SC-006**: 100% of denied handoff tests prevent destination conversation creation or continuation when tenant, connector, participant, lifecycle mutation, or destination permission is missing.
- **SC-007**: 100% of handoff-after-reset tests prove pre-reset turns remain excluded from destination continuity.
- **SC-007a**: 100% of handoff context tests prove eligible source turns are referenced rather than copied into destination history, ineligible source turns are unavailable to the destination, and source-turn references are used only for the first destination response after handoff.
- **SC-008**: 100% of connector conformance tests for connectors claiming support validate conversation shape, source identity, participation policy, duplicate detection, reset behavior, handoff behavior, and safe inspection evidence.
- **SC-009**: 100% of duplicate, replayed, edited, deleted, ignored, blocked, disabled, unsupported, and failed source-message tests avoid duplicate or misleading participation, reset, and handoff records.
- **SC-010**: 100% of restart recovery tests preserve reset events, handoff links, participation decisions, policy outcomes, and final statuses for authorized inspection.
- **SC-011**: Authorized operators can determine why a representative group message participated, did not participate, reset, or handed off within 5 minutes using product evidence.
- **SC-012**: 100% of permission-denial tests prevent unauthorized users from discovering inaccessible thread existence, source identity, participant identity, message content, reset history, handoff destination, or runtime evidence.
- **SC-013**: Redaction validation finds zero exposed secrets, raw provider payloads, disallowed message bodies, unsafe connector metadata, or cross-tenant identifiers in inspection output, tests, fixtures, and logs.
- **SC-014**: Verification confirms this phase creates no group memory, team knowledge base behavior, autonomous delegation, semantic retrieval, summaries, long-term personalization, or cross-room recall in 100% of covered flows.

## Assumptions

- Roadmap 54 provides tenant-scoped thread identity, lifecycle state, session segments, reset/archive/reopen behavior, source linkage, permissions, runtime evidence projections, and the default inspection retention model.
- Roadmap 55 provides bounded recent-turn continuity and reset-boundary behavior without memory, semantic retrieval, summaries, or long-term personalization.
- Reset and handoff creation are lifecycle or routing mutations and use the existing `connectors.manage` permission boundary.
- Group and room support is additive for connectors and clients; unsupported sources must remain safe and inspectable without receiving implicit group or handoff semantics.
- Default group and room participation requires both allowlist eligibility and a qualifying mention; broader room-level participation requires an explicit future policy decision outside this default.
- Reset remains a lifecycle operation scoped to a specific thread boundary and does not delete historical evidence.
- Handoff creates traceable linkage between a source thread and a separate destination thread and may reference eligible current-segment source turns for the first destination response only, but does not copy source turns, merge thread identities, merge unrelated source histories, or bypass destination participation policy.
- Automated verification uses fake or test-environment connector evidence by default. Live connector validation is optional unless a later release-readiness gate explicitly requires approved safe accounts.
