# Feature Specification: Channel Connector Conformance

**Feature Branch**: `033-channel-connector-conformance`  
**Created**: 2026-05-07  
**Status**: Draft  
**Input**: User description: "$speckit-specify 结合 docs/specs/033-channel-connector-conformance-contract.md 完成 phase 48 的工作"

**Upstream authority**: `docs/specs/033-channel-connector-conformance-contract.md` is the authoritative design document for this work (Roadmap 48). This specification translates that design into testable scenarios, requirements, and success criteria. Where the upstream document and this spec disagree, the upstream document wins and this spec must be updated.

## Clarifications

### Session 2026-05-07

- Q: What connector coverage must Phase 48 require? → A: Shared fake connector conformance tests plus Discord regression only.
- Q: How should required versus unsupported connector capabilities be interpreted? → A: Core invariants must pass; provider-specific surfaces may be unsupported or limited.
- Q: What freshness rule should connector diagnostic state use? → A: Cached connector diagnostics may be shown but become stale after 15 minutes.
- Q: How long should connector conformance and diagnostic evidence be retained by default? → A: 90 days by default.
- Q: What identity fields are required for inbound message dedupe? → A: Tenant, connector account, channel or conversation, and provider message ID.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Prove Any Hosted Connector Against One Contract (Priority: P1)

As an engineer adding or maintaining a hosted channel connector, I can run one shared conformance matrix and know whether the connector satisfies required tenant scope, account binding, inbound routing, dedupe, reply progression, diagnostics, and delivery-boundary behavior.

**Why this priority**: Phase 48 only closes if future channel work can depend on one stable connector contract instead of re-deciding routing, retry, redaction, and failure semantics for every provider.

**Independent Test**: Can be tested by running the conformance matrix against fake connectors that support required behavior and deliberately unsupported capabilities, plus Discord regression coverage as the only required real connector baseline for this phase.

**Acceptance Scenarios**:

1. **Given** a connector declares required account, routing, identity, reply, diagnostic, and delivery-boundary capabilities, **When** the conformance matrix runs, **Then** each required capability is evaluated with a pass, fail, or explicit unsupported result and no required area is silently skipped.
2. **Given** a connector cannot support a provider-specific channel surface such as incremental visible updates, rich media, rooms, or thread behavior, **When** the conformance matrix runs, **Then** the connector can still pass hosted readiness if it satisfies all core invariants and declares the unsupported or limited surface explicitly.
3. **Given** a connector violates tenant scope, dedupe, routing, or redaction rules, **When** conformance results are reviewed, **Then** the result identifies the failing contract area and blocks the connector from being treated as hosted-ready.

---

### User Story 2 - Receive Predictable Channel Behavior (Priority: P1)

As a user interacting through a direct message, group, mention, room, or thread, I receive consistent routing, duplicate handling, blocked-channel behavior, and reply progression across Discord, Telegram, Slack, and future hosted connectors.

**Why this priority**: Users experience inconsistent connector behavior as product failure. The contract must make ordinary channel differences explicit without letting each connector invent its own conversation semantics.

**Independent Test**: Can be tested by replaying representative direct message, group, mention, blocked room, thread, duplicate, and retry scenarios through each connector under test and confirming each produces the same user-visible meaning.

**Acceptance Scenarios**:

1. **Given** a user sends an accepted direct message through a conforming connector, **When** the message is handled, **Then** the system routes it to the correct tenant-owned conversation path and returns the reply through the same foreground channel without using background delivery as a substitute.
2. **Given** a group, room, or thread message requires an allowlist, mention, or room permission and the message does not meet that rule, **When** the connector receives it, **Then** the message is rejected or ignored using a stable blocked-channel outcome and no assistant reply is sent.
3. **Given** the same inbound channel message is delivered more than once because of provider retry or restart recovery, **When** the connector handles the duplicate, **Then** the user receives at most one assistant reply and operators can inspect the dedupe outcome.

---

