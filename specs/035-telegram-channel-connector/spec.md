# Feature Specification: Telegram Channel Connector

**Feature Branch**: `035-telegram-channel-connector`
**Created**: 2026-05-07
**Status**: Draft
**Input**: User description: "$speckit-specify结合 docs/specs/035-telegram-channel-connector.md 完成 phase 50 的工作"

## Clarifications

### Session 2026-05-07

- Q: What Telegram attachment scope is included in phase 50? → A: Text-only in phase 50; all attachments are explicit unsupported outcomes.
- Q: Who is allowed to create agent runs through Telegram? → A: Only explicitly allowed Telegram users/chats may create runs.
- Q: What gate is required for Telegram group messages? → A: Allowed group plus bot mention or command.
- Q: What durable identity should suppress duplicate Telegram inbound messages? → A: Dedupe by chat/message identity, retain update identity as delivery evidence.
- Q: What terminal states should Telegram setup use? → A: Use ready, degraded, unavailable, cancelled, and action-required.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Connect Telegram For Hosted Messaging (Priority: P1)

A hosted tenant user can add a Telegram bot as a channel for their personal agent, validate that the bot is usable, and see a readiness state with redacted setup evidence.

**Why this priority**: Telegram cannot safely receive or send agent messages until setup proves tenant ownership, credential validity, redaction, and readiness.

**Independent Test**: Can be tested by starting a Telegram setup attempt with a safe test credential plus explicit tenant allowment, completing validation, and confirming the connector reaches `ready`; the same valid credential without explicit allowment must remain `action-required`, and all terminal outcomes must avoid exposing raw secrets.

**Acceptance Scenarios**:

1. **Given** a hosted tenant user with permission to manage connectors, **When** they submit a Telegram bot credential, configure explicit Telegram allowment, and complete setup, **Then** the connector is bound to that tenant, stores only redacted evidence for display, and reports `ready` when credential, binding, conformance, and allowment validation succeed.
2. **Given** a submitted Telegram credential is invalid or missing required access, **When** setup validation runs, **Then** the connector reports `degraded`, `unavailable`, or `action-required` as appropriate, the user sees a clear remediation next step, and raw credential material is not shown in product output, logs, diagnostics, or support evidence.
3. **Given** a setup attempt is interrupted or fails temporarily, **When** the user retries, replaces, cancels, or disables setup, **Then** the setup state remains auditable and unrelated integration state is preserved.

---

### User Story 2 - Route Telegram Messages To The Agent (Priority: P2)

A user can message the agent through Telegram direct messages, and can enable controlled group behavior so group messages are accepted only when the group is explicitly allowed and the message includes a bot mention or command.

**Why this priority**: Telegram provides value only if inbound messages are routed predictably and duplicates, blocked groups, and unsupported message types cannot create confusing or repeated agent runs.

**Independent Test**: Can be tested with fake Telegram updates covering direct messages, allowed groups, blocked groups, mention-gated groups, duplicate updates, and unsupported message forms.

**Acceptance Scenarios**:

1. **Given** a ready Telegram connector and an inbound direct message from an explicitly allowed user or chat, **When** the message is received, **Then** one agent run is created for the tenant and the routing decision records the Telegram chat and message identity.
2. **Given** group behavior is disabled or the group is not allowed, **When** a group message arrives, **Then** the message is ignored or blocked with a recorded routing decision and no agent run is created.
3. **Given** group behavior is enabled for an explicitly allowed group, **When** a group message does not include a bot mention or command, **Then** it is ignored without replying publicly and the decision remains inspectable.
4. **Given** the same Telegram update or message is delivered more than once, **When** the duplicates are processed, **Then** only the first accepted input can create a run or foreground reply.

---

### User Story 3 - Reply And Deliver Through Telegram With Diagnostics (Priority: P3)

A user receives foreground replies for accepted Telegram messages, and operators can reuse Telegram as a background delivery target while inspecting separate delivery and connector health evidence.

**Why this priority**: Reply and delivery failures must be visible without conflating agent execution success, foreground reply success, and background delivery success.

**Independent Test**: Can be tested by simulating accepted messages, reply success and failure, background delivery success and failure, rate limits, provider outages, and network failures.

**Acceptance Scenarios**:

