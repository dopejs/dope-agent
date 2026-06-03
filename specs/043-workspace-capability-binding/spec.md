# Feature Specification: Workspace And Capability Binding

**Feature Branch**: `043-workspace-capability-binding`  
**Created**: 2026-05-13  
**Status**: Draft  
**Input**: User description: "$speckit-specify 结合 docs/specs/043-workspace-and-capability-binding.md 完成 phase 58 的工作"

**Upstream authority**: `docs/specs/043-workspace-and-capability-binding.md` is the authoritative upstream document for this work (Roadmap 58). This specification translates that document into testable scenarios, requirements, and success criteria. Where the upstream document and this spec disagree, the upstream document wins and this spec must be updated.

**Dependencies**: Roadmap 57 (`specs/042-agent-profile-persona/spec.md`) provides structured agent profile and persona configuration. Roadmap 48 (`specs/033-channel-connector-conformance/spec.md`) provides channel connector conformance and channel identity expectations. Roadmap 37 (`specs/022-hosted-secrets-isolation/spec.md`) provides hosted secrets, integration account, and connector isolation foundations.

## Clarifications

### Session 2026-06-03

- Q: How should the tenant Workspace be represented in storage (resource vs projection)? → A: Persisted resource — a lightweight tenant-owned record (id, label, status, owner); the default personal workspace is auto-created as such a record.
- Q: When new work starts from a binding referencing an invalid/unavailable profile or workspace, what is the default runtime behavior? → A: Fail closed — block new work with a safe repair-required outcome and evidence; never silently substitute.
- Q: Which binding scopes can authorized users directly set capability visibility on this phase? → A: Profile and workspace scopes only; tenant and connector remain higher-level policy limits that are enforced (strictest wins) but not user-edited here.
- Q: How is the default personal workspace created for existing tenants? → A: Lazily and idempotently on first binding/resolution access (no bulk migration touching all tenants at rollout).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Bind Channels And Accounts To Profile And Workspace Defaults (Priority: P1)

As a tenant user, I can bind a channel or integration account to an agent profile and workspace so conversations from different places use the right identity, defaults, and workspace context without relying on hidden memory or ad hoc operator knowledge.

**Why this priority**: A personal agent cannot safely apply persona, workspace, or capability policy to channel-originated work until the applicable bindings are explicit tenant-owned product state.

**Independent Test**: Can be tested by creating a default workspace, binding a channel to a profile and workspace, configuring an integration-account default, starting new work from each source, and confirming new work resolves the expected profile and workspace while existing historical evidence remains unchanged.

**Acceptance Scenarios**:

1. **Given** a tenant has a default profile and workspace, **When** an authorized user binds a channel to a different active profile and workspace, **Then** new work from that channel resolves to the bound profile and workspace.
2. **Given** an integration account has a profile default and a channel under that account has no channel-specific binding, **When** new work starts from that channel, **Then** the integration-account default applies.
3. **Given** a channel has an explicit binding, **When** the integration-account default changes later, **Then** new work from the explicitly bound channel keeps using the channel binding until it is changed or removed.
4. **Given** a user belongs to another tenant or lacks binding-management permission, **When** they attempt to create, update, or remove a binding, **Then** the request is denied without exposing inaccessible binding details.

---

### User Story 2 - Control Capability Visibility And Default Enablement (Priority: P1)

As a tenant user, I can hide, disable, or default-enable capabilities for an agent profile or workspace so risky tools and integrations are not visible or executable where they are not intended.

**Why this priority**: Capability visibility must be policy-backed product truth before runtime can safely offer actions to the agent or explain why an action was unavailable.

**Independent Test**: Can be tested by hiding and disabling representative capabilities for a profile and workspace, starting new work under those bindings, and confirming hidden or disabled capabilities are absent from visible choices and cannot execute even if requested directly.

**Acceptance Scenarios**:

