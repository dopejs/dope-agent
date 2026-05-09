# Feature Specification: Slack Channel Connector

**Feature Branch**: `036-slack-channel-connector`  
**Created**: 2026-05-08  
**Status**: Draft  
**Input**: User description: "$speckit-specify 结合 docs/specs/036-slack-channel-connector.md 完成 phase 51 的工作"

## Clarifications

### Session 2026-05-08

- Q: What Slack direct-message senders may create agent runs in phase 51? → A: Only explicitly allowed Slack users or user groups may DM the agent.
- Q: What Slack thread reply behavior is required in phase 51? → A: Channel mentions reply in a thread rooted at the triggering message; DMs reply normally.
- Q: What durable identity should suppress duplicate Slack inbound messages? → A: Dedupe by workspace, conversation, and message identity; retain event identity as delivery evidence.
- Q: How many Slack workspaces may a tenant connect in phase 51? → A: Multiple Slack connectors per tenant, each bound to one workspace.
- Q: What Slack setup mode is required in phase 51? → A: Hosted Slack app installation/OAuth setup only.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Connect A Slack Workspace (Priority: P1)

A hosted tenant user can install and connect one Slack workspace per Slack connector through the hosted Slack app installation/OAuth setup flow, add more Slack connectors for additional workspaces, select the channels where each connector is allowed to participate, and see whether each connector is ready with redacted setup evidence.

**Why this priority**: Slack cannot safely receive or send agent messages until setup proves tenant ownership, workspace identity, app installation status, scope readiness, channel allowment, and redaction.

**Independent Test**: Can be tested by completing the hosted Slack app installation/OAuth setup flow for a safe Slack test workspace, selecting at least one allowed channel, and confirming the connector reaches `ready`; setup attempts with missing installation, missing scope, wrong workspace, or no allowed route must produce an actionable terminal state without exposing raw credential or authorization material.

**Acceptance Scenarios**:

1. **Given** a hosted tenant user with connector-management permission, **When** they complete the hosted Slack app installation/OAuth setup flow for a workspace and select allowed channels, **Then** one Slack connector is bound to that tenant and exactly one workspace, stores redacted setup evidence, and reports `ready` only when installation, workspace identity, required scopes, channel access, conformance checks, and diagnostic probes pass.
2. **Given** Slack authorization is incomplete, missing required scope, blocked by workspace approval, or linked to a workspace that cannot be tenant-owned, **When** setup validation runs, **Then** the connector reports `action-required`, `degraded`, or `unavailable` as appropriate, shows the remediation owner and next step, and does not expose raw tokens, signing material, provider payloads, or inaccessible workspace data.
3. **Given** setup is interrupted, temporarily unavailable, or later needs repair, **When** the user retries, replaces, cancels, disables, or re-runs validation, **Then** the setup lifecycle remains tenant-scoped, auditable, recoverable, and does not delete unrelated integration state.

---

### User Story 2 - Route Slack DMs And Channel Mentions (Priority: P2)

An explicitly allowed Slack user or user group member can send a direct message to the agent, or a Slack user can mention the agent in an allowed channel, and have exactly one accepted message routed to the correct tenant while ignored, blocked, duplicate, and unsupported inputs remain inspectable.

**Why this priority**: Slack is valuable only if users can reach the agent from normal workspace conversations without cross-tenant leakage, repeated runs, or accidental public responses.

**Independent Test**: Can be tested with fake Slack events covering direct messages, selected channels, unselected channels, mention-gated channel messages, duplicate deliveries, workspace mismatch, disabled connector state, unsupported inputs, and provider delivery retries.

**Acceptance Scenarios**:

1. **Given** a ready Slack connector for a tenant-owned workspace, **When** a direct message from an explicitly allowed Slack user or user group member is received, **Then** one agent run is created for the tenant and the routing decision records the Slack workspace, user, conversation, and message identity.
2. **Given** a ready Slack connector with selected allowed channels, **When** a message in an allowed channel mentions the agent, **Then** one agent run is created for the tenant and the response target is tied to the originating Slack conversation and message.
3. **Given** a message arrives from an unselected channel, wrong workspace, disabled connector, ineligible user, or channel message without an agent mention, **When** routing evaluates the message, **Then** no agent run is created and the decision is recorded as ignored or blocked with redacted evidence.
4. **Given** Slack redelivers the same message through repeated or distinct event deliveries, **When** duplicate deliveries are processed after retry, restart, reconnect, or delayed event delivery, **Then** only the first accepted workspace, conversation, and message identity can create an agent run or foreground reply while Slack event identity is retained as delivery evidence.

---

### User Story 3 - Reply, Thread, And Diagnose Slack Outcomes (Priority: P3)

