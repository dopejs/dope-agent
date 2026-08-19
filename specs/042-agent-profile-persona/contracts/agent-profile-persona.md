# Contract: Agent Profile And Persona Configuration

## Scope

This contract defines the Phase 57 product surfaces for structured agent profiles,
persona fields, tenant-default active profile selection, profile versioning, rollback,
overlay references, and runtime profile projection.

Required surfaces:

- Tenant-scoped profile CRUD, archive/disable, active selection, history, and rollback
- Dedicated `profiles.inspect` and `profiles.manage` tenant permissions
- Structured profile fields for display identity, persona/tone, provider defaults, and
  safety defaults
- Explicit overlay references for prompt/workspace/config files
- Runtime profile projection on threads, sessions, runs, workflows, and handoff
  destinations
- Event/audit evidence for profile lifecycle, validation, denials, rollback, and runtime
  projection
- JSON schemas, fixtures, TypeScript SDK types/methods, web profile editor/history, and
  TUI/operator inspection

Out of scope:

- Memory retrieval or learned preferences
- Agent-generated profile mutation
- Channel, workspace, integration-account, or capability-specific binding
- Skill generation
- Multi-agent autonomous collaboration
- Semantic knowledge retrieval or long-term personalization
- Hard profile deletion while runtime evidence may reference a profile

## Permissions

| Action | Required Permission |
|--------|---------------------|
| List profiles | `profiles.inspect` |
| View profile detail, versions, overlay references, and safe audit summaries | `profiles.inspect` |
| View runtime profile projection on thread/session/run/workflow/handoff evidence | `profiles.inspect` plus access to the containing runtime resource |
| Create profile | `profiles.manage` |
| Update profile fields or overlay references | `profiles.manage` |
| Activate tenant-default profile | `profiles.manage` |
| Roll back to an eligible retained version | `profiles.manage` |
| Archive or disable profile | `profiles.manage` |

Unauthorized requests MUST return a stable denial without exposing inaccessible profile
existence, fields, version history, overlay references, runtime projection, or audit
evidence.

## Profile Resource Contract

`GET /v1/profiles`

Response:

```json
{
  "tenantId": "ten_personal",
  "page": {
    "limit": 20,
    "nextCursor": "",
    "order": "updated_at_desc"
  },
  "items": [
    {
      "profileId": "prof_default",
      "tenantId": "ten_personal",
      "displayName": "Default Agent",
      "status": "active",
      "activeVersionId": "profv_default_3",
      "tenantDefault": true,
      "displayIdentity": {
        "name": "Kura",
        "safeSummary": "Default personal assistant profile"
      },
      "persona": {
        "tone": "direct",
        "safeSummary": "Concise, production-oriented behavior"
      },
      "defaultProviderPreference": {
        "providerId": "codex_managed",
        "model": "gpt-5.4",
        "validationState": "valid"
      },
      "safetyDefaults": {
        "approvalPosture": "ask_for_risky_changes",
        "validationState": "valid"
      },
      "overlayReferenceCount": 1,
      "redactionStatus": "redacted",
      "createdAt": "2026-05-12T10:00:00Z",
      "updatedAt": "2026-05-12T10:10:00Z"
    }
  ]
}
```

`GET /v1/profiles/{profileId}`

Response includes the same profile resource plus recent versions, overlay references,
safe audit summaries, and activation metadata:

```json
{
  "profile": {
    "profileId": "prof_default",
    "tenantId": "ten_personal",
    "displayName": "Default Agent",
    "status": "active",
    "activeVersionId": "profv_default_3",
    "tenantDefault": true,
    "redactionStatus": "redacted",
    "createdAt": "2026-05-12T10:00:00Z",
    "updatedAt": "2026-05-12T10:10:00Z"
  },
  "versions": [],
  "overlayReferences": [],
  "auditEvents": []
}
```

Rules:

- Profile IDs are tenant-scoped and must not reveal cross-tenant existence.
- List ordering must be deterministic and paginated.
- `tenantDefault` is true only for the current tenant-default active profile selection.
- Profile content exposed to users/operators must be safe summaries or redacted structured
  fields.

## Create And Update Contract

`POST /v1/profiles`

Request:

```json
{
  "displayName": "Support Agent",
  "displayIdentity": {
    "name": "Support Agent",
    "description": "Helps triage support work"
  },
  "persona": {
    "tone": "direct",
    "instructions": "Be concise and precise."
  },
  "defaultProviderPreference": {
    "providerId": "codex_managed",
    "model": "gpt-5.4"
  },
  "safetyDefaults": {
    "approvalPosture": "ask_for_risky_changes"
  },
  "overlayReferences": [
    {
      "referenceKind": "prompt_file",
      "referenceUri": "AGENTS.md",
      "scope": "profile"
    }
  ],
  "activate": false,
  "reasonCode": "user_created_profile"
}
```

Response:

```json
{
  "profile": { "profileId": "prof_support", "status": "draft" },
  "version": {
    "profileVersionId": "profv_support_1",
    "versionNumber": 1,
    "changeKind": "created",
    "rollbackEligibility": "eligible"
  },
  "auditEventId": "audit_profile_created_123"
}
```

`PATCH /v1/profiles/{profileId}`

Request accepts the same editable fields. Omitted fields are unchanged. Every material
change creates a new profile version.

Rules:

- Invalid provider defaults, safety defaults, persona fields, or overlay references return
  `400` with a stable safe reason code.
- Permission denials return `403` without profile existence disclosure.
- Required audit/version write failure returns `500` and leaves profile state unchanged.
- Updates to an active profile create a new active version for future work only; historical
  runtime profile projections are not rewritten.

## Active Selection Contract

`POST /v1/profiles/{profileId}/activate`

Request:

```json
{
  "profileVersionId": "profv_support_2",
  "reasonCode": "user_selected_default"
}
```

Response:

```json
{
  "selectionId": "sel_tenant_default_123",
  "tenantId": "ten_personal",
  "profileId": "prof_support",
  "profileVersionId": "profv_support_2",
  "selectionScope": "tenant_default",
  "selectionReason": "user_activated",
  "selectedAt": "2026-05-12T10:15:00Z",
  "auditEventId": "audit_profile_activated_123"
}
```

Rules:

- Phase 57 supports only `tenant_default` active selection.
- Activation fails if the profile is archived, disabled, invalid, cross-tenant, or the
  requested version is not eligible under current policy.
- Channel, workspace, integration-account, and capability-specific selection requests
  must be rejected or reported as deferred to Roadmap 58.

## Version And Rollback Contract

`GET /v1/profiles/{profileId}/versions`

Response:

```json
{
  "profileId": "prof_support",
  "items": [
    {
      "profileVersionId": "profv_support_2",
      "versionNumber": 2,
      "changeKind": "updated",
      "changeSummary": "Updated tone and provider default",
      "rollbackEligibility": "eligible",
      "actorPrincipalId": "prn_user",
      "createdAt": "2026-05-12T10:12:00Z",
      "redactionStatus": "redacted"
    }
  ]
}
```

`POST /v1/profiles/{profileId}/rollback`

Request:

```json
{
  "sourceProfileVersionId": "profv_support_1",
  "reasonCode": "operator_reverted_persona"
}
```

Response:

```json
{
  "profile": { "profileId": "prof_support", "activeVersionId": "profv_support_3" },
  "sourceProfileVersionId": "profv_support_1",
  "resultingProfileVersionId": "profv_support_3",
  "auditEventId": "audit_profile_rollback_123"
}
```

Rules:

- All profile versions are retained while the profile exists.
- Rollback creates a new version; it does not reactivate the historical row in place.
- Rollback eligibility is checked against current provider availability, safety policy,
  overlay validation, redaction confidence, tenant policy, and profile status.
- Ineligible rollback fails closed and leaves active selection unchanged.

## Archive And Disable Contract

`POST /v1/profiles/{profileId}/archive`

`POST /v1/profiles/{profileId}/disable`

Request:

```json
{
  "reasonCode": "operator_retired_profile"
}
```

Rules:

- Archived or disabled profiles cannot be selected for new work.
- Existing runtime projections remain inspectable where authorized.
- If the archived/disabled profile was tenant-default active, the system must require a
  replacement active profile or fall back to a safe default profile with audit evidence.
- Hard delete is not a Phase 57 API.

## Overlay Reference Contract

Overlay references are embedded in profile detail and versions:

```json
{
  "overlayReferenceId": "ovr_agents_md",
  "profileId": "prof_default",
  "profileVersionId": "profv_default_3",
  "referenceKind": "prompt_file",
  "scope": "profile",
  "referenceUri": "AGENTS.md",
  "safeDisplayLabel": "AGENTS.md",
  "validationState": "valid",
  "failureReasonCode": "",
  "lastValidatedAt": "2026-05-12T10:10:00Z",
  "redactionStatus": "redacted"
}
```