1. **Given** a capability is hidden for the active profile, **When** new work starts under that profile, **Then** the capability is not visible as available for that work.
2. **Given** a capability is disabled for the bound workspace, **When** a user or runtime attempts to use it, **Then** execution is denied with safe evidence explaining the disabled workspace policy.
3. **Given** a capability is default-enabled for a profile but prohibited by tenant or connector policy, **When** capability visibility is resolved, **Then** the stricter policy wins and the capability is not executable.
4. **Given** an authorized user inspects a binding, **When** they review capability visibility, **Then** they can see which capabilities are visible, hidden, disabled, or blocked by higher-level policy without seeing secrets or raw integration payloads.

---

### User Story 3 - Inspect Runtime Binding Evidence (Priority: P1)

As an operator, I can inspect which profile, workspace, binding rule, integration account default, and capability set influenced a run so behavior and denied capabilities can be explained after the fact.

**Why this priority**: Binding state is only operationally trustworthy if runtime evidence records what was selected and why, including after binding changes, daemon restart, or client reconnect.

**Independent Test**: Can be tested by starting work before and after binding changes, inspecting runtime evidence for each run, and confirming each record shows the active binding identities, selected profile and workspace, capability visibility summary, and safe denial reasons where applicable.

**Acceptance Scenarios**:

1. **Given** a run starts from a bound channel, **When** an operator inspects the run, **Then** the evidence shows the channel binding, selected profile, selected workspace, and active capability set that influenced execution.
2. **Given** a capability is hidden or disabled, **When** runtime denies use of that capability, **Then** the evidence records the policy outcome and safe reason without exposing sensitive inputs.
3. **Given** a binding changes after a run completes, **When** an operator inspects the historical run, **Then** historical evidence still shows the binding selection that applied when the run started.
4. **Given** the daemon restarts after binding changes, **When** operators inspect bindings and runtime evidence, **Then** binding state and historical selections remain durable and explainable.

---

### User Story 4 - Manage Bindings Through Product And Client Surfaces (Priority: P2)

As a user or client integrator, I can list, inspect, create, update, disable, and remove bindings through supported product surfaces so channel, account, profile, workspace, and capability policy can be managed without direct store edits.

**Why this priority**: Binding state affects production behavior and must be manageable, auditable, and compatible for users, operators, and client integrations.

**Independent Test**: Can be tested by managing bindings and capability visibility through product and client integration flows, then confirming validation, audit records, permission checks, and backward-compatible responses for clients that do not yet use binding features.

**Acceptance Scenarios**:

1. **Given** an authorized user opens the binding management surface, **When** they inspect channel and account bindings, **Then** they see safe labels, scope, status, selected profile, selected workspace, capability visibility summary, and last material change.
2. **Given** a binding references an inactive profile, unavailable workspace, removed channel, or disconnected integration account, **When** an authorized user inspects it, **Then** the binding is marked as needing repair and cannot silently affect new work.
3. **Given** an older client does not understand binding fields, **When** it continues using existing channel and profile flows, **Then** default behavior remains compatible and does not require client-side binding awareness.

---

### User Story 5 - Preserve Explicit Non-Memory And Non-Filesystem Boundaries (Priority: P2)

As a tenant user or operator, I can trust that workspace and capability bindings do not create memory-backed knowledge, physical filesystem access, marketplace behavior, or autonomous capability selection beyond explicit policy.

**Why this priority**: Roadmap 58 is the product-state foundation for later context and memory work. Expanding it into knowledge, storage migration, or marketplace behavior would make rollout and audit boundaries unclear.

**Independent Test**: Can be tested by binding a workspace, attempting to infer memory or filesystem access from that binding alone, and confirming the system records explicit binding state only; separate capabilities remain required for file access, knowledge retrieval, marketplace discovery, or autonomous selection.

**Acceptance Scenarios**:

1. **Given** a workspace is bound to a channel, **When** new work starts from that channel, **Then** the workspace identity is available as binding evidence but no filesystem access is granted unless an allowed capability separately grants it.
2. **Given** users discuss preferences or workspace facts in conversation, **When** future work starts, **Then** bindings are not automatically changed and no memory-backed workspace knowledge is created by this phase.
3. **Given** a capability is not visible under the active policy, **When** the agent attempts to choose capabilities autonomously, **Then** it can only choose from policy-visible capabilities.

