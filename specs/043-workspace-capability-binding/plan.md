# Implementation Plan: Workspace And Capability Binding

**Branch**: `043-workspace-capability-binding` | **Date**: 2026-06-03 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/043-workspace-capability-binding/spec.md`
**Upstream authority**: `docs/specs/043-workspace-and-capability-binding.md` (Roadmap 58)

## Summary

Close Roadmap 58 by making profile, workspace, channel, integration-account, and capability
visibility **bindings** explicit tenant-owned, auditable, restart-safe product state, and by
projecting the resolved binding selection into runtime evidence on new work. The
implementation adds: a persisted tenant-scoped **workspace** record (the default personal
workspace provisioned lazily on first access), **channel→profile/workspace** bindings,
**integration-account→profile** defaults, **capability visibility** policy at profile and
workspace scope, deterministic precedence resolution (channel → account → tenant default),
**fail-closed** handling of invalid bindings, binding lifecycle audit/events, and a runtime
binding projection recorded alongside the existing profile projection.

The design is additive and reversible. It reuses the mature Roadmap 57 profile domain
patterns and the explicit Roadmap 58 deferral hooks already planted in the codebase
(`profiles.ErrScopedBindingDeferred`, `RuntimeProjection.DeferredBindingClassification =
"roadmap_58_deferred_binding_unapplied"`): those become *applied* binding behavior in this
phase. Binding management uses dedicated `bindings.inspect` / `bindings.manage` permissions.
Phase 58 does **not** introduce memory-backed workspace knowledge, physical workspace storage
migration, a plugin/community marketplace, autonomous capability selection beyond
policy-visible capabilities, or filesystem access from workspace binding alone.

## Technical Context

**Language/Version**: Go 1.24 daemon/control-plane; TypeScript 5.7 SDK + client tests;
React 19 / Vite 7 web surface; Node TUI; JSON Schema contracts under `schemas/`; markdown
runtime/operator docs.
**Primary Dependencies**: Existing tenant identity + permission policy in
`daemon/internal/identity`; SQLite store + tenant guards in `daemon/internal/store` and
`daemon/internal/store/tenancy`; the Roadmap 57 profile domain in `daemon/internal/profiles`
(profile resolution, active selection, runtime projection) consumed at work-start in
`daemon/internal/chat/service.go`; connector/channel identity in
`daemon/internal/connectors` (Roadmap 48 conformance); integration-account identity in
`daemon/internal/integrations` (Roadmap 37); capability/skill enumeration in
`daemon/internal/capabilities`, `daemon/internal/skills`, and runtime gating in
`daemon/internal/policy`; event bus in `daemon/internal/events`; tenant audit in
`daemon/internal/identity` audit/auditor; JSON schemas in `schemas/api` + `schemas/events`;
SDK in `sdk/ts/src`; web shell in `web/src`; TUI in `tui/src`.
**Storage**: Existing SQLite daemon store remains authoritative. Add **additive schema
migration v54** (`r58_workspace_capability_binding`); bump `CurrentSchemaVersion` 53 → 54 in
`daemon/internal/store/store.go`. New tables: `workspaces`, `binding_rules` (channel +
integration-account bindings), `capability_visibility_policies`, and
`binding_runtime_projections`; plus binding audit/event document rows. No destructive rewrite
of existing profile, thread, session, run, workflow, handoff, channel, connector, or
integration evidence.
**Testing**: Targeted Go tests under `daemon/internal/{identity,bindings,store,store/tenancy,
api,chat,events,contracts}`; schema/fixture validation via `make daemon-contract-test`;
SDK/Web/TUI via `pnpm test:clients`; client build via `pnpm build`; full daemon via
`go test ./...` from `daemon/`; `go mod tidy` from `daemon/` after implementation.
**Target Platform**: Local-first and hosted daemon, with API, TS SDK, Web, TUI/operator
shell, runtime projection, and test-environment verification. Default local verification uses
`~/.kura-test` and `127.0.0.1:19192`.
**Project Type**: Multi-surface daemon product feature spanning identity permissions, a new
`bindings` domain package, persistence + tenancy guards, contracts/schemas, work-start
resolution, runtime evidence, capability visibility policy enforcement, events/audit,
migration, redaction, restart recovery, SDK/Web/TUI, and docs.
**Performance Goals**: Authorized operators can determine why a representative capability was
visible/hidden/disabled/denied for a run from product evidence within 5 minutes (SC-012).
Binding resolution adds at most one tenant-scoped resolution pass to starting new work,
reusing the existing single profile-resolution read path; no user-visible latency regression
beyond that bounded resolution.
**Constraints**: Binding reads + runtime/capability-visibility inspection require
`bindings.inspect`; create/update/disable/remove/repair + capability visibility/default
changes require `bindings.manage`. Precedence is fixed: channel binding → integration-account
default → tenant default. Invalid/unavailable profile or workspace selection MUST fail closed
(no silent substitution; FR-031). Workspace = persisted record; default personal workspace is
lazily, idempotently provisioned (FR-002, FR-025). Capability visibility is user-editable at
profile + workspace scope only; tenant/connector remain higher-level limits where strictest
wins (FR-017, FR-018). Workspace binding grants no filesystem/repo/doc/connector/knowledge
access by itself (FR-020) and creates no memory-backed knowledge (FR-021).
**Scale/Scope**: One whole roadmap slice, Phase 58. Surfaces: store + migration, bindings
domain model + precedence resolver + capability-visibility resolver, identity permissions,
API, schema, events/audit, SDK, Web, TUI, runtime binding projection on new work, lazy default
workspace provisioning, repair status, restart recovery, redaction, docs, and tests proving
binding CRUD, tenant isolation, permission denial, validation, precedence, audit-write
fail-closed, capability visibility enforcement, hidden-capability denial, fail-closed invalid
resolution, runtime + historical evidence, restart recovery, client compatibility, and
non-memory/non-filesystem guarantees.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Roadmap closure** — PASS. The plan closes Roadmap 58 as a complete vertical slice:
  workspace record + lazy default provisioning, channel/account binding lifecycle,
  capability visibility policy, deterministic precedence + fail-closed resolution, runtime
  binding projection, audit/events, SDK/Web/TUI surfaces, contracts, migration, restart
  recovery, redaction, docs, and verification. It converts the planted
  `roadmap_58_deferred_binding_unapplied` hook into applied binding behavior rather than
  leaving a partial slice.
- **Production-grade, minimal, reversible change** — PASS. The design is additive to tenant
  identity, the profile domain, and runtime evidence. Smallest reversible change set: new
  `bindings` domain + tables + routes + projection, reusing the profile resolution call site
  in `chat/service.go`. Blast radius is bounded to new work-start resolution and new tables;
  existing profile/thread/run behavior is unchanged when no explicit binding exists (default
  workspace + default visibility). Rollback disables binding mutations and capability
  visibility changes while preserving recorded binding state, audit, and runtime evidence.
- **Contracts and auditability** — PASS. All API/schema/event/SDK/operator behavior is
  captured in [contracts/workspace-capability-binding.md](./contracts/workspace-capability-binding.md).
  Binding mutations, denials, validations, repair, capability-visibility changes, and runtime
  selection produce stable audit + event evidence; audit-write failure fails closed (FR-011).
- **Verification and observability** — PASS. Verification covers permissions, binding CRUD,
  tenant isolation, validation, precedence, audit-write fail-closed, capability visibility
  enforcement, hidden-capability denial, fail-closed invalid resolution, runtime + historical
  evidence, restart recovery, redaction, client contracts, web/TUI, and explicit
  non-memory/non-filesystem checks. Operators gain binding lifecycle events, capability
  visibility decisions, denied-capability evidence, active binding projections, and repair
  status as product evidence (FR-013, FR-014, FR-022).
- **Environment and secrets** — PASS. Local work defaults to `~/.kura-test` with fake/seeded
  tenant/channel/account/capability evidence. Live connectors and production tenants are not
  required (Environment & Secrets in spec; FR-028). Binding inspection, runtime evidence,
  tests, fixtures, logs, and audit redact/summarize secrets, tokens, raw provider payloads,
  unsafe capability inputs, sensitive message bodies, and cross-tenant identifiers.

No violations require justification. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/043-workspace-capability-binding/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── workspace-capability-binding.md
├── checklists/
│   └── requirements.md  # (optional; created by /speckit.checklist)
└── tasks.md             # /speckit.tasks output, NOT created by /speckit.plan
```

