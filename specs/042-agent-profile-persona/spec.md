# Feature Specification: Agent Profile And Persona Configuration

**Feature Branch**: `042-agent-profile-persona`  
**Created**: 2026-05-12  
**Status**: Draft  
**Input**: User description: "$speckit-specify 结合 docs/specs/042-agent-profile-and-persona-configuration.md 完成 phase 57 的工作"

**Upstream authority**: `docs/specs/042-agent-profile-and-persona-configuration.md` is the authoritative upstream document for this work (Roadmap 57). This specification translates that document into testable scenarios, requirements, and success criteria. Where the upstream document and this spec disagree, the upstream document wins and this spec must be updated.

**Dependencies**: Roadmap 45 (`specs/030-hosted-tenant-activation/spec.md`) provides hosted signup and tenant activation. Roadmap 54 (`specs/039-thread-session-lifecycle/spec.md`) provides daemon-owned thread and session lifecycle truth. Roadmap 56 (`specs/041-group-room-reset-handoff/spec.md`) provides group, room, reset, and handoff semantics that must preserve active profile evidence.

## Clarifications

### Session 2026-05-12

- Q: Which tenant permissions gate profile reads, runtime inspection, edits, activation, and rollback? → A: Add dedicated `profiles.inspect` and `profiles.manage` permissions for profile reads, runtime inspection, edits, activation, and rollback.
- Q: What active profile selection scope belongs to phase 57? → A: Phase 57 supports one tenant-default active profile; channel, workspace, integration-account, and capability binding is deferred to Roadmap 58.
- Q: How long are profile versions retained for rollback and behavior forensics? → A: Keep all profile versions while the profile exists; rollback eligibility is determined by current validation and policy.
- Q: Does phase 57 support hard deletion of profiles? → A: Phase 57 supports archive or disable only; no hard delete while runtime evidence may reference the profile.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create, Edit, Archive, And Disable Agent Profiles (Priority: P1)

As a tenant user, I can create, edit, archive, and disable a structured agent profile that controls how my agent identifies itself, presents its tone, uses default preferences, and applies safety defaults without relying on hidden prompt files as the source of truth.

**Why this priority**: Persona and identity must be product state before later workspace, capability, and memory work can safely depend on them.

**Independent Test**: Can be tested by creating a tenant profile, editing display identity, persona fields, default provider preferences, safety defaults, and overlay references, archiving or disabling a retired profile, then confirming the saved and retired profile states are schema-backed, visible in product surfaces, isolated from other tenants, and never hard-deleted while runtime evidence may reference them.

**Acceptance Scenarios**:

1. **Given** a tenant has no custom agent profile, **When** an authorized user creates one with valid identity, persona, default preference, safety, and overlay reference fields, **Then** the profile becomes visible as tenant-owned structured configuration.
2. **Given** an existing profile has editable fields, **When** an authorized user updates those fields, **Then** the profile records a new version while preserving prior version evidence.
3. **Given** an active profile should no longer be used for new work, **When** an authorized user archives or disables it, **Then** the profile remains inspectable with retained versions and audit evidence but cannot be selected for new work.
4. **Given** a user belongs to another tenant or lacks `profiles.manage`, **When** they attempt to create, update, archive, or disable a profile, **Then** the request is denied without exposing inaccessible profile details.

---

### User Story 2 - Apply Active Profiles To Runtime Evidence (Priority: P1)

As an operator, I can see which tenant-default profile was active for a thread, session, run, workflow start, or handoff destination so behavior changes can be explained without guessing which prompt files or defaults were in effect.

**Why this priority**: Profiles only become reliable runtime truth if active execution evidence records the profile identity that influenced a conversation or run.

**Independent Test**: Can be tested by selecting the tenant-default active profile, starting conversations, sessions, runs, workflows, and handoff destinations from supported surfaces, changing the tenant-default active profile, then confirming historical runtime evidence retains its original active profile identity while new work uses the current tenant default.

**Acceptance Scenarios**:

1. **Given** a tenant has a tenant-default active profile, **When** a new thread, session, run, workflow, or handoff destination starts, **Then** the runtime evidence records the active profile identity and version visible to authorized inspection.
2. **Given** a profile is edited after runtime work has completed, **When** an operator inspects the historical evidence, **Then** the evidence still points to the profile identity and version that were active when the work started.
3. **Given** a conversation is handed off across supported surfaces, **When** the destination work starts, **Then** the destination records the active profile identity without rewriting source-thread profile evidence.

