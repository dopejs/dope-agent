# Agent Profile And Persona Configuration

Roadmap 57 adds tenant-owned agent profiles as explicit configuration. A profile stores
safe display identity, persona summaries, provider defaults, safety defaults, overlay
references, retained versions, active tenant-default selection, and runtime projection
evidence. It is not memory and does not learn preferences from conversation text.

## API Surface

- `GET /v1/profiles` requires `profiles.inspect`.
- `POST /v1/profiles` requires `profiles.manage`.
- `GET /v1/profiles/{profileId}` requires `profiles.inspect`.
- `PATCH /v1/profiles/{profileId}` requires `profiles.manage`.
- `POST /v1/profiles/{profileId}/activate` requires `profiles.manage`.
- `GET /v1/profiles/{profileId}/versions` requires `profiles.inspect`.
- `POST /v1/profiles/{profileId}/rollback` requires `profiles.manage`.
- `POST /v1/profiles/{profileId}/archive` requires `profiles.manage`.
- `POST /v1/profiles/{profileId}/disable` requires `profiles.manage`.

Archive and disable are non-destructive. Version history remains inspectable, and retiring
the current tenant default records a safe fallback selection.

## Persistence

Schema v53 creates `agent_profiles`, `agent_profile_versions`,
`agent_profile_active_selections`, `agent_profile_overlay_references`,
`agent_profile_runtime_projections`, and `agent_profile_audit_events`.

Every table is tenant-owned. Store access must always carry an explicit tenant context
or a system actor for safe default seeding. Runtime projections snapshot the active
profile and version at work start; later profile edits must not rewrite historical
runtime evidence.

## Operator Verification

An operator investigating behavior should be able to identify the profile influence in
under five minutes:

1. Inspect the run or thread evidence for `activeProfileProjection` or profile runtime
   projection rows.
2. Use the recorded `profileId` and `profileVersionId` with `/v1/profiles/{profileId}`.
3. Compare the retained version snapshot with the active version and audit events.
4. Check overlay validation states and redaction status before retrying work.

Rollback is operationally reversible by disabling profile mutation routes/UI while
leaving read-only profile and runtime evidence available.
