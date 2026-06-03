# Phase 0 Research: Workspace And Capability Binding

All Technical Context unknowns were resolvable from the codebase and the four clarifications
recorded in `spec.md` (Session 2026-06-03). No `NEEDS CLARIFICATION` remain. Findings below
are grounded in current code (verified 2026-06-03).

## R1. Workspace representation: persisted record vs projection

- **Decision**: Workspace is a **persisted tenant-owned record** in a new `workspaces` table.
  The default personal workspace is one such record.
- **Rationale**: Clarification chose "Persisted resource". FR-027 requires restart-survival,
  FR-022 requires repair status, and audit/runtime evidence reference a stable workspace
  identity — all need a durable row. The Roadmap 57 profile domain already establishes the
  record + status + redaction + `document_json` pattern (`agent_profiles` table, migration 53);
  mirroring it keeps the store consistent.
- **Alternatives considered**: Pure projection (rejected — fragile repair/audit anchoring,
  no stable id for runtime evidence); hybrid projection-default/persisted-user (rejected —
  two identity code paths and dual semantics for no benefit).

## R2. Invalid-binding runtime behavior: fail closed

- **Decision**: When binding resolution selects an archived/disabled/removed/unavailable
  profile or workspace, **fail closed** — block new work with a safe repair-required outcome
  and evidence; never silently substitute or fall through to a lower-precedence binding
  (FR-031).
- **Rationale**: Clarification chose "Fail closed". The spec edge case requires invalid
  bindings "cannot silently affect new work", and FR-022 already mandates surfaced repair
  status. Fail-closed keeps audit unambiguous and prevents an invalid binding from quietly
  degrading to default behavior an operator did not intend.
- **Alternatives considered**: Fall back to tenant default (rejected — silent substitution
  hides misconfiguration, contradicts edge case); mixed fail/fallback by resource (rejected —
  inconsistent mental model, harder to test and explain).
- **Implementation note**: Resolution returns a typed `RepairRequired` outcome distinct from
  "no binding" (which legitimately resolves to tenant default + default workspace). Work-start
  in `chat/service.go` must distinguish the two and only fail closed for the invalid case.

## R3. Capability visibility editable scopes: profile + workspace

- **Decision**: Authorized users edit capability visibility (`visible|hidden|disabled|
  default_enabled`) at **profile and workspace scope only**. Tenant and connector policy are
  enforced as higher-level limits where the strictest wins (FR-017), but are not user-edited
  capability-visibility scopes in this phase (FR-018).
- **Rationale**: Clarification chose "Profile + workspace". User Story 2 only references
  profile/workspace; channel/account scopes add surface and a larger test matrix without a
  story driving them. Strictest-wins still honors tenant/connector limits at resolution time.
- **Alternatives considered**: All binding scopes editable (rejected — unjustified surface for
  this slice); profile-only (rejected — User Story 2 explicitly needs workspace-scoped
  disablement).

## R4. Default-workspace provisioning: lazy on first access

- **Decision**: Provision the default personal workspace **lazily and idempotently** on first
  binding/resolution access via `EnsureDefaultWorkspace(tenantID)`. No bulk migration over all
  tenants at rollout (FR-025).
- **Rationale**: Clarification chose "Lazy on first access". Aligns with the spec's
  read-only-default-first rollout and avoids a risky migration touching every tenant. Mirrors
  the existing `EnsureDefaultAgentProfile()` lazy-seed pattern in `profile_projection.go`.
- **Alternatives considered**: Eager migration for all tenants (rejected — heavier/riskier
  rollout step, touches inactive tenants); eager-active/lazy-rest (rejected — added complexity
  without need).
- **Concurrency**: Enforce a unique constraint on `(tenant_id) WHERE is_default = 1`; concurrent
  first-access provisioning converges on one record (insert-or-get).

## R5. Schema migration version and store pattern

- **Decision**: Add **migration v54** named `r58_workspace_capability_binding`; bump
  `CurrentSchemaVersion` from 53 → 54 in `daemon/internal/store/store.go`.
- **Rationale**: Verified current `CurrentSchemaVersion = 53` (store.go:40); migration 53 is
  `r57_agent_profile_persona`. Phase 58 is the next additive migration. Migrations register in
  the `schemaMigrations` slice; each batch is forward-only and additive.
- **Alternatives considered**: Reusing/extending profile tables (rejected — distinct domain,
  see plan Complexity Tracking).

## R6. Work-start resolution hook

- **Decision**: Resolve binding selection in `chat/service.go` alongside `resolveActiveProfile`
  (`ActiveAgentProfileSelection`, service.go:283) and record binding runtime evidence in the
  same pass as `recordActiveProfileProjection` (service.go:312, called from :128/:198).
- **Rationale**: This is the single existing tenant-scoped resolution point for new work; both
  query (lines 102 and 172) call sites already resolve+record the profile. Adding binding
  resolution here keeps the resolution to one bounded extra pass (Performance Goal) and ensures
  profile + workspace + capability summary are recorded atomically per run.
