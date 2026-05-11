# Feature Specification: Channel Management And Repair UX

**Feature Branch**: `038-channel-management-repair`  
**Created**: 2026-05-10  
**Status**: Draft  
**Input**: User description: "$speckit-specify 结合 docs/specs/038-channel-management-and-repair-ux.md 完成 phase 53 的工作"

**Upstream authority**: `docs/specs/038-channel-management-and-repair-ux.md` is the authoritative upstream document for this work (Roadmap 53). This specification translates that document into testable scenarios, requirements, and success criteria. Where the upstream document and this spec disagree, the upstream document wins and this spec must be updated.

## Clarifications

### Session 2026-05-10

- Q: Which tenant permissions gate channel inspection, diagnostics, mutation, repair, reconnect, credential rotation, and support evidence? → A: Redacted inspection requires `credentials.inspect`; diagnostics require `integrations.diagnostics.read`; disable, re-enable, route edits, and repair starts require `connectors.manage`; reconnect or credential rotation also requires `secrets.manage`.
- Q: When should connector diagnostic state be considered stale for repair and re-enable decisions? → A: Diagnostic state becomes stale after 15 minutes; user-initiated repair, reconnect, rotate, disable, or re-enable must produce current diagnostic truth.
- Q: How should concurrent connector mutations be handled? → A: Connector mutations serialize per connector; disable takes precedence for inbound and delivery eligibility until a later validated re-enable succeeds.
- Q: How long should connector diagnostic, repair, routing, reply, delivery, and support evidence be retained by default? → A: Retain connector diagnostic, repair, routing, reply, delivery, and support evidence for 90 days by default unless an authorized tenant policy requires longer retention.
- Q: May support evidence expose channel message bodies or raw provider payloads? → A: Support evidence is metadata-only by default; message bodies and raw provider payloads are never shown in this phase.
- Q: How should channel connector lists handle tenants with many connectors? → A: Channel connector lists must support pagination with a default page size of 20 and allow users to reach all tenant connectors across pages.
- Q: What should happen if required audit evidence cannot be recorded for a connector mutation? → A: Connector mutations must fail closed if required audit evidence cannot be recorded.
- Q: Which product surfaces must phase 53 expose for channel management and repair? → A: Phase 53 must expose channel management through API, TypeScript SDK, and web product surfaces.
- Q: What default ordering should channel connector lists use? → A: Default order is action-required, unavailable, and degraded first, then disabled, then ready; stable by display name and connector ID within each group.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Inspect Channel Fleet Health (Priority: P1)

A tenant user with redacted inspection permission can see every hosted channel connector for the active tenant, understand whether each connector is enabled, ready, degraded, action-required, unavailable, or disabled, and identify the next repair step without reading logs or editing raw configuration.

**Why this priority**: Channel connectors are not product-ready if users cannot tell which channels are active, which are broken, and who must act to restore them.

**Independent Test**: Can be tested by seeding a tenant with multiple connectors in healthy, disabled, degraded, action-required, and unavailable states, then confirming the channel list and detail views show status, setup state, health, diagnostic freshness, and remediation guidance with only tenant-authorized redacted evidence.

**Acceptance Scenarios**:

1. **Given** a tenant has Discord, Telegram, Slack, Matrix, or other production channel connectors configured, **When** an authorized user opens channel management, **Then** connectors for the active tenant are listed with channel kind, display name, enablement state, setup state, health summary, diagnostic freshness, routing summary, and next action, and the user can reach all tenant connectors across pages when the connector count exceeds one page.
2. **Given** a connector has an action-required diagnostic, **When** the authorized user opens its detail view, **Then** the user sees the stable reason, remediation owner, suggested next step, affected capability, last checked time, and redacted supporting evidence.
3. **Given** a user is not authorized to inspect a tenant connector, **When** they request the list, detail, diagnostics, route policy, or support evidence, **Then** access is denied without revealing connector existence, provider account details, routes, or diagnostic evidence for inaccessible tenants.

---

### User Story 2 - Disable, Re-enable, And Preserve History (Priority: P1)

A tenant user can disable a channel to stop new inbound work, re-enable it when safe, and preserve prior messages, replies, delivery outcomes, diagnostics, and audit history for support and review.

**Why this priority**: Disabling is the safest immediate operator action during provider incidents, compromised credentials, noisy routes, or tenant policy changes, but it must not destroy evidence needed to diagnose or roll back the decision.