A user receives Slack foreground replies for accepted messages, channel mentions reply in a thread rooted at the triggering message, direct messages reply normally, and operators can inspect connector health, delivery-target reuse, and provider-specific failure diagnostics without confusing agent execution with Slack delivery.

**Why this priority**: Operators need reliable Slack-specific failure truth for scopes, installation, rate limits, and event delivery while users need predictable responses in DMs and channels.

**Independent Test**: Can be tested by simulating accepted direct messages, accepted channel mentions that require thread-rooted replies, thread-unsupported failures, background deliveries, reply failures, missing scopes, installation missing, rate limits, provider outages, network failures, and event-delivery failures.

**Acceptance Scenarios**:

1. **Given** an accepted Slack message produces an agent response, **When** the response is ready, **Then** the user receives at least a final foreground reply and the reply outcome is recorded separately from the agent execution outcome.
2. **Given** an accepted Slack channel mention produces an agent response, **When** the foreground reply is sent, **Then** the reply is associated with a Slack thread rooted at the triggering channel message or records an explicit unsupported or failed outcome.
3. **Given** Slack is selected as a background delivery target, **When** scheduled or workflow-originated work emits a result, **Then** Slack delivery success, retry, suppression, or failure is recorded separately from foreground replies.
4. **Given** Slack reports missing scopes, missing installation, rate limit, event-delivery failure, provider unavailability, network failure, blocked route, duplicate input, or unsupported behavior, **When** support inspects the connector, **Then** the diagnostic reason, freshness, remediation guidance, and redacted evidence are visible to authorized users.

### Edge Cases

- A Slack workspace is already linked to another tenant or cannot be proven tenant-owned for the requested connector.
- A tenant connects more than one Slack workspace, requiring each workspace to have a distinct Slack connector, route policy, diagnostic state, and rollback boundary.
- Slack authorization is revoked, expires, loses scopes, or is blocked by workspace approval after setup was previously ready.
- A user or operator attempts to configure Slack through submitted raw bot tokens, signing secrets, or local-only credentials instead of the hosted Slack app installation/OAuth setup flow.
- A channel was selected during setup but the app is later removed from the channel or loses permission to read or reply there.
- A direct-message sender is not explicitly allowed for the tenant route, cannot be mapped to the tenant-owned workspace, or is blocked by connector policy.
- A channel message mentions the agent but arrives from an unselected channel, archived channel, private channel without app membership, or wrong workspace.
- Slack redelivers old messages through the same or different event identities after restart, retry, network recovery, or event-delivery backlog processing.
- Slack rate limits foreground replies or background deliveries after the agent run succeeds.
- Slack message forms such as files, voice clips, huddles, canvases, workflow buttons, blocks requiring interactive callbacks, or broad rich media are received before they are explicitly supported.
- Event delivery succeeds but the required channel thread reply fails, or agent execution succeeds while Slack delivery remains delayed, retried, or permanently failed.
- A support user attempts to inspect setup, routing, diagnostics, or workspace evidence for a tenant they are not authorized to view.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST offer a tenant-owned Slack connector setup path through hosted Slack app installation/OAuth only, supporting selected channel allowment, explicit direct-message user or user-group allowment, retry, replacement, cancellation, disablement, readiness status, and redacted setup evidence.
- **FR-001a**: The system MUST allow a tenant to own multiple Slack connectors, and each Slack connector MUST be bound to exactly one Slack workspace.
- **FR-002**: Slack setup MUST use the standard hosted setup terminal states `ready`, `degraded`, `unavailable`, `cancelled`, and `action-required`, with actionable remediation for missing installation, missing OAuth grant, missing scope, workspace approval needed, workspace mismatch, provider unavailability, and network failure.
- **FR-003**: Slack workspace identity MUST be explicit, tenant-scoped, and preserved for every accepted inbound message, foreground reply, diagnostic, setup attempt, and connector-backed delivery outcome.
- **FR-004**: The system MUST prevent cross-tenant Slack routing by rejecting or blocking messages when the workspace, account binding, channel, direct-message sender, or connector state cannot be associated with the active tenant route.
- **FR-005**: The system MUST preserve Slack workspace, channel or direct-message conversation, Slack user, and Slack message identity for every accepted inbound message and related reply evidence.
- **FR-006**: The system MUST apply durable inbound deduplication using Slack workspace, conversation, and message identity so duplicate provider deliveries cannot create duplicate agent runs, duplicate foreground replies, or duplicate background delivery truth, and MUST retain Slack event identity as redacted delivery evidence.
- **FR-007**: The system MUST support direct-message routing for a ready Slack connector only when direct messages are enabled and the sender is an explicitly allowed Slack user or member of an explicitly allowed Slack user group under the tenant-owned workspace route.
- **FR-008**: The system MUST support channel routing only for selected allowed channels and only when the incoming channel message mentions the agent or otherwise uses an explicitly supported invocation signal.
- **FR-009**: The system MUST record every inbound Slack routing decision as accepted, ignored, blocked, duplicate, unsupported, or failed using the shared channel connector outcome vocabulary.
- **FR-010**: The system MUST provide at least final-only foreground replies for accepted Slack messages, and MUST explicitly declare whether thinking visibility, incremental edits, interactive controls, and rich message forms are supported, limited, or unsupported.
- **FR-011**: The system MUST send accepted Slack channel mention replies in a thread rooted at the triggering channel message, MUST send accepted direct-message replies normally in the direct-message conversation, and MUST record an explicit unsupported or failed outcome when required thread reply behavior cannot be provided.
- **FR-012**: The system MUST allow Slack to be reused as a background delivery target for scheduled or workflow-originated results while keeping background delivery truth separate from foreground replies and agent execution truth.
- **FR-013**: The system MUST classify Slack setup, inbound, reply, event-delivery, rate-limit, provider, permission, blocked-route, duplicate, unsupported, and network failures into the shared connector diagnostic reason vocabulary.
- **FR-014**: Diagnostics MUST distinguish missing Slack scopes, missing or revoked workspace installation, workspace approval required, channel membership or access missing, rate limits, event-delivery failures, provider unavailable, local network failure, duplicate input, blocked route, unsupported behavior, and unknown failure when evidence permits.
- **FR-015**: The system MUST redact Slack OAuth tokens, installation grants, authorization payloads, provider request and response payloads, workspace identifiers where required, channel identifiers where required, user identifiers where required, setup evidence, logs, events, fixtures, support output, and diagnostic records before they are exposed outside the trusted connector boundary.
- **FR-015a**: The system MUST reject submitted raw Slack bot tokens, signing secrets, and operator-managed local Slack credentials as supported phase 51 setup inputs.
- **FR-016**: The system MUST expose Slack connector health, readiness, diagnostic freshness, remediation guidance, selected routes, blocked-route evidence, duplicate suppression evidence, and redacted setup evidence to authorized users and support operators.
- **FR-017**: The system MUST fail explicitly for Slack marketplace publication, enterprise grid administration, voice huddles, memory-based team context, and unsupported media or interactive Slack surfaces not included in this phase.
- **FR-018**: The system MUST retain enough setup, routing, dedupe, reply, delivery, diagnostics, and conformance evidence to prove Slack connector behavior and support rollback without exposing raw secrets or cross-tenant workspace data.
- **FR-019**: The system MUST satisfy the shared channel connector conformance contract for tenant ownership, permission-gated inspection, account binding, routing decisions, dedupe, redaction, diagnostics, and foreground/background delivery separation.