---

### User Story 3 - Inspect Profile History And Roll Back Changes (Priority: P1)

As a user or operator, I can inspect profile history and roll back to a prior profile version when a persona, default, or safety configuration change causes undesirable behavior.

**Why this priority**: Profile changes directly affect agent behavior. Operators need auditable history, clear rollback, and durable evidence for support and incident review.

**Independent Test**: Can be tested by making multiple profile changes, inspecting version history and audit events, rolling back to a prior version, and confirming future work uses the restored version while historical run evidence remains unchanged.

**Acceptance Scenarios**:

1. **Given** a profile has multiple versions, **When** an authorized user views history, **Then** each version shows safe change summaries, actor, time, status, and rollback eligibility.
2. **Given** an authorized user rolls back a profile to a prior version, **When** future work starts, **Then** the restored version becomes the active profile version and the rollback is auditable.
3. **Given** rollback audit evidence cannot be recorded, **When** rollback is requested, **Then** the rollback fails closed and the active profile remains unchanged.

---

### User Story 4 - Manage Overlay References Explicitly (Priority: P2)

As a user, I can attach editable prompt or workspace files as explicit overlay references so local workflows remain usable while structured profile configuration remains the primary runtime truth.

**Why this priority**: Existing prompt-file workflows should not break, but hidden file-shaped truth must not control persona and defaults without visibility.

**Independent Test**: Can be tested by attaching, updating, removing, and inspecting overlay references, then confirming the product shows which overlays are referenced and that missing or unsafe overlays fail with clear profile validation or runtime evidence.

**Acceptance Scenarios**:

1. **Given** a profile references an editable overlay file, **When** an authorized user inspects the profile, **Then** the overlay is listed as an explicit reference rather than hidden runtime truth.
2. **Given** an overlay reference is missing, inaccessible, unsafe, or outside the allowed scope, **When** a profile is saved or used for new work, **Then** the system blocks or marks the reference with a clear safe failure reason.
3. **Given** existing local prompt or config behavior is present, **When** profiles are introduced, **Then** compatible behavior maps to explicit profile or overlay references where possible without silently changing active behavior.

---

### User Story 5 - Preserve Non-Memory Scope (Priority: P2)

As a tenant user or operator, I can trust that profile configuration changes do not create learned preferences, memory retrieval, agent-generated self-mutation, or knowledge behavior before those capabilities have their own specification.

**Why this priority**: This phase is a structured configuration foundation. Blurring persona configuration into memory or agent self-modification would make later context engineering unsafe and unauditable.

**Independent Test**: Can be tested by creating profile preferences, changing behavior over several conversations, and confirming future work uses only explicit profile configuration and overlay references, not inferred or learned preferences.

**Acceptance Scenarios**:

1. **Given** a user repeatedly expresses a preference in conversation, **When** future work starts, **Then** the profile is not automatically changed by the agent or by inferred memory behavior.
2. **Given** a profile includes persona and default fields, **When** runtime context is assembled for new work, **Then** those fields are treated as structured configuration, not as memory retrieval or knowledge recall.
3. **Given** an operator inspects a profile, **When** they review its history, **Then** every profile change has an explicit actor and audit trail rather than an implicit learned update.

### Edge Cases