1. **Given** an accepted Telegram message produces an agent response, **When** the response is ready, **Then** the user receives at least a final foreground reply and the reply outcome is recorded separately from the agent execution outcome.
2. **Given** Telegram is selected as a background delivery target, **When** scheduled or workflow-originated work emits a result, **Then** Telegram delivery success, retry, suppression, or failure is recorded separately from foreground replies.
3. **Given** Telegram returns an authentication, permission, rate-limit, provider, unsupported, or network failure, **When** support inspects the connector, **Then** the diagnostic reason, freshness, remediation guidance, and redacted evidence are visible without raw provider payloads.

### Edge Cases

- A Telegram bot credential is malformed, revoked, expired, or belongs to the wrong tenant.
- A connector is disabled while Telegram updates are still arriving.
- Telegram redelivers old updates after restart, reconnect, retry, or webhook recovery.
- Telegram sends unsupported inputs such as attachments, voice, payments, mini apps, media transfer, or provider-specific controls outside the approved phase scope.
- A group message mentions the bot or uses a command but the group is not explicitly allowed for the tenant.
- A direct-message sender contacts the bot but the Telegram user or chat is not explicitly allowed for the tenant.
- Telegram rate limits replies or delivery notifications after the agent run succeeds.
- The provider or network is unavailable during setup, inbound routing, foreground reply, or background delivery.
- A support user inspects setup and diagnostics for a tenant they are not authorized to view.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST offer a tenant-owned Telegram connector setup path that supports submitted bot credentials, explicit allowment validation, retry, replacement, cancellation, disablement, readiness status, and redacted setup evidence, using `ready`, `degraded`, `unavailable`, `cancelled`, and `action-required` as setup terminal states.
- **FR-002**: The system MUST reject or degrade Telegram setup when the credential cannot be validated, required access is missing, the provider is unavailable, or the network fails, and MUST expose an actionable diagnostic state using the standard setup terminal-state vocabulary for each case.
- **FR-003**: The system MUST preserve tenant, connector account, Telegram chat, Telegram user or group, and Telegram message identity for every accepted inbound message, foreground reply, diagnostic, and connector-backed delivery outcome.
- **FR-004**: The system MUST apply durable inbound deduplication using Telegram chat and message identity so duplicate provider deliveries cannot create duplicate agent runs or duplicate foreground replies, and MUST retain Telegram update identity as redacted delivery evidence.
- **FR-005**: The system MUST support direct-message routing for a ready Telegram connector only when the Telegram sender or chat has been explicitly allowed for the tenant.
- **FR-006**: The system MUST support group routing only when group behavior is explicitly enabled for the tenant, the Telegram group is explicitly allowed, and the incoming message includes a bot mention or command.
- **FR-007**: The system MUST record every inbound routing decision as accepted, ignored, blocked, duplicate, unsupported, or failed, using the shared channel connector outcome vocabulary.
- **FR-008**: The system MUST provide at least final-only foreground replies for accepted Telegram messages, and MUST explicitly record any unsupported reply progression capabilities.
- **FR-009**: The system MUST allow Telegram to be reused as a background delivery target for scheduled or workflow-originated results while keeping background delivery truth separate from foreground reply truth.
- **FR-010**: The system MUST classify Telegram setup, inbound, reply, and delivery failures into shared connector diagnostic reasons covering authentication, permission, rate limit, provider availability, network failure, unsupported behavior, blocked route, duplicate input, and unknown failure.
- **FR-011**: The system MUST redact Telegram credentials, provider payloads, account identifiers where required, setup evidence, logs, events, fixtures, support output, and diagnostic records before they are exposed outside the trusted connector boundary.
- **FR-012**: The system MUST expose connector health, readiness, diagnostic freshness, remediation guidance, and redacted evidence to authorized users and support operators.
- **FR-013**: The system MUST fail explicitly for Telegram attachments, voice, payments, mini apps, media transfer, memory behavior, and any other Telegram surface not included in this phase.
- **FR-014**: The system MUST retain enough setup, routing, dedupe, reply, delivery, and diagnostic evidence to prove connector conformance and support rollback without exposing raw secrets.
- **FR-015**: The system MUST satisfy the shared channel connector conformance contract for tenant ownership, permission-gated inspection, account binding, routing decisions, dedupe, redaction, diagnostics, and foreground/background delivery separation.

### Key Entities