### User Story 3 - Diagnose Connector Readiness And Failures (Priority: P2)

As an operator, I can inspect connector lifecycle, health, permissions, rate limits, provider availability, network reachability, and reply failures using stable diagnostic states that do not expose secret or cross-tenant data.

**Why this priority**: Hosted connectors carry external account bindings and provider failure modes. Operators need actionable failure visibility without raw provider errors, tokens, or tenant leaks.

**Independent Test**: Can be tested by seeding or replaying auth missing, permission missing, rate-limited, provider unavailable, network failed, unsupported capability, and reply-failed cases for one tenant and confirming authorized operators see stable redacted diagnostics.

**Acceptance Scenarios**:

1. **Given** connector authorization is missing or revoked, **When** an authorized operator inspects connector health, **Then** the operator sees a stable auth-missing diagnostic, remediation owner, timestamp, freshness state, and redacted account-binding metadata.
2. **Given** a connector cannot send a reply because provider permission is missing, rate limits are active, the provider is unavailable, or the network failed, **When** the failure is shown, **Then** the diagnostic classification distinguishes these causes without exposing raw provider secrets or another tenant's connector existence.
3. **Given** an operator lacks permission for a tenant, **When** the operator requests connector diagnostics for that tenant, **Then** access is denied without revealing tenant, connector, account, secret, or message details.

---

### User Story 4 - Preserve Reply And Delivery Boundaries (Priority: P2)

As a platform engineer or release reviewer, I can confirm that foreground replies and background delivery remain separate product truths even when both reuse the same connector transport mechanics.

**Why this priority**: Previous roadmap decisions require active channel replies and background notifications to stay distinct. Phase 48 must prevent a connector from passing by treating one successful reply as proof of background delivery, or vice versa.

**Independent Test**: Can be tested by running foreground reply scenarios and background result-delivery scenarios through the same connector transport and confirming each records its own outcome, failure state, and operator-visible evidence.

**Acceptance Scenarios**:

1. **Given** an accepted foreground channel message completes successfully, **When** the reply is sent, **Then** the foreground reply outcome is recorded separately from any background delivery target or notification outcome.
2. **Given** background work emits a result through a connector-backed delivery target, **When** delivery succeeds or fails, **Then** the delivery outcome is recorded separately from any active conversation reply state.
3. **Given** a connector can show thinking or incremental reply progress, **When** background delivery uses the same channel mechanics, **Then** the background delivery path does not inherit foreground progression semantics unless explicitly supported by the delivery contract.

### Edge Cases