- A tenant has no explicit profile yet; the system must provide a safe default or activation path without implying that prompt files are primary runtime truth.
- Two tenants use profiles with the same display name; profile identity, history, and runtime evidence must remain tenant-scoped and unambiguous.
- A profile edit happens while runs or thread activity are starting; each affected run or thread must record one auditable active profile identity and version.
- A profile is edited, rolled back, archived, or disabled while older threads remain inspectable; retained profile versions and historical runtime evidence must not be rewritten or lost.
- A user attempts to set unsupported, malformed, overly large, unsafe, or policy-conflicting persona, default preference, safety, or overlay reference values.
- A default provider preference points to a provider that is unavailable, disabled, quota-limited, or no longer allowed for the tenant.
- Overlay files are renamed, removed, permission-denied, outside the allowed workspace, too large, contain unsafe content, or cannot be confidently redacted.
- Local prompt-file behavior exists before profiles are enabled; migration must avoid silent behavior changes and must expose partial or legacy mapping where perfect reconstruction is impossible.
- Profile history contains secrets, raw provider payloads, sensitive user text, cross-tenant identifiers, or unsafe overlay content; inspection must expose only safe summaries.
- A rollback targets a version whose provider preference, safety default, or overlay reference is no longer valid; rollback must fail closed or require explicit repair before activation.
- A handoff destination, group thread, or channel-bound conversation starts after the tenant-default active profile changes; new work must use the current tenant-default active profile while source evidence remains unchanged.
- A user requests channel, workspace, integration-account, or capability-specific profile binding during this phase; the system must preserve tenant-default active profile behavior and defer scoped binding to Roadmap 58.
- Automated tests and fixtures must not leak secrets or live connector evidence while proving profile lifecycle behavior.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST treat agent profiles as tenant-owned structured runtime configuration, not memory, learned preference state, or hidden prompt-file truth.
- **FR-002**: Authorized users MUST be able to create, read, update, list, archive or disable, and select one tenant-default active profile.
- **FR-003**: Profile reads, version history, overlay reference inspection, and runtime profile inspection MUST require `profiles.inspect`, and unauthorized attempts MUST be denied without revealing inaccessible profile existence, fields, history, overlay references, or runtime evidence.
- **FR-003a**: Profile creation, update, activation, archive or disable, overlay reference mutation, and rollback MUST require `profiles.manage`, and unauthorized attempts MUST leave profile state unchanged.
- **FR-004**: Each profile MUST include schema-backed display identity fields, tone or persona fields, default provider preferences, safety defaults, and optional explicit overlay references.
- **FR-005**: Profile validation MUST reject unsupported, malformed, over-limit, unsafe, or policy-conflicting values with user-visible safe failure reasons.
- **FR-006**: Profile fields that may contain sensitive information MUST be redacted or summarized before user-facing, support-facing, audit, fixture, or log exposure.
- **FR-007**: The system MUST preserve profile version history for every material profile change.
- **FR-007a**: The system MUST retain all profile versions while the profile exists so rollback and behavior forensics can evaluate prior versions.
- **FR-008**: Profile versions MUST record actor, tenant, timestamp, changed field summary, prior version reference, resulting version reference, and change reason where provided.
- **FR-009**: Profile create, update, activation, archive or disable, validation failure, permission denial, and rollback outcomes MUST emit audit and event records.
- **FR-010**: Profile mutations MUST fail closed and leave profile state unchanged when required audit or version evidence cannot be recorded.
- **FR-011**: Authorized users MUST be able to roll back an active profile to an eligible prior version.
- **FR-011a**: Rollback eligibility MUST be determined at rollback time by current validation and policy rather than assuming every retained version remains activatable.
- **FR-012**: Rollback MUST create a new auditable active version derived from the prior version rather than deleting, rewriting, or reusing historical evidence ambiguously.
- **FR-013**: Rollback MUST fail closed or require explicit repair when the target version contains provider preferences, safety defaults, or overlay references that are no longer valid or allowed.
- **FR-014**: Each tenant MUST have a deterministic tenant-default active profile resolution path for new work, including a safe default behavior when no custom profile exists.
- **FR-014a**: Phase 57 MUST NOT introduce channel-specific, workspace-specific, integration-account-specific, or capability-specific profile binding; those scoped bindings are deferred to Roadmap 58.
- **FR-014b**: Phase 57 MUST NOT hard-delete profiles while runtime evidence may reference them; archive or disable MUST be the supported profile retirement behavior.
- **FR-015**: Active threads, sessions, runs, workflow starts, and handoff destinations MUST record the active profile identity and version used when the work began.
- **FR-016**: Profile edits, activation changes, archive or disable actions, and rollback MUST NOT silently rewrite historical thread, session, run, workflow, or handoff evidence.
- **FR-017**: Authorized users MUST be able to inspect the active profile identity and version associated with a thread, session, run, workflow start, handoff destination, or equivalent runtime evidence view.
- **FR-018**: Profile runtime projection MUST be restart-safe and durable enough for operators to explain behavior after daemon restart or client reconnect.
- **FR-019**: Editable prompt or workspace files MAY be used only as explicit overlay references attached to a profile or compatible configuration surface.
- **FR-020**: Overlay references MUST show stable reference identity, safe display label, scope, validation status, last validation time where known, and safe failure reason where validation fails.
- **FR-021**: Overlay references MUST NOT be treated as hidden primary truth and MUST NOT override structured profile fields without visible profile evidence.
- **FR-022**: Missing, inaccessible, unsafe, oversized, or out-of-scope overlays MUST NOT silently affect new work.
- **FR-023**: Existing local prompt or config behavior SHOULD migrate or bridge into explicit default profile and overlay references where possible without breaking local workflows.
- **FR-024**: Legacy or partially mapped prompt/config behavior MUST be exposed as partial profile evidence rather than silently discarded or misclassified.
- **FR-025**: Profile lifecycle behavior MUST be available through the product surfaces needed by users, operators, and client integrations for profile CRUD, tenant-default active selection, version history, rollback, overlay reference management, and runtime profile inspection.
- **FR-026**: Profile changes MUST be visible through client-facing contracts with backward-compatible behavior for clients that do not yet use profile lifecycle features.
- **FR-027**: Profile configuration MUST NOT introduce memory retrieval, learned preferences, autonomous profile mutation by the agent, skill generation, multi-agent collaboration, semantic knowledge retrieval, or long-term personalization.
- **FR-028**: Documentation and operator-visible evidence MUST make the difference between structured profile configuration, editable overlays, and future memory or knowledge-plane behavior explicit.
- **FR-029**: Automated verification MUST cover profile CRUD, tenant isolation, permission denial, validation failure, version history, rollback, active profile projection on threads, sessions, runs, workflow starts, and handoff destinations, historical evidence preservation, overlay reference handling, restart recovery, redaction, SDK/client contract behavior, and explicit non-use of memory.