### Edge Cases

- A tenant has no explicit workspace or binding yet; new work must resolve to safe default profile and workspace behavior without breaking existing users.
- A channel binding, integration-account default, and tenant default could all apply; resolution must be deterministic and inspectable.
- A binding references an archived, disabled, deleted, or invalid profile or workspace; new work MUST fail closed with a safe repair-required outcome and evidence, and MUST NOT silently substitute a different profile or workspace. Recovery happens only through an explicit safe repair action.
- A binding references an unavailable workspace, removed channel, disconnected integration account, or connector that no longer supports workspace binding.
- A capability is default-enabled in one scope but hidden, disabled, or prohibited by another scope or tenant policy; the stricter effective policy must win.
- A binding or capability policy changes while runs are starting; each affected run must record one resolved selection and must not mix partial state.
- Existing runs, threads, sessions, and connector evidence predate binding support; inspection must distinguish legacy default behavior from explicit binding evidence.
- Cross-tenant identifiers, raw integration account details, secrets, provider payloads, message bodies, and unsafe capability inputs appear in source data; binding inspection and evidence must redact or summarize them safely.
- A user attempts to bind resources across tenants, bind a channel they cannot manage, enable a capability they cannot use, or inspect hidden capability policy without permission.
- Runtime is restarted or clients reconnect after binding changes; binding state, audit events, and runtime evidence must remain durable and consistent.
- A workspace binding is mistaken for filesystem access; the system must deny file operations unless a separate visible and enabled capability authorizes them.
- Live connectors exist but are not approved for test use; automated verification must use the test environment and fake connector evidence by default.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST treat profile, workspace, channel, integration account, and capability bindings as explicit tenant-owned product configuration, not memory, learned preference state, hidden prompt truth, or connector-only metadata.
- **FR-002**: The system MUST provide a tenant-scoped persisted workspace resource (a lightweight tenant-owned record carrying at minimum a stable identity, safe label, status, and owning tenant) sufficient to select, display, audit, validate, and repair workspace bindings. The default personal workspace MUST be represented as such a persisted record.
- **FR-003**: Each tenant MUST have deterministic default profile and workspace resolution for new work when no more specific binding exists.
- **FR-004**: Authorized users MUST be able to create, read, list, update, disable, and remove channel-to-profile and channel-to-workspace bindings within their tenant.
- **FR-005**: Authorized users MUST be able to configure integration-account-to-profile defaults where a channel or connector source does not have a more specific binding.
- **FR-006**: Binding resolution MUST use a deterministic precedence order: channel binding first, then integration-account default, then tenant default.
- **FR-007**: Binding reads, binding runtime inspection, and capability visibility inspection MUST require `bindings.inspect`, and unauthorized attempts MUST be denied without revealing inaccessible binding existence or details.
- **FR-008**: Binding creation, update, disablement, removal, repair, and capability visibility or default-enablement changes MUST require `bindings.manage`, and unauthorized attempts MUST leave binding state unchanged.
- **FR-009**: Binding validation MUST reject cross-tenant resources, unavailable channels, disconnected integration accounts, archived or disabled profiles, unavailable workspaces, unsupported connector binding surfaces, invalid precedence conflicts, malformed values, and policy-conflicting selections with safe user-visible reasons.
- **FR-010**: Binding changes MUST emit audit and event records containing tenant, actor, scope, affected resource labels, previous selection summary, resulting selection summary, timestamp, and outcome.
- **FR-011**: Binding mutations MUST fail closed and leave binding state unchanged when required audit or event evidence cannot be recorded.
- **FR-012**: The system MUST preserve historical binding evidence and MUST NOT silently rewrite prior run, thread, session, workflow, handoff, channel, or connector evidence when bindings change.
- **FR-013**: Runtime records for new work MUST include active binding identities where they influenced execution, including selected profile identity and version, selected workspace identity, selected channel or integration-account binding, and capability visibility summary.
- **FR-014**: Runtime evidence for denied or hidden capabilities MUST include the safe policy reason and applicable binding scope without exposing secrets, raw provider payloads, unsafe user content, or cross-tenant details.
- **FR-015**: Capability visibility MUST be enforced by policy before a capability is shown to the agent, user, operator, or client integration as available for the active binding.
- **FR-016**: Hidden or disabled capabilities MUST NOT execute even if requested directly by a user, agent, client integration, connector payload, replay, or stale runtime state.
- **FR-017**: Effective capability availability MUST respect the strictest applicable policy across (a) tenant and connector limits enforced as higher-level constraints and (b) the profile and workspace capability visibility policies that are user-editable in this phase. Channel and integration-account bindings select the profile and workspace for new work but do NOT themselves carry capability-visibility constraints in this phase; any capability limit implied by a channel or account is enforced through the resolved profile/workspace policy and the tenant/connector limits, not through a separate channel- or account-scoped capability policy.
- **FR-018**: Authorized users MUST be able to mark capabilities visible, hidden, disabled, and default-enabled at the profile and workspace binding scopes (the user-editable scopes for this phase) while preserving higher-level policy limits. Tenant and connector policy are enforced as higher-level limits under FR-017 but are NOT user-edited capability-visibility scopes in this phase.
- **FR-019**: Capability default enablement MUST influence what is offered by default but MUST NOT override hidden, disabled, unavailable, disallowed, or unconfigured capability policy.
- **FR-020**: Workspace binding MUST NOT grant filesystem, repository, document, connector, or knowledge access unless a separate visible and enabled capability grants that access.
- **FR-021**: Workspace binding MUST NOT create memory-backed workspace knowledge, long-term personalization, learned preferences, or autonomous workspace fact extraction.
- **FR-022**: The system MUST expose repair status for bindings that reference unavailable profiles, workspaces, channels, integration accounts, connectors, or capabilities.
- **FR-023**: Binding lifecycle and capability visibility behavior MUST be available through product surfaces and client-facing contracts needed by users, operators, and integrations.
- **FR-024**: Client-facing binding changes MUST preserve backward-compatible default behavior for clients that do not yet understand explicit binding fields.
- **FR-025**: Existing behavior MUST map to one default personal profile and one default personal workspace for each eligible tenant to avoid breaking current users. The default personal workspace record MUST be provisioned lazily and idempotently on first binding or resolution access for the tenant (no bulk migration touching all tenants at rollout), and concurrent first-access provisioning MUST converge on a single record.
- **FR-026**: Legacy or partially inferred binding evidence MUST be labeled as default or partial rather than presented as an explicit user-configured binding.
- **FR-027**: Binding state, capability visibility, audit evidence, repair status, and runtime binding projections MUST survive daemon restart and client reconnect.
- **FR-028**: Binding inspection, runtime evidence, tests, fixtures, logs, and audit output MUST redact or summarize secrets, tokens, raw provider payloads, sensitive message bodies, unsafe capability inputs, and cross-tenant identifiers.
- **FR-029**: Documentation and operator-facing evidence MUST make clear that this phase does not include memory-backed workspace knowledge, per-tenant physical workspace storage migration, community marketplace behavior, or autonomous capability selection beyond policy-visible capabilities.
- **FR-030**: Automated verification MUST cover binding lifecycle, workspace lifecycle, tenant isolation, permission denial, validation failures, precedence resolution, audit-write failure behavior, capability visibility enforcement, hidden capability denial, runtime evidence, historical evidence preservation, concurrent policy-change atomicity (one resolved selection per work item), repair states, restart recovery, client contract compatibility, product surface behavior, redaction, fail-closed resolution of invalid bindings, lazy default-workspace provisioning, and explicit non-use of memory or filesystem access.
- **FR-031**: When binding resolution for new work selects a profile or workspace that is archived, disabled, removed, unavailable, or otherwise invalid, the system MUST fail closed: new work MUST be blocked with a safe repair-required outcome and runtime evidence, and the system MUST NOT silently substitute a different profile, workspace, or lower-precedence binding. Affected work proceeds only after an explicit safe repair action restores a valid selection.
- **FR-032**: Authorized users with `bindings.manage` MUST be able to create, archive, and disable tenant-scoped workspace records beyond the lazily provisioned default personal workspace. Workspace lifecycle changes MUST be validated, audited, restart-safe, and reversible (archive/disable rather than hard delete while runtime evidence may reference the workspace), consistent with binding-rule lifecycle behavior.
- **FR-033**: When binding or capability visibility policy changes while new work is starting, each affected work item MUST record exactly one resolved binding selection and MUST NOT mix partial or interleaved state from before and after the change.

