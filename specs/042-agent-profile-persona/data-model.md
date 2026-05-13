# Data Model: Agent Profile And Persona Configuration

## Agent Profile

Tenant-owned structured runtime configuration for agent identity, persona, provider
defaults, safety defaults, and overlay references.

### Fields

- `profileId`: stable unique profile identifier.
- `tenantId`: owning tenant.
- `displayName`: user-facing profile label, unique enough for tenant display but not a
  global identity boundary.
- `displayIdentity`: structured display identity such as agent name, short description,
  avatar/color hint where supported, and safe public label.
- `persona`: structured tone/persona fields, including tone label, behavior notes, and
  bounded instruction fields accepted by validation.
- `defaultProviderPreference`: default provider/model preference and optional reasoning
  level or mode where tenant policy and provider availability allow it.
- `safetyDefaults`: explicit default safety posture, such as approval posture, risk
  tolerance labels, and disallowed behavior notes that remain subordinate to system and
  tenant policy.
- `status`: `draft`, `active`, `archived`, or `disabled`.
- `activeVersionId`: current active version when status permits activation.
- `createdAt`, `updatedAt`, `archivedAt`, `disabledAt`: lifecycle timestamps.
- `createdByPrincipalId`, `updatedByPrincipalId`: actor evidence.
- `redactionStatus`: `redacted`, `suppressed`, or `redaction_failed`.
- `documentJson`: canonical resource document for compatibility and schema evolution.

### Validation Rules

- `tenantId`, `profileId`, `displayName`, and a valid `activeVersionId` are required for
  active profiles.
- Only one tenant-default active profile selection may point to an activatable profile per
  tenant in Phase 57.
- Profile values must reject unsupported, malformed, over-limit, unsafe, or
  policy-conflicting identity, persona, provider, safety, and overlay fields.
- Provider preferences are defaults only; provider availability, quota, tenant policy,
  and setup gates remain authoritative.
- Archive/disable prevents future activation and new-work use but preserves versions and
  runtime evidence references.
- Hard delete is not supported in Phase 57 while runtime evidence may reference the
  profile.

### Relationships

- Has many `ProfileVersion` records.
- Has many `OverlayReference` records.
- May be selected by one `ActiveProfileSelection` for a tenant.
- May be referenced by many `RuntimeProfileProjection` records.
- Emits many `ProfileAuditEvent` records.

## Profile Version

Immutable evidence for a material profile state.

### Fields

- `profileVersionId`: stable version identifier.
- `profileId`, `tenantId`: owning profile and tenant.
- `versionNumber`: monotonic tenant/profile-local sequence.
- `sourceVersionId`: prior version or rollback source where applicable.
- `changeKind`: `created`, `updated`, `activated`, `rolled_back`, `archived`,
  `disabled`, or `validated`.
- `changeSummary`: safe field-level summary, without unsafe raw persona or overlay text.
- `snapshot`: structured profile snapshot.
- `rollbackEligibility`: `eligible`, `invalid_provider`, `invalid_overlay`,
  `policy_blocked`, `profile_archived`, `profile_disabled`, or `redaction_failed`.
- `actorPrincipalId`: actor responsible for this version.
- `createdAt`: version creation time.
- `auditEventId`: linked audit/event evidence.
- `redactionStatus`: `redacted`, `suppressed`, or `redaction_failed`.
- `documentJson`: canonical version document.

### Validation Rules

- Versions are retained while the profile exists.
- Versions are immutable after creation except for derived safe eligibility projection if
  implementation stores current eligibility separately.
- Rollback creates a new version derived from a retained source version.
- Rollback eligibility must be computed using current validation and policy.

### Relationships

- Belongs to one `AgentProfile`.
- May be the source for a later rollback-created `ProfileVersion`.
- May be referenced by `RuntimeProfileProjection`.

## Active Profile Selection

Tenant-default resolution used by new work in Phase 57.

### Fields

- `selectionId`: stable active selection identifier.
- `tenantId`: owning tenant.
- `profileId`: selected profile.
- `profileVersionId`: selected version.
- `selectionScope`: fixed to `tenant_default` in Phase 57.
- `selectionReason`: `default_seeded`, `user_activated`, `rollback_activated`, or
  `system_fallback`.
- `selectedByPrincipalId`: actor for user-driven selections.
- `selectedAt`: selection time.
- `auditEventId`: linked audit/event evidence.
- `redactionStatus`: `redacted`, `suppressed`, or `redaction_failed`.
- `documentJson`: canonical selection document.

### Validation Rules

- One current tenant-default active selection per tenant.
- Selected profile must be active and not archived or disabled.
- Selected version must belong to the selected profile and pass current validation.
- Channel, workspace, integration-account, and capability-specific scopes are invalid in
  Phase 57 and must be deferred to Roadmap 58.

### Relationships

- Points to one `AgentProfile` and one `ProfileVersion`.
- Feeds `RuntimeProfileProjection` when new work starts.

## Overlay Reference

Explicit reference from a profile to an editable prompt or workspace file.

### Fields