**Independent Test**: Can be tested by disabling a ready connector, delivering representative inbound events afterward, confirming no new agent work is accepted from that connector, re-enabling after validation, and verifying historical evidence remains inspectable throughout.

**Acceptance Scenarios**:

1. **Given** a ready connector has prior message, reply, delivery, route, setup, diagnostic, and audit history, **When** an authorized user disables the connector, **Then** the connector becomes disabled, new inbound work is rejected or ignored safely, background delivery eligibility is blocked, and prior history remains available as redacted evidence.
2. **Given** a disabled connector receives an inbound provider event, **When** routing evaluates the event, **Then** no new agent run is created and the decision is recorded as disabled, ignored, or blocked with redacted tenant-scoped evidence.
3. **Given** a disabled connector has valid setup and route policy after repair checks pass, **When** an authorized user re-enables it, **Then** eligible new inbound work and delivery targeting may resume while historical disabled-state evidence remains auditable.
4. **Given** a user lacks `connectors.manage`, **When** they attempt to disable or re-enable a connector, **Then** the system denies the mutation and does not reveal raw provider details or inaccessible tenant state.

---

### User Story 3 - Repair Or Reconnect A Broken Connector (Priority: P2)

A tenant user can start repair from a diagnostic next step, reconnect provider authorization, rotate credentials when the connector supports it, and return the connector to ready or a clearly explained terminal state.

**Why this priority**: Most connector failures are caused by revoked authorization, missing scopes, provider outages, rate limits, route access changes, or expired setup. The product must guide users from a problem state to a safe recovery action.

**Independent Test**: Can be tested by inducing representative setup, permission, provider, route, rate-limit, network, credential, and authorization failures, then confirming each diagnostic offers an appropriate repair, reconnect, rotate, retry, disable, or support-escalation path that preserves redacted evidence.

**Acceptance Scenarios**:

1. **Given** a connector reports missing authorization, expired authorization, revoked credentials, missing permission, provider approval needed, route access failure, or setup mismatch, **When** an authorized user chooses repair, **Then** the product starts or links to the appropriate setup session and carries forward the connector, tenant, diagnostic reason, and redacted evidence needed to complete recovery.
2. **Given** a connector supports credential rotation or provider reconnection, **When** an authorized user rotates or reconnects it, **Then** future connector use depends on the new validated setup state while previous setup and diagnostic evidence remains redacted and auditable.
3. **Given** repair cannot complete because the provider is unavailable, rate-limited, unsupported, or requires an external administrator action, **When** repair concludes, **Then** the connector lands in unavailable, degraded, disabled, or action-required state with remediation owner, retry safety, and next step.
4. **Given** a repair attempt is cancelled, interrupted, or retried concurrently, **When** the user returns to the connector detail view, **Then** the current state, in-progress or terminal repair state, and prior repair attempts are coherent and recoverable.

---

### User Story 4 - Manage Routes, Allowlists, And Delivery Visibility (Priority: P2)

A tenant user can configure supported channel route policies, sender or conversation allowlists, invocation gates, and background delivery eligibility, then inspect foreground replies and background delivery outcomes separately.

**Why this priority**: Channel reliability depends on clear route boundaries. Users need safe control over who can trigger the agent, where replies go, and whether a connector can be used for scheduled or workflow-originated delivery.

**Independent Test**: Can be tested by changing route policies for at least two connector kinds, verifying accepted and blocked route behavior, and confirming foreground reply outcomes are visible separately from background delivery outcomes and agent execution status.

**Acceptance Scenarios**:

1. **Given** a connector exposes configurable routes or allowlists, **When** an authorized user changes eligible senders, conversations, rooms, channels, invocation gates, or delivery-target eligibility, **Then** future routing decisions use the new policy and the change is audit-visible.
2. **Given** a connector receives a message from an unallowed sender, route, room, channel, or invocation context, **When** routing evaluates it, **Then** no agent run is created and the blocked or ignored decision is inspectable in redacted form.
3. **Given** an accepted inbound message produces a foreground reply, **When** the user or support inspects the connector activity, **Then** the foreground reply outcome is visible separately from the agent execution outcome and from any background delivery outcome.
4. **Given** a connector is selected or removed as a background delivery target, **When** scheduled or workflow-originated work emits a result, **Then** delivery success, retry, suppression, or failure is recorded independently from foreground replies.

---

### User Story 5 - Provide Support Evidence For Incidents (Priority: P3)

Support users with authorized tenant access can inspect metadata-only redacted connector evidence for incidents, including configuration changes, diagnostic history, route decisions, repair attempts, disabled-state behavior, foreground replies, and background deliveries.