### Source Code (repository root)

```text
daemon/
├── internal/
│   ├── identity/
│   │   ├── types.go                      # add bindings.inspect / bindings.manage + AllSensitivePermissions
│   │   ├── permissions.go                # role-derived grants for binding permissions
│   │   └── permissions_test.go
│   ├── bindings/                         # NEW domain package (mirrors profiles/)
│   │   ├── binding.go                    # Workspace, BindingRule, CapabilityVisibilityPolicy,
│   │   │                                 #   EffectiveBindingSelection, RuntimeBindingEvidence, RepairStatus types
│   │   ├── precedence.go                 # deterministic channel → account → tenant resolver
│   │   ├── visibility.go                 # capability visibility resolver (strictest-wins across scopes)
│   │   ├── policy.go                     # validation + fail-closed rules (FR-009, FR-031)
│   │   ├── projection.go                 # BuildRuntimeBindingEvidence (applied classification)
│   │   ├── redaction.go                  # safe summaries/labels
│   │   └── *_test.go
│   ├── store/
│   │   ├── store.go                      # CurrentSchemaVersion 53→54; migration r58_workspace_capability_binding
│   │   ├── workspace_store.go            # workspace CRUD + lazy EnsureDefaultWorkspace
│   │   ├── binding_store.go              # binding rule + capability visibility persistence
│   │   ├── binding_projection.go         # runtime binding evidence persistence/queries
│   │   ├── workspace_store_test.go
│   │   ├── binding_store_test.go
│   │   └── binding_restart_test.go
│   ├── store/tenancy/
│   │   ├── bindings.go                   # BindingAccessScope (CanInspect/CanManage)
│   │   └── bindings_test.go
│   ├── api/
│   │   ├── workspace_bindings.go         # /v1/workspaces + /v1/bindings + /v1/capability-visibility routes
│   │   ├── workspace_bindings_test.go
│   │   ├── thread_lifecycle.go           # additive binding projection in run/thread detail
│   │   └── server.go                     # route registration (protected + withByIDTenantGuard)
│   ├── chat/
│   │   ├── service.go                    # resolve binding selection alongside resolveActiveProfile;
│   │   │                                 #   record binding runtime evidence; fail closed on invalid
│   │   └── binding_projection_test.go
│   ├── events/
│   │   ├── workspace_capability_bindings.go   # lifecycle + denied + runtime-projected constructors
│   │   └── workspace_capability_bindings_test.go
│   ├── profiles/
│   │   └── projection.go                  # flip deferred classification when an applied binding exists
│   └── contracts/
│       └── workspace_capability_binding_contracts_test.go

schemas/
├── api/
│   ├── workspace-resource.schema.json
│   ├── binding-rule-resource.schema.json
│   ├── capability-visibility-policy.schema.json
│   ├── effective-binding-selection.schema.json
│   ├── binding-runtime-evidence.schema.json
│   ├── binding-repair-status.schema.json
│   ├── workspace-list.response.schema.json
│   ├── binding-list.response.schema.json
│   ├── create-binding.request.schema.json
│   ├── update-binding.request.schema.json
│   ├── update-capability-visibility.request.schema.json
│   ├── thread-detail.response.schema.json       # additive binding projection block
│   ├── run-resource.schema.json                 # additive binding evidence block
│   └── tenant-permission-resource.schema.json   # add bindings.* permissions
└── events/
    ├── binding-lifecycle.event.schema.json
    ├── capability-visibility-changed.event.schema.json
    ├── binding-runtime-projected.event.schema.json
    └── tenant-permission-denied.event.schema.json   # reuse; add bindings gates

sdk/ts/src/
├── index.ts                              # binding/workspace/visibility types + client methods
└── workspace-capability-binding.test.ts

web/src/
├── features/workspace-capability-bindings/
│   ├── WorkspaceBindingEditor.tsx
│   ├── CapabilityVisibilityPanel.tsx
│   ├── BindingRuntimeEvidence.tsx
│   └── workspace-binding-editor.test.tsx
├── features/thread-lifecycle.tsx         # surface binding evidence on run/thread detail
└── app/App.tsx

tui/src/
├── cli.ts                                # binding list/inspect commands
└── cli.test.ts

docs/
├── runtime/workspace-capability-binding.md
├── runtime/thread-session-lifecycle.md   # note binding evidence in runtime projection
└── providers/provider-identity-and-profiles.md  # cross-link profile↔workspace binding
```