- A provider redelivers the same inbound event with changed transport metadata; dedupe must use stable channel message identity and avoid a second assistant reply.
- A connector receives an inbound event without tenant, connector account, channel or conversation, and provider message ID; the event must fail conformance unless the connector provides an equivalent durable identity rule.
- A direct message, group mention, room message, or thread reply arrives for a tenant that has not bound the connector account; the event must fail closed without falling back to another tenant's binding.
- A group or room message includes bot mention syntax that should not be passed to the assistant; the contract must require normalized user intent without connector-specific mention artifacts.
- A connector supports final-only replies but not thinking or incremental visible updates; this is a valid degradation path when the unsupported capabilities are explicit.
- A connector claims incremental reply support but cannot throttle visible updates safely; it must degrade to a safer reply progression level rather than producing excessive edits.
- A foreground reply fails after the assistant work completed; operators must see reply failure separately from execution success.
- A background delivery target reuses a connector account that is disabled, disconnected, or permission-blocked; delivery must record its own retry, suppression, or failure outcome.
- Diagnostic evidence includes provider request identifiers or localized error text; useful non-secret context may be retained only when redaction is reliable.
- Diagnostic evidence cannot be confidently redacted; the product must suppress detailed evidence and show only a safe generic classification.
- Cached connector diagnostic state is older than 15 minutes; inspection surfaces may show it only when marked stale, and connector actions that fail must produce current diagnostic truth before presenting remediation.
- Connector conformance or diagnostic evidence reaches its default retention limit; it must expire from normal inspection after 90 days unless a longer authorized retention policy applies.
- Tenant switching occurs while connector status or diagnostics are open; previous-tenant connector details must be cleared, hidden, or denied before the new tenant's data is shown.
- Existing Discord behavior that is not yet contract-compliant must either be adapted to the shared contract or declare an explicit unsupported capability with regression coverage.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST define one shared hosted channel connector conformance contract that applies to Discord, Telegram, Slack, and future hosted channel connectors before they are considered hosted-ready.
- **FR-002**: The conformance contract MUST define required connector lifecycle and health states, including configured, disabled, starting, healthy, degraded, failed, permission-blocked, rate-limited, and unsupported-capability states.
- **FR-003**: The conformance contract MUST require every hosted connector account binding and connector configuration to be tenant-owned, permission-gated, and inspectable only through redacted metadata.
- **FR-004**: The conformance contract MUST require connector runtime behavior to resolve inbound events, outbound foreground replies, diagnostics, and connector-backed delivery attempts only within the active tenant's account binding.
- **FR-005**: The conformance contract MUST require inbound message identity that includes tenant, connector account, channel or conversation, and provider message ID so provider retries, restart replays, and repeated delivery attempts can be deduplicated without sending duplicate assistant replies.
- **FR-005a**: A connector MAY use an equivalent durable identity rule only when provider mechanics cannot supply the standard identity fields and the conformance matrix proves the equivalent rule preserves tenant scope and duplicate suppression.
- **FR-006**: The conformance contract MUST define accepted, ignored, blocked, duplicate, unsupported, and failed inbound outcomes with stable meanings that are consistent across connector providers.
- **FR-007**: The conformance contract MUST define routing expectations for direct messages, groups, mentions, rooms, and threads, including how connector-specific source identifiers map to tenant-owned conversation routing without local routing inventions.
- **FR-008**: The conformance contract MUST require group, room, and thread gating decisions to be explicit, including allowlist, mention-required, direct-message-enabled, blocked-channel, and unsupported-room outcomes where applicable.
- **FR-009**: The conformance contract MUST require connector-specific mention or addressing artifacts to be normalized away before user intent is presented for assistant handling.
- **FR-010**: The conformance contract MUST define reply progression capability levels as final-only, thinking plus final, thinking plus incremental updates, and unsupported, with final-only delivery as the minimum required foreground reply behavior for accepted messages.
- **FR-011**: The conformance contract MUST require connectors that support visible thinking or incremental updates to declare safe update behavior and degrade to a safer level when the channel cannot support safe progression.
- **FR-012**: The conformance contract MUST require foreground reply progression to remain tied to daemon-owned conversation, run, and outcome truth rather than connector-local session semantics.
- **FR-013**: The conformance contract MUST require foreground reply outcomes and background delivery outcomes to remain separate, even when they reuse the same channel transport or account binding.
- **FR-014**: The conformance contract MUST require background delivery through connector-backed targets to report selected target, attempt outcome, retry or suppression state, and terminal failure separately from any foreground reply state.
- **FR-015**: The conformance contract MUST define required diagnostic classifications for auth missing, permission missing, rate limited, provider unavailable, network failed, unsupported capability, blocked route, duplicate inbound, reply failed, and unknown connector failure.
- **FR-016**: Connector diagnostics MUST identify remediation owner, user-visible severity, retry safety where relevant, timestamp, freshness, and redacted evidence status for every required diagnostic classification.
- **FR-016a**: Cached connector diagnostic state MAY be shown for inspection, but it MUST be marked stale after 15 minutes.
- **FR-016b**: Connector actions that fail MUST produce current diagnostic truth before presenting remediation, even when cached diagnostic state exists.
- **FR-017**: Connector diagnostics, logs, events, replay fixtures, evaluation artifacts, support output, and conformance results MUST redact tokens, secret values, authorization headers, credential-bearing payloads, external account secrets, and cross-tenant data.
- **FR-018**: If diagnostic or conformance evidence cannot be confidently redacted, the system MUST suppress detailed evidence, record a redaction-failure outcome for authorized operators, and show only a safe generic classification.
- **FR-018a**: Connector conformance results, diagnostic evidence, and redaction-failure outcomes MUST use a 90-day default retention period unless an authorized longer retention policy applies.
- **FR-019**: The conformance matrix MUST include positive and negative fake connector cases for tenant ownership, permission denial, account binding, inbound routing, group gating, message identity, durable dedupe, reply progression, diagnostics, redaction, and foreground/background delivery separation.
- **FR-020**: The conformance matrix MUST allow connector-specific provider surfaces to be declared as supported, unsupported, or limited without weakening core invariants.
- **FR-020a**: Core invariants MUST include tenant ownership, permission gating, redaction, active-tenant account binding, inbound identity, durable dedupe, stable routing decisions, minimum final-only foreground reply delivery for accepted messages, required diagnostic classifications, and foreground/background delivery separation.
- **FR-020b**: Provider-specific surfaces MAY be unsupported or limited only when the connector declares that status explicitly and the conformance result proves the unsupported surface does not bypass or weaken any core invariant.
- **FR-021**: Existing Discord connector behavior MUST be evaluated against the shared contract and either pass required conformance or record explicit unsupported or limited capabilities with regression coverage.
- **FR-022**: Future channel specifications MUST be able to reference this conformance contract for shared connector behavior and focus only on provider-specific mechanics and optional enhancements.
- **FR-023**: Phase 48 MUST NOT implement a new channel connector, mobile app nodes, voice or media workers, rich media completion, memory-based conversation continuity, or provider-specific channel behavior beyond the shared contract and conformance evidence.
- **FR-024**: Phase 48 MUST NOT require Telegram, Slack, or other non-Discord connector stubs or regressions; those providers consume the contract in later provider-specific phases.

