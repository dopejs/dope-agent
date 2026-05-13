# Research: Agent Profile And Persona Configuration

## Decision: Add a dedicated profile domain package

Use a new `daemon/internal/profiles` package for profile resources, validation, state
transitions, rollback eligibility, runtime projection summaries, and redaction helpers.
Persistence remains in `daemon/internal/store`, and tenant-safe wrappers live in
`daemon/internal/store/tenancy`.

**Rationale**: Profile/persona configuration is product state with its own lifecycle and
audit rules. Keeping domain logic out of API handlers and provider manager code preserves
current module boundaries and makes rollback, validation, and redaction testable without
starting the daemon.

**Alternatives considered**:

- Put profile logic directly in `daemon/internal/api`: rejected because validation and
  rollback rules would be hard to reuse from chat/runtime startup paths.
- Extend `daemon/internal/providers`: rejected because provider defaults are one field of
  a profile, not the owner of persona, overlays, safety defaults, or audit history.
- Store profile JSON only with no domain package: rejected because it hides validation
  and state transition invariants.

## Decision: Introduce `profiles.inspect` and `profiles.manage`

Add dedicated tenant permissions for profile inspection and profile mutation. Owners and
admins should receive both by default. Read/runtime inspection uses `profiles.inspect`;
create/update/activation/archive/disable/overlay mutation/rollback uses
`profiles.manage`.

**Rationale**: Profiles influence runtime behavior but are not connector lifecycle,
credentials, or tenant membership. Dedicated permissions avoid granting profile mutation
to users who can manage connectors or inspect credentials for unrelated reasons.

**Alternatives considered**:

- Reuse `credentials.inspect` and `connectors.manage`: rejected because it couples persona
  management to credential and connector operations.
- Use `tenant.manage`: rejected because profile edits should not require broad tenant
  administration.
- Role-only gating: rejected because existing authorization is permission-based and SDK
  contracts expose permission evaluations.

## Decision: Store one tenant-default active profile in Phase 57

Support exactly one tenant-default active profile selection in Phase 57. Channel,
workspace, integration-account, and capability-specific profile binding is deferred to
Roadmap 58.

**Rationale**: Roadmap 58 is the authoritative binding slice. Phase 57 must provide the
profile resource model, versioning, overlay references, and runtime projection foundation
without pre-implementing scoped binding semantics.

**Alternatives considered**:

- Per-channel selection in Phase 57: rejected because it overlaps Roadmap 58 and expands
  connector-specific policy before the binding model exists.
- Multiple scopes in Phase 57: rejected because it would require workspace/capability
  policy decisions outside this roadmap.
- No active selection: rejected because runtime profile projection and default behavior
  require deterministic profile resolution for new work.

## Decision: Retain all profile versions while the profile exists

Every material profile change creates a retained version. Rollback creates a new active
version derived from a retained version after current validation and policy checks pass.

**Rationale**: Durable profile history is necessary for behavior forensics and rollback.
Validity is time-dependent because providers, safety policy, and overlay references can
change, so rollback eligibility must be recomputed instead of assumed.

**Alternatives considered**:

- Keep current version only: rejected because rollback and behavior explanation would be
  impossible after edits.
- Retain versions for 90 days: rejected because profiles are configuration truth, not
  short-lived thread inspection evidence.
- Retain only the latest N versions: rejected because it creates arbitrary loss of
  operator evidence and rollback source material.

## Decision: Archive/disable only; no hard delete in Phase 57

Retire profiles by archive or disable state. Do not hard-delete profiles while runtime
evidence may reference them.

**Rationale**: Threads, runs, workflows, and handoff destinations must keep stable profile
identity and version evidence. Hard deletion would either break historical references or
require tombstone semantics that belong in a broader retention/deletion design.

**Alternatives considered**:

- Hard delete profiles with no runtime evidence: rejected because it adds branching
  lifecycle behavior that does not close this roadmap.
- Hard delete with tombstones: rejected because archive/disable already satisfies product
  retirement needs with lower migration risk.
- Hard delete including versions: rejected because it conflicts with auditability and
  historical runtime evidence.

## Decision: Use additive SQLite schema v53

Add new tables for profile resources, versions, active selections, overlay references,
and runtime projections. Do not mutate existing provider preference, thread, session, run, workflow, handoff, or
workflow tables destructively. Add profile projection references to existing response
documents and schemas as additive optional fields.

**Rationale**: Additive tables preserve backward compatibility, simplify rollback, and
allow default profile seeding without rewriting existing runtime history. Optional API
fields let older clients continue to parse existing resources.

**Alternatives considered**:

- Store profiles only inside tenant records: rejected because version history, overlays,
  active selection, and runtime projections need independent lifecycle and indexes.
- Add many profile columns to threads/sessions/runs/workflows/handoffs: rejected because runtime projection
  evidence is shared and should remain extensible.
- Rewrite provider preferences as profile records: rejected because provider preferences
  are still used by existing provider surfaces and should be bridged, not destroyed.

## Decision: Runtime profile projection is snapshot evidence

When new work starts, record profile ID, version ID, display label, status, source
selection reason, redaction status, and safe summary on the thread/session/run/workflow/handoff
runtime evidence. Historical work must not be rewritten when a profile changes.

**Rationale**: Operators need to know which profile influenced behavior. A snapshot-style
projection avoids ambiguity after edits or rollback and matches existing thread runtime
projection patterns.

**Alternatives considered**:

- Store only active profile ID and join to current version at inspection time: rejected
  because it rewrites history semantically after edits.
- Copy full profile content into every run: rejected because it increases secret and
  sensitive text exposure risk.
- Rely on logs: rejected because logs are not product evidence and may not be retained.

## Decision: Overlay references are validated metadata, not primary truth

Profiles may reference editable prompt or workspace files as overlay references with
stable reference identity, scope, safe label, validation state, and failure reason. Missing
or unsafe overlays must not silently affect new work.

**Rationale**: This preserves local file workflows while making hidden file-shaped truth
inspectable and auditable. The profile remains the primary structured configuration.

**Alternatives considered**:

- Import overlay file contents into profile versions: rejected because it risks secret
  exposure and makes external files look like structured truth.
- Keep prompt files as implicit runtime inputs: rejected because it preserves the audit
  gap this roadmap is meant to close.
- Reject all overlays: rejected because it would break existing local workflows.

## Decision: Contract shape is additive and SDK-first

Expose profile list/detail/create/update/activate/history/rollback/archive routes, schema
resources, SDK types/methods, and web/TUI views. Existing clients that do not use profile
routes continue to work. Existing runtime evidence responses receive optional profile projection
fields.

**Rationale**: Contract-first planning keeps API/schema/event/SDK surfaces aligned with
constitution requirements. Additive optional fields reduce client migration cost.

**Alternatives considered**:

- Hide profile lifecycle behind config route: rejected because profile lifecycle needs
  independent permissions, history, rollback, and audit evidence.
- Expose profile behavior only in web UI: rejected because SDK and tests need stable
  contracts.
- Make runtime projection internal only: rejected because operators need product-visible
  behavior evidence.
