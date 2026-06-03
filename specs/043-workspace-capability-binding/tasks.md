---
description: "Task list for Workspace And Capability Binding (Roadmap 58)"
---

# Tasks: Workspace And Capability Binding

**Input**: Design documents from `/specs/043-workspace-capability-binding/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/workspace-capability-binding.md, quickstart.md

**Tests**: REQUIRED per constitution (Principle IV). Every behavior in
`contracts/workspace-capability-binding.md` (B1–B24) maps to a proving test below. Contract
tests are mandatory because API/schema/event/persistence surfaces change.

**Organization**: Grouped by user story. P1 stories (US1–US3) form the core slice; US4–US5
(P2) complete management surfaces and boundary guarantees. MVP = US1.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no incomplete dependencies)
- **[Story]**: US1–US5 for story phases; Setup/Foundational/Polish carry no story label
- All paths are repo-relative to `/Users/John/Code/dope-agent`

## Path Conventions

Multi-surface daemon product. Go daemon under `daemon/internal/`, JSON Schemas under
`schemas/`, SDK under `sdk/ts/src/`, web under `web/src/`, TUI under `tui/src/`, docs under
`docs/`.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the new domain/package scaffolding referenced by all later phases.

- [X] T001 Create the bindings domain package directory `daemon/internal/bindings/` with a package doc comment in `daemon/internal/bindings/doc.go`
- [X] T002 [P] Create the web feature directory `web/src/features/workspace-capability-bindings/` with an empty `index.ts` barrel
- [X] T003 [P] Create the contract test skeleton `daemon/internal/contracts/workspace_capability_binding_contracts_test.go` referencing the new schema paths (table stub, no assertions yet)

**Checkpoint**: Package/dir scaffolding exists; nothing wired.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Permissions, schema/migration, base store, base domain types, tenancy guard,
events, and route registration that EVERY user story depends on.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

### Permissions

- [X] T004 Add `PermissionBindingsInspect` (`bindings.inspect`) and `PermissionBindingsManage` (`bindings.manage`) constants and append to `AllSensitivePermissions` in `daemon/internal/identity/types.go`
- [X] T005 Grant `bindings.inspect`/`bindings.manage` to owner/admin role-derived sets in `daemon/internal/identity/permissions.go`
- [X] T006 [P] Add permission-grant + denial unit tests in `daemon/internal/identity/permissions_test.go`
- [X] T007 [P] Add the two permission ids to `schemas/api/tenant-permission-resource.schema.json`

### Schema / migration

- [X] T008 Bump `CurrentSchemaVersion` 53 → 54 and register additive migration `r58_workspace_capability_binding` (tables `workspaces`, `binding_rules`, `capability_visibility_policies`, `binding_runtime_projections` with indexes per data-model.md) in `daemon/internal/store/store.go`
- [X] T009 Add the partial unique index `(tenant_id) WHERE is_default = 1` on `workspaces` and `(tenant_id, scope_kind, scope_ref) WHERE status='active'` on `binding_rules` within the same migration in `daemon/internal/store/store.go`

### Base domain types + redaction

- [X] T010 [P] Define core types (`Workspace`, `BindingRule`, `CapabilityVisibilityPolicy`, `EffectiveBindingSelection`, `RuntimeBindingEvidence`, `RepairStatus`, scope/visibility/outcome enums) in `daemon/internal/bindings/binding.go`
- [X] T011 [P] Implement safe-summary/label redaction helpers in `daemon/internal/bindings/redaction.go` with `daemon/internal/bindings/redaction_test.go`

### Base store + tenancy guard

- [X] T012 Implement workspace persistence + lazy `EnsureDefaultWorkspace(ctx, tenantID)` (insert-or-get, idempotent) in `daemon/internal/store/workspace_store.go`
- [X] T013 Implement binding-rule + capability-visibility persistence scaffolding (create/read/list/update/disable/remove) in `daemon/internal/store/binding_store.go`
- [X] T014 Implement runtime binding evidence persistence (`RecordRuntimeBindingEvidence`, list/queries, append-only) in `daemon/internal/store/binding_projection.go`
- [X] T015 [P] Implement `BindingAccessScope` (`CanInspect`/`CanManage`, tenant + permission checks) in `daemon/internal/store/tenancy/bindings.go` with `daemon/internal/store/tenancy/bindings_test.go`

### Events / audit base

- [X] T016 [P] Implement binding event constructors (lifecycle, capability-visibility-changed, runtime-projected, permission-denied) in `daemon/internal/events/workspace_capability_bindings.go` with `daemon/internal/events/workspace_capability_bindings_test.go`

### API registration scaffold

- [X] T017 Register `/v1/workspaces`, `/v1/workspaces/`, `/v1/bindings`, `/v1/bindings/`, and `/v1/capability-visibility` routes via `protected(...)` + `withByIDTenantGuard(...)` and add empty handlers in `daemon/internal/api/workspace_bindings.go` + `daemon/internal/api/server.go`

**Checkpoint**: Permissions, schema v54, base store, domain types, tenancy guard, events, and
routes exist. User stories can now proceed (in parallel if staffed).

---

## Phase 3: User Story 1 - Bind Channels And Accounts To Profile And Workspace Defaults (Priority: P1) 🎯 MVP

**Goal**: New work from a channel/account resolves the correct profile + workspace via
deterministic precedence (channel → account → tenant default), with creation audited,
tenant-isolated, validated, and fail-closed on invalid references.

**Independent Test**: Create a default workspace, bind a channel to a profile+workspace,
configure an account default, start new work from each source, and confirm each resolves the
expected profile+workspace; confirm an explicit channel binding survives a later account
default change; confirm an invalid binding fails closed.

### Tests for User Story 1 ⚠️ (write first, ensure they fail)

- [X] T018 [P] [US1] Precedence resolver tests (channel→account→tenant default; channel stable when account default changes — B3, B4) in `daemon/internal/bindings/precedence_test.go`
- [X] T019 [P] [US1] Validation + fail-closed policy tests (cross-tenant/unavailable/malformed reject with safe reason B18; invalid profile/workspace → `repair_required`, no silent substitution B5) in `daemon/internal/bindings/policy_test.go`
- [X] T020 [P] [US1] Lazy default-workspace store tests (one per tenant, concurrent first-access converges — B6) in `daemon/internal/store/workspace_store_test.go`
- [X] T021 [P] [US1] Create-binding API tests (CRUD create B1; tenant isolation + no existence leak B2; audit/event emitted B11) in `daemon/internal/api/workspace_bindings_test.go`
- [X] T022 [P] [US1] Work-start resolution test (binding selection produced and recorded; fail-closed work-start on invalid B5) in `daemon/internal/chat/binding_projection_test.go`

### Implementation for User Story 1

- [X] T023 [US1] Implement deterministic precedence resolver `ResolveSelection(...)` (channel → integration-account → tenant default; workspace from channel binding else default) in `daemon/internal/bindings/precedence.go`
- [X] T024 [US1] Implement validation + fail-closed rules (`ValidateBinding`, `repair_required` outcome distinct from `default`) in `daemon/internal/bindings/policy.go`
- [X] T025 [US1] Implement create-binding store path with channel/account scope_ref validation against connector/integration identity in `daemon/internal/store/binding_store.go`
- [X] T026 [US1] Implement `POST /v1/bindings` handler (create channel/account binding, `bindings.manage` gate, audit `Auditor.Require()`, emit lifecycle event) in `daemon/internal/api/workspace_bindings.go`
- [X] T027 [US1] Hook binding resolution into work-start alongside `resolveActiveProfile` (resolve profile+workspace from a single consistent read so exactly one selection is recorded per work item even under a concurrent binding/visibility change — FR-033; fail closed on `repair_required` with safe evidence) in `daemon/internal/chat/service.go`
- [X] T028 [P] [US1] Add `create-binding.request.schema.json`, `binding-rule-resource.schema.json`, `workspace-resource.schema.json`, `effective-binding-selection.schema.json`, `binding-repair-status.schema.json` under `schemas/api/`
- [X] T029 [US1] Extend the contract test in `daemon/internal/contracts/workspace_capability_binding_contracts_test.go` to validate the US1 schemas + fixtures

**Checkpoint**: Channel/account bindings resolve deterministically on new work; MVP is
independently testable.

---

## Phase 4: User Story 2 - Control Capability Visibility And Default Enablement (Priority: P1)

**Goal**: Hide/disable/default-enable capabilities at profile + workspace scope; hidden or
disabled capabilities are absent from offered choices and cannot execute, with strictest
policy winning.

**Independent Test**: Hide/disable representative capabilities for a profile and a workspace,
start new work, and confirm hidden/disabled capabilities are not offered and cannot execute
even on direct request; confirm `default_enabled` cannot override a stricter hidden/disabled
policy.

### Tests for User Story 2 ⚠️

- [X] T030 [P] [US2] Visibility resolver tests (strictest-wins across tenant/connector/profile/workspace B8; `default_enabled` cannot override hidden/disabled/unavailable B10) in `daemon/internal/bindings/visibility_test.go`
- [X] T031 [P] [US2] Enforcement tests (hidden/disabled cannot execute via direct/agent/client/connector/replay/stale B9) in `daemon/internal/bindings/visibility_enforcement_test.go`
- [X] T032 [P] [US2] Capability-visibility API tests (set/list at profile+workspace scope, `bindings.manage` gate, event emitted) in `daemon/internal/api/workspace_bindings_test.go`

### Implementation for User Story 2

- [X] T033 [US2] Implement capability-visibility resolver (effective = strictest of tenant/connector limits ∧ profile ∧ workspace; FR-017/19) in `daemon/internal/bindings/visibility.go`
- [X] T034 [US2] Implement capability-visibility persistence (set/list per profile/workspace scope, uniqueness) in `daemon/internal/store/binding_store.go`
- [X] T035 [US2] Implement `GET/PUT /v1/capability-visibility` handlers (profile+workspace scope only, audit + `capability_visibility.changed` event) in `daemon/internal/api/workspace_bindings.go`
- [X] T036 [US2] Wire visibility enforcement into the runtime capability gate so hidden/disabled capabilities are neither offered nor executable in `daemon/internal/policy/` (enforcement point) and consumed at work-start in `daemon/internal/chat/service.go`
- [X] T037 [P] [US2] Add `capability-visibility-policy.schema.json` and `update-capability-visibility.request.schema.json` under `schemas/api/`; add `capability-visibility-changed.event.schema.json` under `schemas/events/`
- [X] T038 [US2] Extend the contract test to validate US2 schemas/fixtures in `daemon/internal/contracts/workspace_capability_binding_contracts_test.go`

**Checkpoint**: Capability visibility is enforced product truth; US1 + US2 work independently.

---

## Phase 5: User Story 3 - Inspect Runtime Binding Evidence (Priority: P1)

**Goal**: Each run records the active binding identities, selected profile/workspace, binding
scope, and capability visibility summary; historical evidence is immutable; the planted
deferral classification flips to applied.

**Independent Test**: Start work before/after binding changes, inspect runtime evidence per
run, and confirm each record shows active binding identities, selected profile+workspace,
capability summary, and safe denial reasons; confirm prior runs are unchanged after a binding
change; confirm restart preserves evidence.

### Tests for User Story 3 ⚠️

- [X] T039 [P] [US3] Runtime evidence tests (profile+workspace+scope+capability summary recorded on new work B13; denied-capability safe reason, no secrets/cross-tenant B14; evidence exposes a per-capability decision + reason sufficient to explain a visibility/denial outcome — SC-012) in `daemon/internal/chat/binding_projection_test.go`
- [X] T040 [P] [US3] Historical immutability test (binding change does not rewrite prior projections B15) in `daemon/internal/store/binding_store_test.go`
- [X] T041 [P] [US3] Classification test (applied vs default/legacy; planted `roadmap_58_deferred_binding_unapplied` marker flips B22) in `daemon/internal/store/binding_projection_test.go`

### Implementation for User Story 3

- [X] T042 [US3] Implement `BuildRuntimeBindingEvidence(...)` (classification `applied_binding`/`default_binding`/`legacy_default`, capability visibility summary, safe selection reason) in `daemon/internal/bindings/projection.go`
- [X] T043 [US3] Record binding runtime evidence in the same work-start pass as `recordActiveProfileProjection` and publish `binding.runtime_projected` in `daemon/internal/chat/service.go`
- [X] T044 [US3] Flip the deferral classification when an explicit binding influenced the run (set applied classification; keep default/legacy labeling otherwise) in `daemon/internal/profiles/projection.go`
- [X] T045 [US3] Add additive `bindingProjection` block to thread/run detail in `daemon/internal/api/thread_lifecycle.go`
- [X] T046 [P] [US3] Add `binding-runtime-evidence.schema.json` under `schemas/api/`, the additive `bindingProjection` block to `schemas/api/thread-detail.response.schema.json` + `schemas/api/run-resource.schema.json`, and `binding-runtime-projected.event.schema.json` under `schemas/events/`
- [X] T047 [US3] Update `daemon/internal/contracts/agent_profile_contracts_test.go` fixtures that reference `roadmap_58_deferred_binding_unapplied` to the applied-binding value, and extend the binding contract test for US3 schemas

**Checkpoint**: Runtime binding evidence is durable, explainable, and immutable; US1–US3
(all P1) work independently.

---

## Phase 6: User Story 4 - Manage Bindings Through Product And Client Surfaces (Priority: P2)

**Goal**: Full binding lifecycle (list, inspect, update, disable, remove, repair) through API
+ SDK + Web + TUI with repair status, audit-write fail-closed, and backward-compatible client
behavior.

**Independent Test**: Manage bindings + capability visibility through product and client
flows; confirm validation, audit records, permission checks, repair status, and that older
clients ignoring binding fields keep working.

### Tests for User Story 4 ⚠️

- [X] T048 [P] [US4] Lifecycle API tests (binding list/inspect/update/disable/remove/repair; workspace create/archive/disable under `bindings.manage` — FR-032; safe labels/scope/status/last-change in list) in `daemon/internal/api/workspace_bindings_test.go`
- [X] T049 [P] [US4] Audit-write-failure test (mutation fails closed, state unchanged B12) in `daemon/internal/api/workspace_bindings_test.go`
- [X] T050 [P] [US4] Repair-status tests (inactive profile/unavailable workspace/removed channel/disconnected account/unsupported connector marked needs-repair, cannot silently affect new work B17) in `daemon/internal/bindings/policy_test.go`
- [X] T051 [P] [US4] SDK backward-compat test (older client without binding fields keeps default behavior B19) in `sdk/ts/src/workspace-capability-binding.test.ts`
- [X] T052 [P] [US4] Web binding editor test in `web/src/features/workspace-capability-bindings/workspace-binding-editor.test.tsx`

### Implementation for User Story 4

- [X] T053 [US4] Implement list/detail/update/disable/remove/repair handlers (`GET /v1/bindings`, `GET/PATCH/DELETE /v1/bindings/{id}`, `POST /v1/bindings/{id}/repair`, `GET /v1/workspaces`, `GET /v1/workspaces/{id}`, `POST /v1/workspaces`, `PATCH /v1/workspaces/{id}` for archive/disable — FR-032) with `bindings.inspect`/`bindings.manage` gates in `daemon/internal/api/workspace_bindings.go`
- [X] T054 [US4] Implement repair-status computation + `needs_repair` flips on referenced-resource loss in `daemon/internal/bindings/policy.go` and `daemon/internal/store/binding_store.go`
- [X] T055 [US4] Enforce audit-write fail-closed on all binding mutations via `Auditor.Require()` (`ErrAuditWriteFailed`) in `daemon/internal/api/workspace_bindings.go`
- [X] T056 [P] [US4] Add `workspace-list.response.schema.json`, `binding-list.response.schema.json`, `update-binding.request.schema.json`, `create-workspace.request.schema.json`, `update-workspace.request.schema.json` under `schemas/api/`; add `binding-lifecycle.event.schema.json` under `schemas/events/`
- [X] T057 [P] [US4] Add SDK types + client methods (workspaces, bindings, capability visibility, runtime evidence) in `sdk/ts/src/index.ts`
- [X] T058 [P] [US4] Implement web surfaces `WorkspaceBindingEditor.tsx`, `CapabilityVisibilityPanel.tsx`, `BindingRuntimeEvidence.tsx` in `web/src/features/workspace-capability-bindings/` and surface evidence in `web/src/features/thread-lifecycle.tsx`
- [X] T059 [P] [US4] Add TUI binding list/inspect commands in `tui/src/cli.ts` with `tui/src/cli.test.ts`
- [X] T060 [US4] Extend the contract test for US4 schemas/fixtures in `daemon/internal/contracts/workspace_capability_binding_contracts_test.go`

**Checkpoint**: Bindings are fully manageable across product + client surfaces with repair
and fail-closed audit.

---

## Phase 7: User Story 5 - Preserve Explicit Non-Memory And Non-Filesystem Boundaries (Priority: P2)

**Goal**: Workspace/capability bindings create no memory-backed knowledge, no filesystem
access, no marketplace, and no autonomous capability selection beyond policy-visible
capabilities.

**Independent Test**: Bind a workspace, attempt to infer memory/filesystem access from the
binding alone, and confirm only explicit binding state is recorded; confirm autonomous
selection is limited to policy-visible capabilities.

### Tests for User Story 5 ⚠️

- [X] T061 [P] [US5] Boundary tests (workspace binding grants no filesystem/repo/doc/connector/knowledge access B7; no memory-backed workspace knowledge or autonomous fact extraction B21) in `daemon/internal/bindings/binding_test.go`
- [X] T062 [P] [US5] Autonomous-selection-bounded test (agent can only choose policy-visible capabilities) in `daemon/internal/bindings/visibility_enforcement_test.go`
- [X] T063 [P] [US5] Redaction sweep test (zero exposed secrets/tokens/payloads/bodies/cross-tenant ids across inspection/evidence/audit/fixtures B20) in `daemon/internal/bindings/redaction_test.go`

### Implementation for User Story 5

- [X] T064 [US5] Add explicit guards/assertions that workspace binding resolution returns identity only (no filesystem/knowledge handle) in `daemon/internal/bindings/binding.go` and the resolution path in `daemon/internal/chat/service.go`
- [X] T065 [P] [US5] Document non-scope guarantees (no memory, no storage migration, no marketplace, no autonomous selection beyond visible) in `docs/runtime/workspace-capability-binding.md`

**Checkpoint**: Boundary guarantees are enforced and documented; all five stories complete.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Restart recovery, docs, observability, and full-suite verification across stories.

- [X] T066 [P] Restart-recovery tests (workspaces, bindings, capability visibility, audit, repair status, runtime evidence survive daemon restart + client reconnect B16) in `daemon/internal/store/binding_restart_test.go`
- [X] T067 [P] Author `docs/runtime/workspace-capability-binding.md` and cross-link `docs/runtime/thread-session-lifecycle.md` + `docs/providers/provider-identity-and-profiles.md`
- [X] T068 [P] Update `docs/runtime/daemon-roadmaps.md` / `docs/runtime/daemon-tasks.md` marking Roadmap 58 scope and the deferral-flip
- [X] T069 Run `make daemon-contract-test` and resolve any schema/fixture mismatches (incl. the flipped classification)
- [X] T070 Run `cd daemon && go test ./...` and `go mod tidy`; fix failures
- [X] T071 Run `pnpm build && pnpm test:clients`; fix SDK/Web/TUI failures
- [X] T072 Execute `specs/043-workspace-capability-binding/quickstart.md` smoke walkthrough in the test env (`~/.dope-test`, `:19192`) and confirm SC-001…SC-015 acceptance gates
- [X] T073 [P] Concurrency/atomicity test: a binding or capability-visibility change racing a work-start records exactly one resolved selection per work item with no mixed/partial state (FR-033, spec Edge Cases) in `daemon/internal/chat/binding_projection_test.go`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies.
- **Foundational (Phase 2)**: Depends on Setup. BLOCKS all user stories.
- **US1 (Phase 3)**: Depends on Foundational. MVP.
- **US2 (Phase 4)**: Depends on Foundational. Independent of US1 (separate visibility tables/resolver); enforcement consumes the work-start path US1 also touches — coordinate edits to `chat/service.go`.
- **US3 (Phase 5)**: Depends on Foundational. Records evidence for whatever selection exists; richest when US1/US2 are present but its evidence-recording tasks are self-contained.
- **US4 (Phase 6)**: Depends on Foundational; completes the lifecycle/surfaces over US1/US2 entities.
- **US5 (Phase 7)**: Depends on Foundational; verifies boundaries over US1–US2 behavior.
- **Polish (Phase 8)**: Depends on all targeted stories.

### Shared-file coordination (avoid conflicts)

- `daemon/internal/chat/service.go`: T027 (US1), T036 (US2), T043 (US3), T064 (US5) — sequence these; do not run in parallel.
- `daemon/internal/api/workspace_bindings.go`: T026 (US1), T035 (US2), T053/T055 (US4) — sequence.
- `daemon/internal/store/binding_store.go`: T025 (US1), T034 (US2), T054 (US4) — sequence.
- `daemon/internal/contracts/workspace_capability_binding_contracts_test.go`: T029/T038/T047/T060 — sequence.

### Within Each User Story

- Tests first (must fail) → resolver/domain → store → API/runtime → schema → contract.

### Parallel Opportunities

- Setup T002/T003 parallel.
- Foundational [P] tasks T006/T007, T010/T011, T015/T016 parallel.
- Each story's `[P]` test tasks run together; schema-only `[P]` tasks parallel with code in different files.
- US1–US5 can be staffed in parallel after Foundational, respecting the shared-file coordination list above.

---

## Parallel Example: User Story 1

```bash
# Tests together (different files):
Task: T018 Precedence resolver tests in daemon/internal/bindings/precedence_test.go
Task: T019 Validation/fail-closed tests in daemon/internal/bindings/policy_test.go
Task: T020 Lazy default-workspace tests in daemon/internal/store/workspace_store_test.go
Task: T021 Create-binding API tests in daemon/internal/api/workspace_bindings_test.go
Task: T022 Work-start resolution test in daemon/internal/chat/binding_projection_test.go
```

---

## Implementation Strategy

### MVP First (User Story 1)

1. Phase 1 Setup → Phase 2 Foundational → Phase 3 US1.
2. **STOP & VALIDATE**: channel/account bindings resolve deterministically on new work with
   audit, isolation, validation, and fail-closed behavior.
3. Demo: bind a channel, start work, confirm resolved selection.

### Incremental Delivery

1. Foundational ready → US1 (MVP) → US2 (capability visibility) → US3 (runtime evidence) →
   US4 (management surfaces) → US5 (boundary guarantees) → Polish.
2. Each P1 story (US1–US3) adds standalone value and is independently testable.

### Parallel Team Strategy

After Foundational: Dev A → US1, Dev B → US2, Dev C → US3, with the shared-file coordination
list owning `chat/service.go`, `workspace_bindings.go`, `binding_store.go`, and the contract
test as serialized hand-offs.

---

## Notes

- [P] = different files, no incomplete dependencies.
- Constitution requires tests to fail before implementation for new behavior.
- Audit-write fail-closed (FR-011) and invalid-resolution fail-closed (FR-031) are
  non-negotiable acceptance gates — verify explicitly, do not stub.
- Commit after each task or logical group; keep schema + fixture + code changes together.
- Redaction (FR-028) applies to every surface; never expose secrets/payloads/cross-tenant ids.