### Key Entities

- **Hosted Channel Connector**: A tenant-owned channel integration that can receive inbound messages, send foreground replies, and optionally serve as a background delivery target while satisfying shared conformance rules.
- **Connector Account Binding**: The tenant-scoped relationship between a tenant and an external channel account, bot, workspace, room, or similar provider identity. It includes redacted ownership, permission, and lifecycle state.
- **Connector Capability Profile**: The declared support level for core invariants and provider-specific surfaces. Core invariants must pass; provider-specific surfaces may be supported, unsupported, or limited when explicit and covered by conformance evidence.
- **Inbound Channel Event**: A normalized user-originated channel event with stable provider identity, tenant binding, routing context, sender context, and redacted source metadata.
- **Message Identity**: The durable identity used to recognize duplicate inbound deliveries and retry replays for the same user-visible channel message. The standard identity is tenant, connector account, channel or conversation, and provider message ID; equivalent provider-specific rules must be explicit and conformance-tested.
- **Routing Decision**: The contract result that determines whether an inbound event is accepted, ignored, blocked, unsupported, duplicate, or failed, including the reason and tenant-owned conversation destination where accepted.
- **Reply Progression Level**: The visible foreground reply capability for a connector: final-only, thinking plus final, thinking plus incremental updates, or unsupported.
- **Connector Diagnostic State**: Redacted operator- and user-visible status for connector health, authorization, permissions, provider availability, rate limits, network reachability, reply failures, unsupported capabilities, remediation owner, timestamp, freshness, and retention expiry.
- **Foreground Reply Outcome**: The result of answering an active inbound channel message through the same conversation surface, recorded separately from background delivery.
- **Background Delivery Outcome**: The result of delivering background work through a configured target that may reuse connector transport mechanics but preserves separate target, attempt, retry, suppression, and terminal outcome truth.
- **Conformance Result**: Evidence produced by the shared conformance matrix that records pass, fail, unsupported, or limited outcomes for each required and optional connector contract area, including retention expiry.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: This phase defines additive connector contract, capability, diagnostic, event, schema, documentation, and conformance evidence expectations. Existing Discord behavior remains the compatibility baseline but must be adapted to shared contract meanings rather than treated as a special case.
- **Migration / Rollback**: No user data migration or new connector rollout is in scope. Rollback should remove or ignore the new conformance gating and capability projections while preserving existing Discord channel behavior and any already-recorded redacted conformance evidence as diagnostic history where appropriate.
- **Verification Strategy**: Required validation includes shared fake connector conformance tests, Discord regression coverage as the only required real connector baseline, tenant isolation and permission-denial cases, durable dedupe and restart-replay cases, reply progression degradation cases, foreground/background delivery separation cases, diagnostic classification cases, redaction tests, and contract/schema verification for capability and diagnostic projections.
- **Observability Impact**: Operators must gain consistent connector health, diagnostic reason, remediation owner, unsupported capability, redaction status, dedupe, blocked-route, reply-failure, retention-expiry, and delivery-boundary evidence across hosted connectors without reading raw provider logs.
- **Environment & Secrets**: Automated verification must default to the test environment with fake connectors and fake credentials. Live connectors, production tenants, real channel credentials, rich media, voice, and mobile push are not required for acceptance and must not be touched unless an operator explicitly chooses a separate live validation path.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of core invariants pass for each hosted-ready connector under test, and 100% of provider-specific surfaces produce a supported, unsupported, or limited result with zero silent skips.
- **SC-002**: 100% of representative duplicate and retry fixture cases using the standard message identity, or an explicit equivalent durable identity rule, result in at most one user-visible assistant reply per original inbound channel message.
- **SC-003**: 100% of tenant isolation and permission-denial fixture cases fail closed without revealing inaccessible tenant, connector, account binding, secret, or message details.
- **SC-004**: Existing Discord connector regression coverage either passes every required contract area or records explicit unsupported or limited capabilities for 100% of unmet optional behaviors, with no Telegram, Slack, or other non-Discord connector regression required for Phase 48.
- **SC-005**: 100% of required diagnostic fixture cases classify auth missing, permission missing, rate limited, provider unavailable, network failed, blocked route, duplicate inbound, unsupported capability, reply failed, and unknown connector failure with stable reason meanings.
- **SC-005a**: 100% of connector diagnostic inspection views mark cached diagnostic state older than 15 minutes as stale, and 100% of connector action failures produce current diagnostic truth before remediation is shown.
- **SC-005b**: 100% of connector conformance results, diagnostic evidence, and redaction-failure outcomes in retention tests expire from normal inspection after 90 days unless covered by an authorized longer retention policy.
- **SC-006**: 100% of redaction tests confirm connector diagnostics, logs, events, replay fixtures, evaluation artifacts, support output, and conformance results exclude raw tokens, secret values, authorization headers, credential-bearing payloads, and cross-tenant data.
- **SC-007**: 100% of foreground reply and background delivery separation tests preserve separate outcomes when both paths use the same connector transport mechanics.
- **SC-008**: A future connector specification can reference the Phase 48 contract and omit shared routing, dedupe, reply progression, diagnostic, tenant-scope, and delivery-boundary requirements without losing acceptance-test coverage for those behaviors.

## Assumptions

- Roadmap 28 delivery and notifications, Roadmap 37 hosted secrets and connector isolation, and Roadmap 42 integration health and permission diagnostics are available as prerequisite product truths.
- Discord is the existing reference connector and must be adapted to the shared contract rather than exempted from it.
- Telegram, Slack, and future connectors are target consumers of this contract, but implementing those connectors is out of scope for Phase 48.
- Foreground connector conversations and background delivery targets may share transport mechanics, but their user-visible outcomes and operator-visible histories remain separate.
- Rich media, voice, channel-specific advanced interactions, mobile push, and memory-based conversation continuity are optional future enhancements and cannot be required to pass core hosted connector conformance.
- Conformance is production gating evidence, not only local developer documentation; unsupported behavior must be explicit and reviewable.