Allowed `validationState` values:

- `valid`
- `partial`
- `missing`
- `permission_denied`
- `out_of_scope`
- `too_large`
- `unsafe_content`
- `redaction_failed`

Rules:

- Invalid overlays must not silently influence new work.
- Overlay content is not exposed in raw form through profile APIs.
- Legacy prompt/config bridging must produce explicit overlay references or partial
  mapping evidence.

## Runtime Projection Contract

Thread detail, run resources, workflow resources where exposed, and handoff destination
evidence gain optional active profile projection:

```json
{
  "activeProfile": {
    "runtimeProfileProjectionId": "rpp_run_123",
    "profileId": "prof_support",
    "profileVersionId": "profv_support_2",
    "selectionId": "sel_tenant_default_123",
    "selectionScope": "tenant_default",
    "selectionReason": "user_activated",
    "safeDisplayName": "Support Agent",
    "safeSummary": "Support-oriented direct persona",
    "occurredAt": "2026-05-12T10:16:00Z",
    "redactionStatus": "redacted"
  }
}
```

Rules:

- Projection is recorded when new work begins.
- Later profile edits, rollback, archive, or disable must not rewrite historical
  projections.
- If no custom profile exists, projection records the safe default profile or fallback
  selection with audit evidence.
- Projection must survive daemon restart.

## Event Contract

Profile lifecycle event payloads use stable classifications:

```json
{
  "tenantId": "ten_personal",
  "profileId": "prof_support",
  "profileVersionId": "profv_support_2",
  "eventKind": "profile.updated",
  "outcome": "succeeded",
  "permissionGate": "profiles.manage",
  "reasonCode": "user_updated_profile",
  "auditEventId": "audit_profile_updated_123",
  "redactionStatus": "redacted"
}
```

Required event schemas:

- `agent-profile-lifecycle.event.schema.json`
- `agent-profile-version-created.event.schema.json`
- `agent-profile-runtime-projected.event.schema.json`

Rules:

- Event and audit output must avoid secrets, raw provider payloads, unsafe overlay
  content, disallowed message bodies, and cross-tenant identifiers.
- Reason code additions require schema, SDK, fixture, and contract updates.
- Audit-write failure must fail closed for profile mutations.

## SDK, Web, And TUI Contract

SDK must expose:

- `listProfiles`
- `getProfile`
- `createProfile`
- `updateProfile`
- `activateProfile`
- `listProfileVersions`
- `rollbackProfile`
- `archiveProfile`
- `disableProfile`
- active profile projection types on thread/session/run/workflow/handoff resources

Web shell must expose:

- Profile list/detail
- Profile editor
- Tenant-default active profile selection
- Version history and rollback
- Overlay reference validation status
- Runtime profile evidence on relevant thread/session/run/workflow/handoff views

TUI/operator shell must expose at least:

- Profile list/detail/history inspection
- Active profile identity on thread/session/run/workflow/handoff inspection
- Safe failure and permission denial summaries

## Compatibility And Migration Contract

- All route additions are additive.
- Existing provider default APIs remain compatible; profile provider preference bridges to
  provider defaults without deleting existing provider preference records.
- Existing prompt/config behavior maps to default profile and overlay references where
  safe; otherwise partial mapping evidence is shown.
- Existing clients can ignore optional profile projection fields.
- Schema migration v53 must be rollback-safe by disabling new routes/actions while
  retaining metadata evidence.

## Verification Matrix

| Surface | Required Coverage |
|---------|-------------------|
| Permissions | `profiles.inspect` and `profiles.manage` grants, denials, role-derived permission tests |
| CRUD | create, list, detail, update, archive, disable, tenant isolation |
| Validation | invalid persona, provider unavailable, policy conflict, unsafe overlay, oversized overlay |
| Versioning | every material change creates retained immutable version evidence |
| Rollback | eligible rollback succeeds; invalid rollback fails closed; history remains immutable |
| Active selection | one tenant-default active profile; scoped binding deferred to Roadmap 58 |
| Runtime projection | thread, session, run, workflow, and handoff destination projection created and not rewritten |
| Restart recovery | profiles, active selection, overlay validation, and runtime projections survive restart |
| Redaction | no secrets/raw provider payload/unsafe overlay/cross-tenant identifiers exposed |
| SDK/Web/TUI | client methods, editor/history views, runtime evidence display |
| Non-memory | no learned preference, memory retrieval, semantic recall, or agent self-mutation |