- **Telegram Connector**: A tenant-owned channel configuration that represents Telegram readiness, enablement, allowed routing behavior, diagnostic state, and delivery-target eligibility.
- **Telegram Setup Attempt**: A recoverable setup lifecycle record with tenant, actor, target channel, current state, terminal state, redacted evidence, diagnostic linkage, and retry or replacement history.
- **Telegram Account Binding**: The validated association between a tenant and the Telegram bot identity that may receive inbound messages and send replies or deliveries.
- **Telegram Conversation**: A direct message or group context with routing policy, chat identity, explicit group allowment when the context is a group, and group bot mention or command-gating behavior.
- **Telegram Message Event**: An inbound provider event with durable Telegram chat and message identity, tenant binding, sender context, message content classification, routing decision, dedupe evidence, and retained update identity as delivery evidence.
- **Telegram Reply Outcome**: The foreground response result for an accepted inbound message, including success, retry, failure, unsupported progression, and redacted provider evidence.
- **Telegram Delivery Outcome**: The background notification result for scheduled or workflow-originated work delivered through Telegram, tracked independently from foreground replies.
- **Telegram Diagnostic Record**: Redacted, tenant-scoped evidence of setup, permission, provider, rate-limit, network, routing, reply, delivery, or unsupported-surface failures with freshness and remediation guidance.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Adds a new Telegram connector surface, setup state, routing and diagnostic evidence, capability declarations, and delivery-target eligibility. Existing channel behavior remains valid, and Telegram must consume the shared channel connector contract rather than redefining routing, dedupe, diagnostics, redaction, or delivery-boundary meanings.
- **Migration / Rollback**: No existing tenants are automatically enrolled. Rollout can be gated by connector availability and tenant setup. Rollback disables new Telegram setup and delivery eligibility while retaining redacted setup, diagnostic, routing, reply, delivery, and conformance evidence for support and audit.
- **Verification Strategy**: Required validation includes fake Telegram transport coverage for setup, routing, dedupe, replies, delivery, restart or reconnect recovery, failure classification, and unsupported surfaces; shared connector conformance coverage; contract coverage for connector resources and events; and one test-environment smoke path with safe Telegram credentials or a structured live-skip record.
- **Observability Impact**: Connector health, readiness, setup attempts, routing decisions, duplicate suppression, foreground reply outcomes, background delivery outcomes, diagnostic freshness, remediation codes, and redaction evidence must be observable to authorized operators without raw Telegram secrets or raw provider payloads.
- **Environment & Secrets**: Development and verification default to the test environment with fake transport and fake credentials. Safe live Telegram credentials are optional for smoke validation and must be tenant-owned, redacted, scoped to the test environment, and never assumed safe for production use.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A hosted user can complete Telegram connector setup or receive an actionable terminal-state diagnostic in under 5 minutes without using raw operator-only resource calls.
- **SC-002**: In conformance tests, 100% of duplicate Telegram update or message deliveries are suppressed so no duplicate agent run or duplicate foreground reply is created.
- **SC-003**: 100% of supported direct-message and explicitly allowed group-message routing scenarios record a tenant-scoped routing decision with preserved Telegram conversation and message identity.
- **SC-004**: 100% of unsupported Telegram surfaces in this phase produce an explicit unsupported outcome instead of silent acceptance or ambiguous failure.
- **SC-005**: 100% of setup, diagnostic, event, support, fixture, and log evidence exposed outside the trusted connector boundary is redacted of raw Telegram bot credentials and raw provider payloads.
- **SC-006**: Authorized support inspection can identify the latest Telegram connector health state, diagnostic reason, remediation guidance, and freshness for auth, permission, rate-limit, provider, and network failures within 2 minutes.
- **SC-007**: Foreground reply outcomes and background delivery outcomes remain separately inspectable for 100% of Telegram messages and Telegram-backed deliveries exercised by the verification suite.

## Assumptions

- Roadmap 48 channel connector conformance and Roadmap 46 hosted credential setup behavior are available as upstream contracts for this phase.
- The first Telegram slice is limited to bot-based text messaging, direct messages, allowed group behavior, foreground replies, diagnostics, and background delivery target reuse.
- Telegram group behavior defaults to disabled until a tenant explicitly enables the group, allows the route, and uses bot mention or command gating.
- Final-only foreground replies are sufficient for the first phase; richer reply progression is allowed only if it still reports capability support or unsupported status explicitly.
- Telegram attachments are out of scope for phase 50 and must produce explicit unsupported outcomes.
- Production tenants and live connector credentials are not touched during default verification.
