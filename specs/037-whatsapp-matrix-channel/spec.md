# Feature Specification: Matrix Channel Connector

**Feature Branch**: `037-whatsapp-matrix-channel`  
**Created**: 2026-05-09  
**Status**: Draft  
**Input**: User description: "$speckit-specify combine docs/specs/037-whatsapp-or-matrix-channel-connector.md and complete phase 52"

## Clarifications

### Session 2026-05-09

- Q: Which provider should phase 52 implement? → A: Matrix.
- Q: What Matrix setup ownership model should phase 52 use? → A: Tenant-provided Matrix bot account on a tenant-selected homeserver.
- Q: What invocation gate should Matrix room routing use? → A: Allowed rooms require bot mention or configured command.
- Q: What encrypted Matrix room scope is included in phase 52? → A: Unencrypted text only; encrypted rooms and undecryptable events are unsupported.
- Q: What durable identity should suppress duplicate Matrix inbound events? → A: Dedupe by homeserver, room or direct conversation, and Matrix event ID; retain sync or transaction identity as delivery evidence.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Record Matrix As The Fourth Channel (Priority: P1)

A product owner can see Matrix selected as the fourth real channel connector, with WhatsApp recorded as the rejected alternative based on hosted viability, provider policy risk, setup safety, diagnostics coverage, and shared connector conformance.

**Why this priority**: Phase 52 explicitly depends on a provider-risk decision before implementation. Building both channels or building against unsupported behavior would create scope drift, hidden operational risk, and unreliable hosted behavior.

**Independent Test**: Can be tested by confirming that the provider selection record before implementation planning names Matrix as chosen, WhatsApp as rejected, decision owner, provider-risk evidence, hosted viability evidence, unsupported behavior boundaries, and conformance implications.

**Acceptance Scenarios**:

1. **Given** phase 52 is ready for planning, **When** the provider decision is reviewed, **Then** Matrix is recorded as the only provider selected for implementation and WhatsApp is recorded as rejected for this phase.
2. **Given** Matrix support would require unsupported, brittle, or policy-violating behavior for hosted operation, **When** the provider-risk decision is reviewed, **Then** the phase remains blocked until a hosted-safe Matrix path is defined.
3. **Given** Matrix has room identity, membership, federation, encryption, and homeserver constraints, **When** the decision is accepted, **Then** unencrypted text is the only supported message scope, encrypted rooms and undecryptable events are recorded as unsupported, and the remaining constraints are recorded as implementation requirements and verification risks before planning begins.

---

### User Story 2 - Connect Matrix For Hosted Messaging (Priority: P2)

A hosted tenant user can connect Matrix by providing a tenant-owned Matrix bot account on a tenant-selected homeserver, validate that the connector is ready for safe inbound and outbound messaging, and inspect redacted setup status and remediation guidance.

**Why this priority**: The connector cannot safely route messages or send replies until setup proves tenant ownership, provider identity, route eligibility, credential safety, diagnostics readiness, and redaction.

**Independent Test**: Can be tested by completing Matrix setup in the test environment using fake tenant-provided bot account evidence and, when safe credentials are available, a structured live smoke; invalid bot credentials, missing permissions, unsafe setup modes, unsupported homeserver behavior, or unavailable Matrix behavior must produce an actionable terminal state without exposing raw secrets or raw provider payloads.

**Acceptance Scenarios**:

1. **Given** a hosted tenant user with connector-management permission, **When** they submit a tenant-provided Matrix bot account for a tenant-selected homeserver and complete the supported Matrix setup path, **Then** one connector is bound to that tenant, stores only redacted setup evidence, and reports `ready` only when Matrix account identity, homeserver reachability, room identity, tenant ownership, required permissions, route policy, conformance checks, and diagnostic probes pass.
2. **Given** setup is incomplete, unsafe, unauthorized, missing required permissions, provider-blocked, unsupported by the selected homeserver, or linked to an account or room identity that cannot be proven tenant-owned, **When** setup validation runs, **Then** the connector reports `action-required`, `degraded`, or `unavailable` as appropriate and provides a remediation owner and next step.
3. **Given** setup is interrupted, temporarily unavailable, later revoked, or needs repair, **When** the user retries, replaces, cancels, disables, or re-runs validation, **Then** the lifecycle remains tenant-scoped, auditable, recoverable, and isolated from unrelated integration state.