### Key Entities *(include if feature involves data)*

- **Agent Profile**: A tenant-owned structured configuration resource that defines display identity, persona or tone, default provider preferences, safety defaults, and explicit overlay references.
- **Profile Version**: An immutable historical snapshot or equivalent evidence record for a material profile state, retained while the profile exists and including actor, time, change summary, prior version, resulting version, and current rollback eligibility.
- **Active Profile Selection**: The tenant-default resolution that determines which profile and version apply to newly started work in phase 57 before Roadmap 58 adds channel, workspace, integration-account, or capability binding.
- **Runtime Profile Projection**: The durable evidence attached to a thread, session, run, workflow start, or handoff destination showing which profile identity and version were active when work began.
- **Overlay Reference**: An explicit reference from a profile to an editable prompt or workspace file, including scope, safe display label, validation state, and failure reason where applicable.
- **Default Provider Preference**: Profile-owned defaults for selecting provider behavior where tenant policy allows, without overriding unavailable, disabled, quota-limited, or disallowed provider constraints.
- **Safety Defaults**: Profile-owned behavior constraints that express default safety posture and must remain compatible with tenant and system policy.
- **Profile Audit Event**: Tenant-scoped evidence for profile creation, update, activation, archive or disable, validation failure, permission denial, rollback, and runtime projection outcomes.
- **Archived Or Disabled Profile**: A retired profile state that prevents future activation or use for new work while preserving profile versions, audit events, and runtime evidence references for authorized inspection.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Adds tenant-scoped profile, profile version, tenant-default active selection, overlay reference, audit event, runtime projection, SDK, and web shell surfaces. Existing local prompt and config workflows must remain compatible by mapping or bridging them into explicit profile and overlay references where possible.
- **Migration / Rollback**: Rollout can begin with a default profile projection and read-only inspection, then enable profile editing, activation, and rollback after version and audit evidence is durable. Migration should create or identify one default profile per eligible tenant and attach compatible legacy prompt/config behavior as explicit overlay references where safe. Operational rollback disables new profile edits and activation changes while preserving already-recorded profile, version, audit, and runtime projection evidence for authorized inspection.
- **Verification Strategy**: Required validation includes profile CRUD, tenant-default active profile selection, version history, rollback, validation failure behavior, audit-write failure behavior, permission denial, tenant isolation, overlay reference validation, legacy prompt/config bridging, active profile linkage on threads, sessions, runs, workflow starts, and handoff destinations, restart recovery, SDK/client contract coverage, web shell profile editor coverage, and redaction.
- **Observability Impact**: Operators must gain profile lifecycle events, active profile runtime projections, version summaries, rollback outcomes, overlay validation status, permission denials, validation failures, audit-write failures, and redaction-limited classifications as product evidence without relying on raw logs or hidden prompt files.
- **Environment & Secrets**: Development and automated verification must default to the repository test environment. Live connectors and production tenants must not be touched by default. Secrets, tokens, raw provider payloads, unsafe overlay content, disallowed message bodies, and cross-tenant identifiers must not be exposed in tests, fixtures, logs, audit output, or profile inspection.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of profile CRUD tests prove users with `profiles.inspect` can inspect profiles and runtime profile evidence, users with `profiles.manage` can create, update, archive or disable, activate one tenant-default profile, and roll back profiles within their tenant, and unauthorized users learn nothing about inaccessible profiles.
- **SC-002**: 100% of profile validation tests reject malformed, unsupported, over-limit, unsafe, policy-conflicting, missing-overlay, inaccessible-overlay, and unavailable-provider cases with safe user-visible reasons.
- **SC-003**: 100% of profile change tests create version history and audit evidence with actor, time, changed field summary, prior version, resulting version, and outcome.
- **SC-004**: 100% of rollback tests restore an eligible prior profile state for future work while preserving immutable history and creating a new rollback audit event.
- **SC-005**: 100% of rollback-invalid-target tests fail closed or require explicit repair before activation when a prior version references now-invalid provider preferences, safety defaults, or overlays.
- **SC-005a**: 100% of version retention tests prove every profile version remains inspectable while the profile exists and rollback eligibility reflects current validation and policy.
- **SC-005b**: 100% of profile retirement tests prove archived or disabled profiles cannot be selected for new work and are not hard-deleted while historical runtime evidence references them.
- **SC-006**: 100% of thread, session, run, workflow-start, and handoff-destination tests record the tenant-default active profile identity and version used when work begins.
- **SC-006a**: 100% of channel, workspace, integration-account, and capability scoped-binding attempts in this phase either preserve tenant-default active profile behavior or report that scoped binding is deferred to Roadmap 58.
- **SC-007**: 100% of historical evidence tests prove profile edits, activation changes, archive or disable actions, and rollbacks do not rewrite previous run, thread, session, workflow, or handoff profile evidence.
- **SC-008**: 100% of restart recovery tests preserve profile records, active profile selection, version history, audit events, overlay validation state, and runtime profile projections.
- **SC-009**: Authorized operators can identify which profile version influenced a representative behavior change from product evidence within 5 minutes.
- **SC-010**: 100% of compatible legacy prompt/config fixtures either map to explicit default profile or overlay evidence or report partial mapping status without silently changing active behavior.
- **SC-011**: Redaction validation finds zero exposed secrets, tokens, raw provider payloads, unsafe overlay content, disallowed message bodies, or cross-tenant identifiers in user-facing, support-facing, test, fixture, log, and audit output.
- **SC-012**: 100% of SDK/client and web shell tests cover profile list, detail, create, edit, tenant-default active selection, version history, rollback, overlay references, and runtime profile inspection paths.
- **SC-013**: Verification confirms this phase creates no memory retrieval, learned preferences, agent-generated profile mutation, skill generation, semantic knowledge retrieval, long-term personalization, or autonomous multi-agent collaboration in 100% of covered flows.

## Assumptions

- Roadmap 45 provides tenant activation, default personal tenant behavior, and the hosted identity foundation needed to own profiles by tenant.
- Roadmap 54 provides thread, session, run, source linkage, lifecycle inspection, audit, permission, retention, and restart-safety foundations that profile runtime projection can attach to.
- Roadmap 56 provides group, room, reset, and handoff semantics that must carry tenant-default profile identity without merging or rewriting source and destination evidence.
- Profile management introduces dedicated `profiles.inspect` and `profiles.manage` tenant permissions rather than reusing credential inspection, connector management, or tenant management permissions.
- A tenant can have a safe default profile before a user creates a custom one.
- Channel, workspace, integration-account, and capability-specific binding begins in Roadmap 58 and is not part of phase 57.
- Provider preferences are defaults, not hard guarantees; tenant policy, provider availability, quotas, and safety rules remain authoritative.
- Overlay references preserve compatibility with editable prompt or workspace files but do not become primary truth.
- Automated verification uses fake or test-environment provider, connector, thread, and overlay evidence by default. Live connector validation is optional unless a later release-readiness gate explicitly requires approved safe accounts.