### Key Entities

- **Slack Connector**: A tenant-owned channel configuration bound to exactly one Slack workspace that represents workspace readiness, enablement, selected routes, direct-message policy, diagnostic state, and delivery-target eligibility.
- **Slack Setup Attempt**: A recoverable hosted Slack app installation/OAuth lifecycle record with tenant, actor, target Slack workspace, current state, terminal state, redacted evidence, diagnostic linkage, and retry or replacement history.
- **Slack Workspace Binding**: The validated association between one Slack connector, one tenant, and one Slack workspace installation that may receive inbound messages and send replies or deliveries.
- **Slack Route Policy**: The tenant-scoped, connector-specific policy defining selected channels, explicitly allowed direct-message users or user groups, mention gating, disabled or blocked routes, and delivery-target eligibility.
- **Slack Conversation**: A direct-message or channel context with workspace identity, conversation identity, route policy, required channel thread-root behavior, and selected-channel status.
- **Slack Message Event**: An inbound provider event with durable Slack workspace, conversation, and message identity, retained Slack event identity as delivery evidence, tenant binding, sender context, content classification, routing decision, dedupe evidence, and redacted provider delivery evidence.
- **Slack Reply Outcome**: The foreground response result for an accepted inbound message, including success, retry, failure, unsupported progression, required channel thread association, direct-message reply association, and redacted provider evidence.
- **Slack Delivery Outcome**: The background notification result for scheduled or workflow-originated work delivered through Slack, tracked independently from foreground replies.
- **Slack Diagnostic Record**: Redacted, tenant-scoped evidence of setup, permission, provider, rate-limit, event-delivery, network, routing, reply, delivery, or unsupported-surface failures with freshness and remediation guidance.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Adds a new Slack connector surface, hosted Slack app installation/OAuth setup lifecycle, one-workspace-per-connector binding, selected-route policy, routing and diagnostic evidence, capability declarations, and delivery-target eligibility. Existing channel behavior remains valid, and Slack must consume the shared channel connector contract rather than redefining routing, dedupe, diagnostics, redaction, or delivery-boundary meanings.
- **Migration / Rollback**: No existing tenants are automatically enrolled. Rollout can be gated by connector availability, tenant setup, and workspace authorization. Rollback disables new Slack setup and Slack delivery eligibility while retaining redacted setup, diagnostic, routing, reply, delivery, and conformance evidence for support and audit.
- **Verification Strategy**: Required validation includes fake Slack transport coverage for hosted app installation/OAuth setup, workspace binding, selected-channel routing, direct messages, mention gating, required channel thread replies, duplicate suppression, replies, background delivery, restart or reconnect recovery, failure classification, unsupported setup modes, unsupported surfaces, shared connector conformance coverage, contract coverage for connector resources and events, and one test-environment smoke path with safe Slack workspace credentials or a structured live-skip record.
- **Observability Impact**: Connector health, readiness, setup attempts, workspace binding, selected routes, routing decisions, duplicate suppression, foreground reply outcomes, background delivery outcomes, event-delivery health, diagnostic freshness, remediation codes, and redaction evidence must be observable to authorized operators without raw Slack secrets or raw provider payloads.
- **Environment & Secrets**: Development and verification default to the test environment with fake Slack transport and fake OAuth installation evidence. Safe live Slack workspace authorization is optional for smoke validation and must be tenant-owned, redacted, scoped to the test environment, and never assumed safe for production use.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A hosted user can complete Slack connector setup for a safe workspace through the hosted Slack app installation/OAuth setup flow, or receive an actionable terminal-state diagnostic, in under 5 minutes without using raw operator-only resource calls.
- **SC-002**: In conformance tests, 100% of duplicate Slack deliveries for the same workspace, conversation, and message identity are suppressed so no duplicate agent run, duplicate foreground reply, or duplicate delivery outcome is created, while event identity remains available as redacted delivery evidence.
- **SC-003**: 100% of supported explicitly allowed direct-message and selected-channel mention routing scenarios record a tenant-scoped routing decision with preserved Slack workspace, conversation, user, and message identity.
- **SC-004**: 100% of wrong-workspace, unselected-channel, ineligible-sender, disabled-connector, unmentioned-channel-message, and unsupported-surface scenarios produce explicit ignored, blocked, or unsupported outcomes instead of silent acceptance.
- **SC-005**: 100% of setup, diagnostic, event, support, fixture, and log evidence exposed outside the trusted connector boundary is redacted of raw Slack OAuth tokens, installation grants, authorization payloads, and raw provider payloads.
- **SC-006**: Authorized support inspection can identify the latest Slack connector health state, diagnostic reason, remediation guidance, freshness, single-workspace binding, selected routes, and event-delivery status within 2 minutes.
- **SC-007**: Foreground reply outcomes, required channel thread outcomes, direct-message reply outcomes, and background delivery outcomes remain separately inspectable for 100% of Slack messages and Slack-backed deliveries exercised by the verification suite.