**Why this priority**: Support needs evidence that is specific enough to resolve incidents but safe enough to avoid raw secrets, provider payloads, or cross-tenant data exposure.

**Independent Test**: Can be tested by generating connector incidents across setup, routing, disablement, repair, reply, and delivery flows, then verifying authorized support can reconstruct what happened from redacted evidence while unauthorized support cannot inspect it.

**Acceptance Scenarios**:

1. **Given** a connector incident is reported, **When** authorized support opens the connector evidence view, **Then** support can see tenant scope, connector identity, current state, recent state transitions, diagnostic reasons, repair attempts, route decisions, reply outcomes, delivery outcomes, and audit events without message bodies, raw provider payloads, or secrets.
2. **Given** diagnostic or support evidence cannot be confidently redacted, **When** it would otherwise be displayed, **Then** the unsafe detail is suppressed, a redaction-failure audit record is emitted, and support sees only a generic safe classification.
3. **Given** support lacks access to the tenant, **When** they search for or open connector evidence, **Then** the request is denied without exposing connector existence, provider identity, route details, messages, or diagnostic history.

### Edge Cases

- A tenant has no configured production connectors; the management surface must show an empty state and no misleading repair actions.
- A tenant has more connectors than fit on the first channel management page; the user must be able to continue through pages until all tenant connectors are reachable.
- Connector health changes while a user pages through the channel list; pagination must still use a deterministic ordering and avoid missing or duplicated connector entries in the returned page sequence.
- A tenant has multiple connectors of the same kind, such as multiple workspace or room bindings, and each connector must remain separately inspectable, repairable, auditable, and permission-gated.
- A connector is disabled while setup repair, credential rotation, route editing, inbound processing, or background delivery is in progress; disablement takes precedence for new inbound work and new background delivery eligibility until a later validated re-enable succeeds.
- A connector is re-enabled while diagnostics are stale, older than 15 minutes, setup is degraded, route policy is incomplete, or provider authorization has changed since the last check.
- Provider authorization is revoked, expires, loses scope, loses route membership, is blocked by provider approval, or is rate-limited after the connector was previously ready.
- Route policy changes remove the only allowed sender, room, channel, conversation, or delivery target for a connector.
- Inbound work arrives after disablement through provider retry, webhook replay, restart recovery, sync replay, delayed delivery, or duplicated provider event.
- Repair links to a setup session that is cancelled, unavailable, already completed, owned by a different tenant, or no longer compatible with the connector.
- A connector kind does not support credential rotation, route editing, foreground replies, or background delivery; the surface must show unsupported capability instead of a broken action.
- Diagnostic state is stale, unavailable, contradictory across connector sources, or lacks enough evidence to assign a specific reason.
- Support evidence includes raw provider payloads, credentials, tokens, authorization grants, message bodies, route identifiers, or user identifiers that cannot be safely displayed; message bodies and raw provider payloads must never be shown in this phase.
- Connector diagnostic, repair, routing, reply, delivery, or support evidence reaches its default retention limit; it must expire from normal inspection after 90 days unless covered by an authorized longer tenant retention policy.
- Required audit evidence cannot be recorded for a connector mutation; the mutation must fail closed and leave connector state unchanged.
- A permission change occurs while a user has a management or support view open; subsequent reads and mutations must enforce the new permission state.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide a tenant-scoped channel management surface that lists all production channel connectors for the active tenant across one or more pages.
- **FR-001a**: Channel connector lists MUST support pagination with a default page size of 20 and MUST allow authorized users to reach all tenant connectors across pages.
- **FR-001b**: Channel connector lists MUST use a deterministic default order that places action-required, unavailable, and degraded connectors first, then disabled connectors, then ready connectors, with stable ordering by display name and connector ID within each group.
- **FR-001c**: Channel management and repair behavior MUST be available through API, TypeScript SDK, and web product surfaces for list, detail, disable, re-enable, repair, reconnect, supported credential rotation, route policy, diagnostics, reply status, delivery status, and support evidence flows.
- **FR-002**: The channel list MUST show each connector's channel kind, display name, enablement state, setup state, health summary, diagnostic freshness, routing summary, delivery eligibility, and available next action.
- **FR-003**: The connector detail view MUST show current state, setup state, diagnostic summary, remediation owner, next step, selected routes, recent route decisions, foreground reply outcomes, background delivery outcomes, repair attempts, and redacted support evidence.
- **FR-004**: Channel management reads and mutations MUST be permission-gated by tenant, and unauthorized users MUST receive stable denials without connector existence, provider identity, route, diagnostic, or evidence leakage.
- **FR-004a**: Redacted connector list, detail, route, reply, delivery, and support-evidence inspection MUST require `credentials.inspect`.
- **FR-004b**: Connector diagnostic inspection MUST require `integrations.diagnostics.read` in addition to any connector inspection permission needed for the surface.
- **FR-005**: Users with `connectors.manage` permission MUST be able to disable and re-enable connectors that belong to the active tenant.
- **FR-006**: Disabling a connector MUST preserve prior setup, message, route, reply, delivery, diagnostic, support, and audit history.
- **FR-007**: Disabled connectors MUST reject, ignore, or block new inbound work safely and MUST NOT create new agent runs from inbound provider events while disabled.
- **FR-008**: Disabled connectors MUST NOT be eligible for new background deliveries until re-enabled and validated as eligible.
- **FR-009**: Re-enabling a connector MUST require current setup, health, and route policy to be safe enough for the connector's declared capabilities.
- **FR-010**: The system MUST record enablement, disablement, re-enablement, route changes, repair starts, repair completions, reconnects, credential rotations, and permission denials as tenant-scoped audit evidence.
- **FR-010a**: Connector mutations MUST be serialized per connector so concurrent disable, re-enable, route edit, repair, reconnect, credential rotation, and delivery-eligibility changes converge to one auditable connector state.
- **FR-010b**: Disablement MUST take precedence over concurrent or in-progress repair, reconnect, credential rotation, route edit, inbound processing, and background delivery eligibility until a later validated re-enable succeeds.
- **FR-010c**: Connector mutations MUST fail closed and leave connector state unchanged when required audit evidence cannot be recorded.
- **FR-011**: Repair actions MUST be available to users with `connectors.manage` from diagnostic next steps when a connector can be repaired by retrying setup, reconnecting authorization, rotating supported credentials, updating permissions, revalidating route access, or rerunning diagnostics.
- **FR-012**: Repair actions MUST link the connector, tenant, actor, diagnostic reason, setup session, current state, retry safety, and redacted evidence needed to complete or explain the repair.
- **FR-013**: The system MUST represent repair outcomes as ready, degraded, unavailable, disabled, cancelled, or action-required, with no ambiguous terminal state.
- **FR-014**: Credential rotation MUST be available only for connector kinds that support it and only to users with both `connectors.manage` and `secrets.manage`; unsupported rotation MUST be shown as unsupported rather than as a failing repair action.
- **FR-015**: Reconnect and credential-rotation flows MUST require both `connectors.manage` and `secrets.manage`, make future connector use depend on the newly validated setup state, and preserve redacted historical evidence.
- **FR-016**: The system MUST expose connector health and diagnostics using stable reason codes and remediation meanings shared with integration diagnostics and hosted setup sessions.
- **FR-017**: The system MUST mark connector diagnostic state older than 15 minutes as stale and MUST produce current diagnostic truth before presenting a repair result after a user-initiated repair, reconnect, rotate, disable, or re-enable action.
- **FR-018**: Users with `connectors.manage` permission MUST be able to inspect and update supported route policy settings, including eligible senders, rooms, channels, conversations, invocation gates, and background delivery eligibility.
- **FR-019**: Route policy changes MUST affect only future routing and delivery decisions and MUST NOT rewrite historical message, reply, delivery, or diagnostic evidence.
- **FR-020**: The system MUST record every inbound routing decision for managed connectors as accepted, ignored, blocked, duplicate, unsupported, failed, or disabled using the shared channel connector outcome vocabulary.
- **FR-021**: The system MUST present foreground reply outcomes separately from agent execution outcomes and background delivery outcomes.
- **FR-022**: The system MUST present background delivery target eligibility and delivery outcomes separately from foreground reply outcomes and connector setup state.
- **FR-023**: Connector capabilities MUST be explicit so unsupported actions, such as route editing, credential rotation, foreground replies, or background delivery, are shown as unsupported instead of available or broken.
- **FR-024**: Support users with authorized tenant access and the required inspection permissions MUST be able to inspect metadata-only redacted connector incident evidence for setup, health, diagnostics, route decisions, enablement, repair, reply, delivery, and audit events.
- **FR-025**: Support evidence MUST redact raw provider credentials, tokens, authorization grants, provider payloads, message bodies, route identifiers where required, user identifiers where required, logs, fixtures, and diagnostic details before exposure outside the trusted connector boundary.
- **FR-025a**: Support evidence MUST NOT display channel message bodies or raw provider payloads in this phase, even to authorized support users.
- **FR-026**: If connector evidence cannot be confidently redacted, the system MUST suppress unsafe detail, emit redaction-failure audit evidence, and show only a generic safe classification.
- **FR-026a**: Connector diagnostic, repair, routing, reply, delivery, and support evidence MUST use a 90-day default retention period unless an authorized tenant retention policy requires longer retention.
- **FR-027**: The feature MUST NOT add a new channel connector, channel marketplace, mobile push app, memory-driven channel ranking, autonomous provider remediation, or provider-specific behavior outside the existing production connector contracts.
- **FR-028**: Existing connector contracts, setup sessions, diagnostics, routing decisions, delivery outcomes, audit records, and channel conformance meanings MUST remain authoritative for connector behavior.