**Structure Decision**: Implement workspace + capability binding as a **new `bindings`
domain package** that mirrors the mature `profiles` package (types + policy + precedence +
visibility resolver + projection + redaction), with additive store tables, tenancy guards,
API routes, schemas, events, and client surfaces. Binding resolution hooks the **existing
work-start path** in `chat/service.go` next to `resolveActiveProfile`
(`ActiveAgentProfileSelection`), so binding selection is resolved and recorded in the same
pass that already records the profile runtime projection. Workspace is a persisted record
kept separate from memory, filesystem, and provider-auth state. The previously planted
`roadmap_58_deferred_binding_unapplied` classification becomes an applied binding
classification once an explicit binding influences a run.

## Roadmap 58 Planning Contracts

The implementation plan MUST keep this artifact complete before `/speckit.tasks`:

- [contracts/workspace-capability-binding.md](./contracts/workspace-capability-binding.md)
  - Workspace record shape + lazy default provisioning; binding rule shapes (channel,
    integration-account); capability visibility policy shape + scopes; precedence resolution;
    fail-closed invalid resolution; effective selection + runtime binding evidence; permission
    gates; validation + repair status; API routes; SDK/Web/TUI expectations; event + audit
    evidence; JSON schemas; redaction; compatibility; migration; and rollback.