## Assumptions

- Roadmap 48 channel connector conformance and Roadmap 46 hosted credential setup behavior are available as upstream contracts for this phase.
- Roadmap 42 integration diagnostics provide the stable reason-code model used for Slack authorization, scope, installation, rate-limit, provider, event-delivery, and network failures.
- Slack phase 51 setup uses hosted Slack app installation/OAuth only; submitted raw bot tokens, signing secrets, and local-only credentials are unsupported setup inputs.
- The first Slack slice is limited to workspace-aware setup, selected channels, direct messages, mention-gated channel messages, foreground replies, required channel thread reply behavior, diagnostics, and background delivery target reuse.
- Slack workspace ownership is treated as tenant-scoped connector state; ambiguous or cross-tenant workspace binding blocks routing until repaired.
- A tenant may connect multiple Slack workspaces by creating multiple Slack connectors; each connector has exactly one workspace binding.
- Slack direct messages are blocked unless the sender is an explicitly allowed Slack user or member of an explicitly allowed Slack user group for the tenant route.
- Slack channel messages default to ignored unless the channel is selected and the message mentions the agent or uses another explicitly supported invocation signal.
- Slack channel mention replies are sent in a thread rooted at the triggering channel message; direct-message replies stay in the direct-message conversation.
- Final-only foreground replies are sufficient for the first phase; richer reply progression, interactive controls, and rich message forms are allowed only when they still report supported, limited, or unsupported status explicitly.
- Slack marketplace publication, enterprise grid administration, voice huddles, and memory-based team context are out of scope for phase 51.
- Production tenants and live connector credentials are not touched during default verification.