- **Implementation note**: Resolve workspace/capability via the resolved profile + originating
  channel/account identity; on `RepairRequired` (R2) fail the work-start with safe evidence.

## R7. Reusing the planted Roadmap 58 deferral hooks

- **Decision**: Flip the existing deferral markers to applied-binding behavior:
  `profiles.RuntimeProjection.DeferredBindingClassification =
  "roadmap_58_deferred_binding_unapplied"` (profiles/projection.go:38) becomes an applied
  classification when an explicit binding influenced the run; `profiles.ErrScopedBindingDeferred`
  (policy.go:13) scoped-overlay path is superseded by real binding scopes where applicable.
- **Rationale**: The codebase deliberately reserved these as Roadmap 58 integration points
  (verified via grep). Honoring them avoids a parallel/competing mechanism and satisfies
  roadmap-closure (no leftover "deferred" state once 58 lands). Contract tests reference the
  classification string, so the new applied value must be covered by updated contract fixtures.
- **Alternatives considered**: Leaving the deferred classification untouched and adding a
  separate flag (rejected — leaves a misleading "deferred" signal after the roadmap closes).
- **Compatibility note**: Runs with no explicit binding keep recording the default/legacy
  classification so historical and legacy-default evidence stay labeled as default, not
  user-configured (FR-026).

## R8. Connector channel identity and integration-account identity

- **Decision**: Bind channels by tenant-owned channel identity from
  `daemon/internal/connectors` (Roadmap 48 conformance: tenant_id + connector_id + durable
  channel fields). Bind account defaults by integration-account identity from
  `daemon/internal/integrations` (`AccountBinding` / `Resource`, Roadmap 37).
- **Rationale**: Both identities already exist and are tenant-scoped; bindings store opaque
  scope_ref values validated against these sources at create time and at resolution
  (`FR-009` validation, `FR-022` repair when the referenced channel/account is gone).
- **Alternatives considered**: Redefining channel/account identity inside bindings (rejected —
  Assumptions forbid redefining connector routing/dedupe or account isolation).

## R9. Capability enumeration and runtime gating

- **Decision**: Treat capabilities as the existing product-visible action/integration surfaces
  enumerated via `daemon/internal/capabilities` + `daemon/internal/skills`. Add a capability
  **visibility resolver** (`bindings/visibility.go`) that, before a capability is offered or
  executed, computes effective visibility = strictest of tenant/connector limits + profile +
  workspace policy. Enforce at the runtime gate that already mediates action availability
  (`daemon/internal/policy`).
- **Rationale**: No persistent capability-visibility table exists yet; policy currently gates
  via approval decisions. Adding a tenant-owned `capability_visibility_policies` table plus a
  resolver gives policy-backed product truth (FR-015) and a denial gate that blocks hidden/
  disabled capabilities even on direct/replay/stale requests (FR-016) without inventing a
  marketplace.
- **Alternatives considered**: Enforcing only at the presentation layer (rejected — FR-016
  requires execution denial, not just hiding); per-capability ad hoc flags (rejected — not
  tenant-owned/auditable).

## R10. Audit-write fail-closed and events

- **Decision**: Reuse the `identity` audit `Auditor.Require()` (`ErrAuditWriteFailed`) so
  binding mutations fail closed and leave state unchanged when audit/event evidence cannot be
  recorded (FR-011). Add binding event constructors in
  `events/workspace_capability_bindings.go` mirroring `events/agent_profiles.go`.
- **Rationale**: The profile domain already establishes the require-audit-before-commit
  pattern and event constructor shape; reuse keeps observability consistent and satisfies
  FR-010/FR-011.
- **Alternatives considered**: Best-effort audit (rejected — violates FR-011 fail-closed).

## R11. Redaction

- **Decision**: All binding inspection, runtime evidence, events, audit, logs, tests, and
  fixtures pass through `bindings/redaction.go` safe-summary helpers (mirror
  `profiles/redaction.go`), exposing safe labels/scope/status only — never secrets, tokens,
  raw provider payloads, unsafe capability inputs, message bodies, or cross-tenant identifiers
  (FR-028, SC-014).
- **Rationale**: Matches the established profile redaction discipline and the spec's Environment
  & Secrets section.

## Testing strategy summary

- Go unit/domain: precedence resolver, visibility strictest-wins, fail-closed invalid
  resolution, validation, repair status, redaction (`daemon/internal/bindings`).
- Store: workspace + binding CRUD, lazy default workspace + concurrency, restart recovery
  (`daemon/internal/store`).
- Tenancy: `BindingAccessScope` inspect/manage isolation (`store/tenancy`).
- API: route CRUD, permission denial without leaking existence, validation responses,
  client-compatible defaults (`daemon/internal/api`).
- Chat: binding resolution + runtime evidence on new work, fail-closed work-start
  (`daemon/internal/chat`).
- Contracts: schema/fixture validation incl. applied binding classification
  (`make daemon-contract-test`).
- Clients: SDK/Web/TUI binding surfaces + backward-compatible older-client behavior
  (`pnpm test:clients`).