### Key Entities *(include if feature involves data)*

- **Workspace**: A tenant-scoped persisted product record (stable identity, safe label, status, owning tenant) used for binding identity, safe display, status, audit, repair, and future context inputs without granting storage or filesystem access by itself. The default personal workspace is one such record, provisioned lazily on first access.
- **Binding Rule**: Tenant-owned configuration that connects a binding scope to a selected profile, workspace, or capability visibility outcome.
- **Channel Binding**: A binding rule that applies to new work originating from a specific tenant-owned channel.
- **Integration Account Default Binding**: A binding rule that supplies default profile selection for new work associated with an integration account when no channel-specific binding applies.
- **Capability Visibility Policy**: Tenant-owned policy state describing whether a capability is visible, hidden, disabled, default-enabled, or blocked by a stricter policy for a binding scope.
- **Effective Binding Selection**: The resolved profile, workspace, binding scope, and capability visibility set that applies when new work starts.
- **Runtime Binding Evidence**: Durable evidence attached to run, thread, session, workflow, handoff, channel, or connector inspection showing which binding selections influenced execution.
- **Binding Audit Event**: Tenant-scoped evidence for binding creation, update, disablement, removal, repair, validation failure, permission denial, capability visibility change, and runtime selection outcomes.
- **Binding Repair Status**: Safe user-facing state that marks a binding as healthy, disabled, invalid, stale, unsupported, or needing repair.
- **Capability**: A tenant-visible action or integration surface that can be made visible, hidden, disabled, or default-enabled by binding policy, subject to higher-level policy and connector availability.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Adds a tenant-scoped persisted workspace record, binding lifecycle, capability visibility, audit, runtime evidence, product surface, and client contract behavior. Existing tenant-default profile and channel behavior must continue by mapping current behavior to a default personal workspace and default capability visibility where possible.
- **Migration / Rollback**: Rollout can begin with read-only default binding projection and runtime evidence, then enable binding mutations and capability visibility controls after audit and restart behavior are durable. Rather than a bulk migration, the default personal workspace record is created or identified lazily and idempotently on a tenant's first binding or resolution access, preserving current tenant-default profile behavior. Operational rollback disables new binding mutations and capability visibility changes while preserving already-recorded binding state, audit events, and runtime evidence for inspection.
- **Verification Strategy**: Required validation includes binding lifecycle, precedence resolution, tenant isolation, permission denial, validation failure behavior, audit-write failure behavior, capability visibility enforcement, hidden capability denial, default workspace mapping, runtime evidence on new work, historical evidence preservation, repair status, restart recovery, client contract compatibility, product surface coverage, redaction, and explicit non-use of memory or filesystem access.
- **Observability Impact**: Operators must gain binding lifecycle events, capability visibility decisions, denied-capability evidence, active binding runtime projections, repair status, validation failures, permission denials, audit-write failures, and redaction-limited summaries as product evidence without relying on raw logs or hidden connector state.
- **Environment & Secrets**: Development and automated verification must default to the repository test environment. Live connectors and production tenants must not be touched by default. Secrets, tokens, raw provider payloads, unsafe capability inputs, sensitive message bodies, and cross-tenant identifiers must not be exposed in tests, fixtures, logs, audit output, or binding inspection.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of binding lifecycle tests prove users with `bindings.inspect` can inspect binding state and runtime binding evidence, users with `bindings.manage` can create, update, disable, remove, and repair bindings within their tenant, and unauthorized users learn nothing about inaccessible bindings.
- **SC-002**: 100% of precedence tests prove channel bindings override integration-account defaults, integration-account defaults override tenant defaults, and every new work item records the resolved selection.
- **SC-003**: 100% of workspace binding tests prove workspace identity is selected and visible without granting filesystem, repository, document, connector, or knowledge access by itself (the broader non-use guarantee is covered by SC-015).
- **SC-004**: 100% of capability visibility tests prove hidden, disabled, unavailable, and higher-policy-blocked capabilities are not visible as available under the active binding.
- **SC-005**: 100% of denied-capability tests prove hidden or disabled capabilities cannot execute through direct user requests, agent choice, client integration calls, connector payloads, replay, or stale runtime state.
- **SC-006**: 100% of binding change tests create audit and event evidence with actor, tenant, scope, affected resource labels, previous selection summary, resulting selection summary, timestamp, and outcome.
- **SC-007**: 100% of audit-write failure tests prove binding mutations fail closed and leave binding state unchanged when required evidence cannot be recorded.
- **SC-008**: 100% of runtime evidence tests for supported channel, thread, session, run, workflow, and handoff starts record active profile, active workspace, active binding scope, and capability visibility summary where those selections influenced execution.
- **SC-009**: 100% of historical evidence tests prove binding and capability visibility changes do not rewrite previous run, thread, session, workflow, handoff, channel, or connector evidence.
- **SC-010**: 100% of restart recovery tests preserve binding records, default workspace mapping, capability visibility, audit events, repair status, and runtime binding evidence after daemon restart and client reconnect.
- **SC-011**: 100% of invalid binding tests mark unavailable profile, workspace, channel, integration account, connector, and capability references as denied or needing repair with safe user-visible reasons.
- **SC-012**: Authorized operators can determine why a representative capability was visible, hidden, disabled, or denied for a run from product evidence within 5 minutes.
- **SC-013**: 100% of compatibility tests prove existing users without explicit bindings continue to get default personal profile, default personal workspace, and compatible capability behavior.
- **SC-014**: Redaction validation finds zero exposed secrets, tokens, raw provider payloads, unsafe capability inputs, sensitive message bodies, or cross-tenant identifiers in user-facing, support-facing, test, fixture, log, and audit output.
- **SC-015**: Verification confirms this phase creates no memory-backed workspace knowledge, physical workspace storage migration, community marketplace behavior, autonomous capability selection beyond policy-visible capabilities, or filesystem access from workspace binding alone in 100% of covered flows.

## Assumptions

- Roadmap 57 provides tenant-owned active profiles, profile versions, profile permissions, profile audit evidence, and tenant-default active profile behavior.
- Roadmap 48 provides connector identity and channel conformance expectations sufficient to bind tenant-owned channels without redefining connector routing or dedupe semantics.
- Roadmap 37 provides hosted integration account isolation and safe integration-account identity needed for account-level defaults.
- Binding management introduces dedicated `bindings.inspect` and `bindings.manage` tenant permissions rather than reusing profile, connector, credential, or tenant administrator permissions.
- Binding precedence is channel binding, then integration-account default, then tenant default.
- Each eligible tenant can have one safe default personal workspace record, provisioned lazily on first access, before a user creates explicit workspace choices.
- Capabilities are existing product-visible action or integration surfaces; this phase controls visibility and default enablement but does not create a plugin marketplace.
- Capability visibility is an allowability and presentation decision, not a guarantee that a capability will succeed once invoked.
- Automated verification uses fake or test-environment connector, channel, profile, workspace, and capability evidence by default. Live connector validation is optional unless a later release-readiness gate explicitly requires approved safe accounts.