---

### User Story 3 - Route Messages And Send Replies Through Matrix (Priority: P3)

An authorized user can reach the agent through Matrix using supported unencrypted direct conversation or unencrypted room behavior, receive foreground replies for accepted text messages, and avoid duplicate agent runs when Matrix retries or redelivers messages.

**Why this priority**: The fourth channel only adds production value if inbound messages route to the correct tenant once, unsupported contexts are blocked or ignored predictably, and replies are tracked separately from agent execution.

**Independent Test**: Can be tested with fake Matrix events covering accepted unencrypted direct conversations, accepted unencrypted room messages, encrypted rooms, undecryptable events, disallowed senders or rooms, wrong tenant identity, disabled connector state, unsupported message forms, duplicate deliveries, outbound reply success, outbound reply failure, provider retry, and restart recovery.

**Acceptance Scenarios**:

1. **Given** a ready Matrix connector, **When** an allowed unencrypted direct conversation text message is received from an eligible sender, **Then** one agent run is created for the tenant and the routing decision records Matrix homeserver, account or room, conversation, sender, and event identity.
2. **Given** Matrix room behavior is supported, **When** an unencrypted text message arrives in an allowed room and includes a bot mention or configured command, **Then** one agent run is created for the tenant and the response target is tied to the originating Matrix room and event.
3. **Given** a message arrives from a wrong account, unallowed room, ineligible sender, disabled connector, unsupported provider context, encrypted room, undecryptable event, or allowed room message without a bot mention or configured command, **When** routing evaluates the message, **Then** no agent run is created and the decision is recorded as ignored, blocked, unsupported, or failed with redacted evidence.
4. **Given** Matrix redelivers the same event after retry, restart, reconnect, network recovery, sync replay, transaction retry, or delayed delivery, **When** duplicate deliveries are processed, **Then** only the first accepted homeserver, room or direct conversation, and Matrix event ID can create an agent run or foreground reply while sync or transaction identity remains available as redacted delivery evidence.
5. **Given** an accepted Matrix event produces an agent response, **When** the response is ready, **Then** the user receives at least a final foreground reply when Matrix delivery permits it and the reply outcome is recorded separately from the agent execution outcome.

---

### User Story 4 - Diagnose Provider-Specific Failures (Priority: P4)

Authorized operators can inspect Matrix connector health, setup status, routing decisions, duplicate suppression, outbound reply results, background delivery results, and Matrix-specific failure diagnostics without seeing raw secrets or cross-tenant provider data.

**Why this priority**: A materially different provider introduces new operational failure modes. Operators need clear provider-specific truth for authentication, permission, rate-limit, provider, network, blocked-route, duplicate, and unsupported-behavior cases.

**Independent Test**: Can be tested by simulating setup failures, revoked access, permission denial, rate limits, provider outages, network failures, unsupported message types, duplicate inputs, blocked routes, foreground reply failures, background delivery failures, and unauthorized support inspection.

**Acceptance Scenarios**:

1. **Given** Matrix reports authentication, permission, rate-limit, provider, network, duplicate, blocked-route, unsupported-behavior, setup, inbound, reply, or delivery failure evidence, **When** support inspects the connector, **Then** the diagnostic reason, freshness, remediation guidance, and redacted evidence are visible to authorized users.
2. **Given** the Matrix connector is selected as a background delivery target, **When** scheduled or workflow-originated work emits a result, **Then** delivery success, retry, suppression, or failure is recorded separately from foreground replies and agent execution truth.
3. **Given** a support user is not authorized for the tenant, **When** they attempt to inspect setup, route, message, diagnostic, or delivery evidence, **Then** access is denied and no provider identity, provider payload, or tenant-owned route evidence is exposed.

### Edge Cases