- `overlayReferenceId`: stable overlay reference identifier.
- `profileId`, `profileVersionId`, `tenantId`: owning profile/version/tenant.
- `referenceKind`: `prompt_file`, `workspace_file`, `operator_note`, or `legacy_config`.
- `scope`: `profile` in Phase 57; future scoped binding is Roadmap 58.
- `referenceUri`: stable file or config reference after normalization.
- `safeDisplayLabel`: redacted user-facing label.
- `validationState`: `valid`, `partial`, `missing`, `permission_denied`,
  `out_of_scope`, `too_large`, `unsafe_content`, or `redaction_failed`.
- `failureReasonCode`: stable reason when validation is not valid.
- `lastValidatedAt`: latest validation timestamp.
- `createdAt`, `updatedAt`: lifecycle timestamps.
- `redactionStatus`: `redacted`, `suppressed`, or `redaction_failed`.
- `documentJson`: canonical overlay reference document.

### Validation Rules

- Missing, inaccessible, unsafe, oversized, or out-of-scope overlays must not silently
  affect new work.
- Overlay content is not copied into profile versions unless a later explicit design says
  so; Phase 57 stores references and safe summaries only.
- Overlay references must not override structured profile fields without visible profile
  evidence.

### Relationships

- Belongs to one `AgentProfile`.
- Is captured by or referenced from a `ProfileVersion`.
- May affect `RuntimeProfileProjection` only as explicit overlay reference evidence.

## Runtime Profile Projection

Durable evidence showing which profile identity/version was active when work began.

### Fields

- `runtimeProfileProjectionId`: stable projection identifier.
- `tenantId`: owning tenant.
- `profileId`, `profileVersionId`: selected profile and version.
- `selectionId`: active profile selection used for resolution.
- `resourceKind`: `thread`, `session`, `run`, `workflow`, or `handoff_destination`.
- `resourceId`: target resource identifier.
- `threadId`, `sessionSegmentId`, `runId`, `workflowId`, `handoffLinkId`: optional
  linkage fields where applicable.
- `selectionScope`: `tenant_default`.
- `selectionReason`: reason copied from active selection or fallback.
- `safeDisplayName`: safe profile label used for inspection.
- `safeSummary`: redacted profile summary.
- `occurredAt`: projection time.
- `retentionExpiresAt`: runtime evidence retention boundary where applicable.
- `redactionStatus`: `redacted`, `suppressed`, or `redaction_failed`.
- `documentJson`: canonical projection document.

### Validation Rules

- Projection is created when new work begins and must not be rewritten by later profile
  edits, activation changes, rollback, archive, or disable.
- Projection must be available in authorized thread/session/run/workflow/handoff evidence.
- Projection must survive daemon restart.
- Projection must expose safe summaries only.

### Relationships

- References one `AgentProfile`, one `ProfileVersion`, and one active selection.
- Attaches to threads, sessions, runs, workflows, or handoff destinations.

## Profile Audit Event

Tenant-scoped evidence for profile lifecycle decisions.

### Fields

- `auditEventId`: stable audit/event identifier.
- `tenantId`: owning tenant.
- `profileId`, `profileVersionId`: affected profile/version where applicable.
- `actorPrincipalId`: actor or system principal.
- `eventKind`: `profile.created`, `profile.updated`, `profile.activated`,
  `profile.archived`, `profile.disabled`, `profile.rollback_requested`,
  `profile.rollback_succeeded`, `profile.rollback_denied`,
  `profile.validation_failed`, `profile.permission_denied`,
  `profile.runtime_projected`, or `profile.audit_failed_closed`.
- `outcome`: `succeeded`, `denied`, `failed_closed`, `invalid`, or `projected`.
- `permissionGate`: `profiles.inspect` or `profiles.manage` where applicable.
- `reasonCode`: stable reason classification.
- `safeSummary`: redacted operator-facing summary.
- `occurredAt`: event time.
- `redactionStatus`: `redacted`, `suppressed`, or `redaction_failed`.
- `documentJson`: canonical audit/event document.

### Validation Rules

- Profile mutations fail closed if required audit or version evidence cannot be recorded.
- Permission denials must not reveal inaccessible profile existence, fields, history,
  overlay references, or runtime evidence.
- Audit summaries must not expose secrets, raw provider payloads, unsafe overlay content,
  disallowed message bodies, or cross-tenant identifiers.

### Relationships

- May reference an `AgentProfile`, `ProfileVersion`, `OverlayReference`, or
  `RuntimeProfileProjection`.

## State Transitions

```text
draft -> active       create or activate after validation
active -> active      update creates a new active version
active -> active      rollback creates a new active version from a retained source
active -> archived    archive prevents future activation/use
active -> disabled    disable prevents future activation/use
archived -> active    not supported in Phase 57 unless a later explicit unarchive action is designed
disabled -> active    not supported in Phase 57 unless a later explicit re-enable action is designed
any -> deleted        not supported in Phase 57
```

## Tenant And Runtime Invariants

- Every profile, version, overlay reference, active selection, audit event, and runtime
  projection is tenant-owned.
- Tenant IDs are mandatory for new profile rows.
- Runtime evidence references stable profile/version IDs and must deny cross-tenant reads.
- Existing runs, threads, workflows, handoff links, provider preferences, and local prompt
  files are never destructively rewritten by profile rollout.
- Profile configuration is not memory and must not learn or mutate itself from
  conversation content.