This artifact is a planning gate. Implementation is incomplete if any binding can affect
runtime behavior, capability availability, operator inspection, or historical evidence
without a contract row and a proving test.

## Migration And Rollback Plan

1. Add additive schema **migration v54** (`r58_workspace_capability_binding`) with tables:
   - `workspaces` (workspace_id, tenant_id, display_name, status, is_default, owner_principal,
     repair_status, redaction_status, created_at, updated_at, archived_at, document_json).
   - `binding_rules` (binding_id, tenant_id, scope_kind=`channel|integration_account`,
     scope_ref, selected_profile_id, selected_profile_version_id, selected_workspace_id,
     status, repair_status, actor, audit/event refs, validation_status, created_at,
     updated_at, disabled_at, document_json).
   - `capability_visibility_policies` (policy_id, tenant_id, scope_kind=`profile|workspace`,
     scope_ref, capability_id, visibility=`visible|hidden|disabled|default_enabled`, actor,
     validation_status, created_at, updated_at, document_json).
   - `binding_runtime_projections` (projection_id, tenant_id, resource_kind, resource_id,
     selected_profile_id, selected_workspace_id, binding_scope, capability_visibility_summary,
     selection_reason, classification, occurred_at, redaction_status, document_json).
   Bump `CurrentSchemaVersion` 53 → 54.
2. Add `bindings.inspect` and `bindings.manage` to `identity` permissions + role-derived
   grants + `AllSensitivePermissions`. Owners/admins receive both by default; other roles only
   if product policy requires it during implementation.
3. **Lazy** default-workspace provisioning: do NOT run a bulk migration over all tenants.
   `EnsureDefaultWorkspace(tenantID)` creates/identifies one default personal workspace
   idempotently on first binding or resolution access; concurrent first access converges on a
   single record (unique constraint on `tenant_id WHERE is_default`).
4. Roll out in stages: (a) schema + permissions; (b) read-only default workspace + binding
   projection on new work (record-only, no behavior change); (c) workspace + binding
   list/detail; (d) binding create/update + precedence resolution applied to new work;
   (e) capability visibility + default enablement enforcement; (f) disable/remove/repair +
   fail-closed invalid resolution.
5. On rollback, disable binding/workspace/capability-visibility mutations and ignore new
   binding selection changes (new work falls back to tenant-default profile + default
   workspace + default visibility) while preserving already-recorded binding state, capability
   visibility, audit events, repair status, and runtime binding evidence for authorized
   inspection. Existing chat, profile, thread, run, workflow, handoff, channel, connector, and
   integration behavior remains compatible.
6. Irreversible behavior is limited to recording metadata-only evidence rows and creating
   default workspace records. No memory backfill, no physical workspace storage migration, no
   marketplace, no autonomous capability selection, no filesystem grant, and no destructive
   rewrite of historical run/thread/connector evidence is allowed in Phase 58.

## Post-Design Constitution Re-check

- **Roadmap closure** — PASS. `research.md`, `data-model.md`, the contract, and `quickstart.md`
  cover the full Roadmap 58 surface across identity, store, bindings domain (precedence +
  visibility + fail-closed), API, schema, events/audit, SDK/Web/TUI, runtime projection, lazy
  default workspace, repair status, restart recovery, redaction, and verification.
- **Production-grade, minimal, reversible change** — PASS. Design artifacts preserve existing
  profile/thread/run/connector/integration behavior while adding binding truth only through
  explicit tenant-owned records and runtime projection, reusing the existing resolution call
  site.
- **Contracts and auditability** — PASS. The contract defines route shapes, workspace/binding/
  visibility/evidence resources, event + audit evidence, permission gates, validation +
  repair, fail-closed resolution, redaction, compatibility, migration, and rollback.
- **Verification and observability** — PASS. The quickstart + contract identify targeted Go,
  contract, SDK, Web, TUI, restart, redaction, migration, precedence, fail-closed, capability
  enforcement, and non-memory/non-filesystem checks.
- **Environment and secrets** — PASS. Verification uses test-environment seeded/fake evidence
  by default, with no production tenants or live connector credentials required.

No post-design violations require justification.

## Complexity Tracking

No constitution violations or complexity exceptions. The one notable addition — a separate
`bindings` domain package rather than extending `profiles` — is justified because binding is a
distinct tenant-owned concern (channels, accounts, workspaces, capability policy) with its own
permissions, precedence, and fail-closed semantics; folding it into `profiles` would widen the
profile blast radius and blur the deferral boundary the codebase deliberately planted.