- Matrix cannot provide a hosted-safe setup or operation path.
- Matrix supported behavior changes before implementation planning or before release.
- A tenant attempts to configure both WhatsApp and Matrix in phase 52.
- A tenant attempts to configure Matrix through unsupported unofficial automation, local-only sessions, a DopeAgent-operated shared homeserver, raw secrets outside the supported setup path, or behavior that violates provider policy.
- A tenant-selected homeserver is unreachable, unsupported, rate-limited, misconfigured, or rejects required bot account and room operations.
- Matrix account, room, sender, user, device, or federation identity cannot be proven tenant-owned or cannot be mapped to exactly one tenant route.
- Provider authorization is revoked, expires, loses required permissions, or is blocked by provider approval after setup was previously ready.
- Matrix redelivers old events through the same or different sync batches or transaction identities after restart, retry, network recovery, or delivery backlog processing.
- A message is received in a Matrix room, but the sender, room, bot mention, configured command, membership, or invocation gate is not eligible for routing.
- Provider replies or background deliveries are rate-limited, delayed, blocked, or unsupported after the agent run succeeds.
- Unsupported inputs such as calls, voice, rich media, reactions, encrypted rooms, undecryptable events, bridge-specific metadata, or provider-specific controls are received before explicit support exists.
- Matrix-specific room identity, membership, federation, encrypted-room semantics, or end-to-end encryption key/session management conflicts with tenant isolation or diagnostics expectations.
- WhatsApp remains out of scope for phase 52 and must not be partially implemented as a fallback.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST implement Matrix as the fourth real channel connector in phase 52, and MUST NOT implement WhatsApp in this phase.
- **FR-002**: The system MUST record the provider selection decision before implementation planning, including Matrix as the chosen provider, WhatsApp as the rejected alternative, decision owner, decision date, hosted viability evidence, provider-policy risk, operational risks, unsupported behavior boundaries, and conformance implications.
- **FR-003**: The system MUST block implementation planning when Matrix depends on unsupported unofficial automation, policy-violating behavior, unsafe credential handling, or provider behavior that cannot be operated reliably in hosted environments.
- **FR-004**: The Matrix connector MUST satisfy the shared channel connector conformance contract for tenant ownership, permission-gated inspection, account binding, routing decisions, durable deduplication, redaction, diagnostics, and foreground versus background delivery separation.
- **FR-005**: The Matrix connector MUST reference the shared channel connector conformance behavior for routing, dedupe, diagnostics, redaction, delivery boundaries, and outcome vocabulary instead of redefining those shared meanings.
- **FR-006**: The system MUST offer a tenant-scoped hosted Matrix setup path for a tenant-provided Matrix bot account on a tenant-selected homeserver, supporting retry, replacement, cancellation, disablement, readiness status, route policy validation, diagnostic probes, and redacted setup evidence.
- **FR-006a**: The system MUST NOT operate a shared hosted Matrix homeserver, provision Matrix accounts for tenants, or require tenants to move rooms to DopeAgent-controlled Matrix infrastructure in phase 52.
- **FR-007**: Setup MUST use the standard terminal states `ready`, `degraded`, `unavailable`, `cancelled`, and `action-required`, with actionable remediation for invalid bot authorization, missing permissions, ownership mismatch, unsupported homeserver behavior, unsafe setup mode, provider unavailability, and network failure.
- **FR-008**: The system MUST preserve tenant, connector, tenant-selected Matrix homeserver, Matrix bot account identity, Matrix room or direct conversation identity, sender identity, and Matrix event identity for every accepted inbound message, foreground reply, diagnostic, setup attempt, and connector-backed delivery outcome.
- **FR-009**: The system MUST prevent cross-tenant routing by rejecting or blocking messages when provider account, room, sender, conversation, connector state, or route policy cannot be associated with exactly one active tenant route.
- **FR-010**: The system MUST apply durable inbound deduplication using tenant-selected homeserver, Matrix room or direct conversation identity, and Matrix event ID so duplicate Matrix deliveries cannot create duplicate agent runs, duplicate foreground replies, or duplicate background delivery truth.
- **FR-011**: The system MUST retain Matrix sync batch or transaction delivery identity as redacted evidence when it differs from the durable event identity used for deduplication.
- **FR-012**: The Matrix connector MUST support direct conversation routing only for unencrypted text messages from eligible senders and tenant-allowed routes.
- **FR-013**: The Matrix connector MUST support room routing only for unencrypted text messages when the room is tenant-allowed and the message includes a bot mention or configured command.
- **FR-014**: The system MUST record every inbound routing decision as accepted, ignored, blocked, duplicate, unsupported, or failed using the shared channel connector outcome vocabulary.
- **FR-015**: The system MUST provide at least final-only foreground replies for accepted Matrix messages when Matrix delivery permits outbound replies, and MUST record explicit unsupported or failed outcomes when Matrix delivery cannot safely send a reply.
- **FR-016**: The system MUST allow the Matrix connector to be reused as a background delivery target for scheduled or workflow-originated results while keeping background delivery truth separate from foreground replies and agent execution truth.
- **FR-017**: The system MUST classify setup, inbound, reply, delivery, rate-limit, provider, permission, authentication, blocked-route, duplicate, unsupported-behavior, and network failures into the shared connector diagnostic reason vocabulary.
- **FR-018**: Matrix-specific diagnostics MUST distinguish bot authorization or authentication failure, missing permission, ownership mismatch, route or room access failure, unsupported tenant-selected homeserver behavior, federation or homeserver failure, provider rate limit, provider unavailability, local network failure, duplicate input, blocked route, unsupported behavior, and unknown failure when evidence permits.
- **FR-019**: The system MUST redact provider credentials, authorization grants, account identifiers where required, room or conversation identifiers where required, user or phone identifiers where required, raw provider payloads, setup evidence, logs, events, fixtures, support output, and diagnostic records before they are exposed outside the trusted connector boundary.
- **FR-020**: The system MUST expose Matrix connector health, readiness, diagnostic freshness, remediation guidance, selected routes, blocked-route evidence, duplicate suppression evidence, redacted setup evidence, and delivery-target eligibility to authorized users and support operators.
- **FR-021**: The system MUST fail explicitly for the rejected provider, unsupported setup modes, encrypted rooms, undecryptable events, end-to-end encryption key/session management, voice, calls, media-rich workflows, memory-based channel personalization, and provider-specific surfaces not included in phase 52.
- **FR-022**: The system MUST retain enough provider decision, setup, routing, dedupe, reply, delivery, diagnostics, and conformance evidence to prove behavior and support rollback without exposing raw secrets or cross-tenant provider data.