### Key Entities *(include if feature involves data)*

- **Channel Connector**: A tenant-owned production channel configuration with channel kind, display name, enablement state, setup state, health, capabilities, route policy, diagnostic state, and delivery eligibility.
- **Channel Management Projection**: The user-visible list and detail summary for one connector, composed from authoritative connector state, setup state, diagnostics, route policy, reply outcomes, delivery outcomes, and support evidence, with list pagination metadata when multiple pages exist.
- **Connector Enablement State**: The tenant-scoped state that determines whether new inbound work and background deliveries are allowed, disabled, blocked, or awaiting repair; disablement takes precedence until a later validated re-enable succeeds.
- **Connector Capability**: A declared product behavior supported by a connector kind, such as disablement, repair, reconnect, credential rotation, route editing, foreground replies, or background delivery.
- **Repair Action**: A user-initiated recovery path tied to a diagnostic next step, setup session, reconnect, credential rotation, route revalidation, or diagnostic rerun.
- **Route Policy**: Tenant-scoped connector configuration defining eligible senders, rooms, channels, conversations, invocation gates, blocked routes, and background delivery eligibility.
- **Routing Decision**: Redacted evidence for how an inbound provider event was handled, including accepted, ignored, blocked, duplicate, unsupported, failed, or disabled outcome.
- **Foreground Reply Outcome**: The response-delivery result for an accepted inbound channel message, tracked independently from agent execution and background delivery.
- **Background Delivery Outcome**: The delivery result for scheduled or workflow-originated work sent through a channel connector, tracked independently from foreground replies.
- **Connector Diagnostic Summary**: Stable reason, freshness, remediation owner, next step, affected capability, and redacted evidence for connector health and repair readiness.
- **Support Evidence Bundle**: Metadata-only, redacted, permission-gated incident evidence that combines setup, diagnostic, route, reply, delivery, repair, enablement, audit records, and retention expiry without channel message bodies or raw provider payloads.
- **Connector Audit Record**: Tenant-scoped evidence of connector inspection, mutation, route change, enablement transition, repair attempt, credential rotation, reconnect, denial, redaction failure, or audit-write failure, including the permission gate evaluated for the action.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Adds a product management and support projection across existing production channel connectors, exposed through API, TypeScript SDK, and web product surfaces. Existing connector contracts remain authoritative; this feature depends on their setup, diagnostic, route, delivery, audit, permission, redaction, and conformance meanings instead of replacing them.
- **Migration / Rollback**: No tenant is enrolled into a new connector. Rollout can expose the management surface behind channel-management availability while preserving existing connector behavior. Rollback hides new management and repair surfaces and blocks new management mutations while preserving connector state, disablement state already applied, setup history, diagnostics, route decisions, delivery outcomes, and audit evidence for authorized review.
- **Verification Strategy**: Required validation includes API, TypeScript SDK, and web product coverage for channel list and detail, permission denials, disable and re-enable behavior, disabled inbound suppression, delivery eligibility blocking, repair-to-setup linkage, reconnect and supported credential rotation flows, route policy updates, foreground reply and background delivery separation, stale diagnostic handling, redaction tests, support evidence inspection, audit evidence, restart recovery, and conformance regression after disable and re-enable for at least two connector kinds.
- **Observability Impact**: The feature must make connector state transitions, route changes, repair actions, diagnostic freshness, disabled inbound decisions, foreground reply outcomes, background delivery outcomes, permission denials, redaction failures, support evidence, and retention expiry available to authorized users and operators without requiring raw log access.
- **Environment & Secrets**: Development and automated verification must default to the test environment. Live connector credentials and production tenants are not used by default. Any live smoke walkthrough must use explicitly approved safe tenant-owned connector accounts, redact provider evidence, and avoid exposing raw secrets, tokens, authorization grants, or provider payloads.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Authorized users can identify the current status, health, diagnostic freshness, and next action for 100% of configured tenant production connectors from the channel management surface within 2 minutes.
- **SC-001a**: For tenants with more than 20 configured production connectors, authorized users can reach 100% of tenant connectors across paginated channel management results with no missing or duplicated connector entries.
- **SC-001b**: 100% of paginated channel connector list tests return connectors in deterministic default order: action-required, unavailable, and degraded connectors first, then disabled connectors, then ready connectors, with stable display-name and connector-ID ordering inside each group.
- **SC-002**: 100% of supported disable actions prevent new inbound provider events from creating agent runs and prevent new background delivery use while preserving historical evidence for inspection.
- **SC-003**: 100% of supported re-enable actions require current safe setup, health, and route eligibility before inbound work or background delivery can resume.
- **SC-003a**: 100% of concurrent connector mutation tests converge to one auditable connector state, and disablement prevents new inbound work and background delivery eligibility until a later validated re-enable succeeds.
- **SC-003b**: 100% of connector mutation attempts with required audit-write failure are denied without changing connector state.
- **SC-004**: At least 95% of representative connector failures for authorization, permission, setup, route access, provider outage, rate limit, network failure, and unsupported behavior show a stable reason, remediation owner, next step, and repair or support path without raw provider error text.
- **SC-004a**: 100% of connector inspection views and repair decisions mark diagnostic state older than 15 minutes as stale and refresh diagnostic truth before completing user-initiated repair, reconnect, rotate, disable, or re-enable actions.
- **SC-005**: 100% of repair, reconnect, and supported credential-rotation flows end in ready, degraded, unavailable, disabled, cancelled, or action-required state with auditable redacted evidence.
- **SC-006**: 100% of route policy changes in the verification suite affect future routing and delivery decisions without rewriting historical route, reply, delivery, diagnostic, or audit evidence.
- **SC-007**: Foreground reply outcomes and background delivery outcomes remain separately inspectable for 100% of connector events and deliveries exercised by the verification suite.
- **SC-008**: 100% of unauthorized list, detail, mutation, repair, route, and support-evidence attempts are denied without exposing inaccessible tenant connector existence or provider details.
- **SC-009**: Redaction validation finds zero raw provider credentials, tokens, authorization grants, raw provider payloads, channel message bodies, and prohibited route details in user-facing, support-facing, test, log, fixture, and audit output.
- **SC-010**: Authorized support can reconstruct current state, latest diagnostic reason, last repair attempt, recent route decisions, disabled-state behavior, and reply or delivery outcomes for a representative connector incident within 5 minutes.
- **SC-010a**: 100% of connector diagnostic, repair, routing, reply, delivery, and support evidence in retention tests expires from normal inspection after 90 days unless covered by an authorized longer tenant retention policy.
- **SC-011**: Manual test-environment walkthrough covers at least two connector kinds and verifies list, detail, disable, re-enable, repair, route inspection or update, diagnostic summary, foreground reply status, background delivery status, and support evidence.
- **SC-012**: API, TypeScript SDK, and web product verification each cover list, detail, disable, re-enable, repair, diagnostics, and support-evidence flows for at least one representative connector.

## Assumptions

- Roadmaps 48-52 provide the production channel connector conformance behavior and connector-specific setup, routing, diagnostics, redaction, and delivery outcome contracts needed for this management surface.
- Roadmap 46 hosted credential and OAuth setup sessions are available for connector repair, reconnect, and supported credential rotation flows.
- Roadmap 42 integration diagnostics provide the stable diagnostic reason-code and remediation model reused by connector repair.
- This phase manages existing production connectors only; it does not add a new connector, marketplace, mobile push app, or memory-based channel ranking.
- Channel state is tenant-scoped and permission-gated for both product users and support users.
- Disabling a connector is reversible and preserves history, but it must block new inbound work and new background delivery eligibility until re-enabled safely.
- Existing connector contracts, setup sessions, diagnostics, routing decisions, delivery outcomes, and audit records remain the source of truth; the management UX presents and coordinates them.
- Default verification uses the test environment and fake connector evidence. Live connector walkthroughs are optional unless a later release-readiness gate explicitly requires approved safe accounts.