### Key Entities

- **Provider Selection Decision**: The phase 52 record naming Matrix as the chosen provider, WhatsApp as the rejected alternative, decision owner, risk evidence, hosted viability assessment, unsupported behavior boundaries, and planning readiness.
- **Matrix Channel Connector**: A tenant-owned channel configuration for Matrix that represents readiness, enablement, selected routes, diagnostic state, and delivery-target eligibility.
- **Setup Attempt**: A recoverable hosted Matrix setup lifecycle record with tenant, actor, tenant-selected homeserver, tenant-provided Matrix bot account identity, current state, terminal state, redacted evidence, diagnostic linkage, and retry or replacement history.
- **Matrix Account Or Room Binding**: The validated association between one tenant connector, one tenant-selected homeserver, the tenant-provided Matrix bot account, and the Matrix room or direct conversation identities that may receive inbound messages and send replies or deliveries.
- **Route Policy**: The tenant-scoped, connector-specific policy defining eligible senders, selected rooms or conversations, bot mention and configured command invocation gates, disabled or blocked routes, and delivery-target eligibility.
- **Matrix Conversation**: An unencrypted direct or unencrypted room context with conversation identity, route policy, supported participant semantics, and outbound reply eligibility.
- **Matrix Message Event**: An inbound Matrix event with durable homeserver, room or direct conversation identity, Matrix event ID, encryption support classification, retained sync or transaction delivery identity, tenant binding, sender context, content classification, routing decision, dedupe evidence, and redacted provider evidence.
- **Reply Outcome**: The foreground response result for an accepted inbound message, including success, retry, failure, unsupported behavior, provider conversation association, and redacted provider evidence.
- **Delivery Outcome**: The background notification result for scheduled or workflow-originated work delivered through Matrix, tracked independently from foreground replies.
- **Diagnostic Record**: Redacted, tenant-scoped evidence of setup, permission, provider, rate-limit, network, routing, reply, delivery, duplicate, blocked-route, or unsupported-surface failures with freshness and remediation guidance.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Adds one new Matrix channel connector surface, plus provider decision evidence, hosted setup lifecycle for tenant-provided bot accounts, Matrix account or room binding, route policy, routing and diagnostic evidence, capability declarations, and delivery-target eligibility. Existing channel behavior remains valid, and Matrix must consume the shared channel connector contract rather than redefining routing, dedupe, diagnostics, redaction, or delivery-boundary meanings.
- **Migration / Rollback**: No existing tenants are automatically enrolled. Rollout can be gated by connector availability, tenant-provided bot setup, tenant-selected homeserver and room validation, and route validation. Rollback disables new Matrix setup and delivery eligibility while retaining redacted decision, setup, diagnostic, routing, reply, delivery, and conformance evidence for support and audit.
- **Verification Strategy**: Required validation includes provider-risk research, fake Matrix transport coverage for setup, account or room binding, unencrypted direct routing, unencrypted allowed room routing with bot mention or configured command, room messages without invocation gates, encrypted rooms, undecryptable events, duplicate suppression by homeserver, room or direct conversation, and Matrix event ID, retained sync or transaction delivery evidence, foreground replies, background delivery, restart or reconnect recovery, failure classification, unsupported setup modes, unsupported surfaces, shared connector conformance coverage, contract coverage for connector resources and events, and one safe real-account smoke path where credentials are available or a structured live-skip record.
- **Observability Impact**: Connector health, readiness, setup attempts, provider decision evidence, Matrix binding, selected routes, routing decisions, duplicate suppression, foreground reply outcomes, background delivery outcomes, diagnostic freshness, remediation codes, provider-risk notes, and redaction evidence must be observable to authorized operators without raw Matrix secrets, raw provider payloads, or cross-tenant provider data.
- **Environment & Secrets**: Development and verification default to the test environment with fake provider transport and fake tenant-provided bot credential evidence. Safe live Matrix bot credentials are optional for smoke validation and must be tenant-owned, provider-compliant, redacted, scoped to the test environment, and never assumed safe for production use.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Before implementation planning, 100% of phase 52 planning artifacts identify Matrix as the chosen provider, WhatsApp as the rejected alternative, provider-risk evidence, hosted viability evidence, and unsupported behavior boundaries.
- **SC-002**: A hosted user can complete setup for the Matrix connector with a tenant-provided Matrix bot account on a tenant-selected homeserver, or receive an actionable terminal-state diagnostic, in under 5 minutes without using unsupported unofficial automation or exposing raw secret material.
- **SC-003**: In conformance tests, 100% of duplicate deliveries for the same tenant-selected homeserver, Matrix room or direct conversation, and Matrix event ID are suppressed so no duplicate agent run, foreground reply, or delivery outcome is created, while sync or transaction identity remains available as redacted delivery evidence.
- **SC-004**: 100% of supported unencrypted direct conversation and unencrypted allowed-room bot mention or configured command routing scenarios record a tenant-scoped routing decision with preserved Matrix homeserver, account or room, conversation, sender, and event identity.
- **SC-005**: 100% of wrong-account, wrong-room, ineligible-sender, disabled-connector, encrypted-room, undecryptable-event, unsupported-context, unsafe-setup, and room-message-without-bot-mention-or-command scenarios produce explicit ignored, blocked, failed, or unsupported outcomes instead of silent acceptance.
- **SC-006**: 100% of setup, diagnostic, event, support, fixture, and log evidence exposed outside the trusted connector boundary is redacted of raw provider credentials, authorization grants, and raw provider payloads.
- **SC-007**: Authorized support inspection can identify the latest Matrix connector health state, diagnostic reason, remediation guidance, freshness, selected routes, duplicate suppression state, and delivery eligibility within 2 minutes.
- **SC-008**: Foreground reply outcomes and background delivery outcomes remain separately inspectable for 100% of Matrix messages and deliveries exercised by the verification suite.

## Assumptions

- Roadmap 48 channel connector conformance and the previous production channel connector slices are available as upstream contracts for phase 52.
- Phase 52 implements one channel only: Matrix; WhatsApp is rejected for this phase and remains out of scope unless a later roadmap phase selects it.
- Matrix must have a provider-compliant hosted setup and operation path before implementation planning proceeds.
- Phase 52 uses tenant-provided Matrix bot accounts on tenant-selected homeservers; DopeAgent-operated shared Matrix homeserver provisioning is out of scope.
- Direct conversation support is required for unencrypted Matrix text messages. Room behavior is included only for unencrypted text messages in tenant-allowed rooms where messages include a bot mention or configured command.
- End-to-end encrypted Matrix rooms, undecryptable events, and Matrix key/session management are out of scope for phase 52.
- Final-only foreground replies are sufficient for the first phase; richer reply progression, interactive controls, media, calls, voice, and provider-specific controls are allowed only when they still report supported, limited, failed, or unsupported status explicitly.
- Production tenants and live connector credentials are not touched during default verification.
