# Phase 1 Data Model: Workspace And Capability Binding

Derived from `spec.md` Key Entities + Functional Requirements and the persistence patterns in
`daemon/internal/store` (migration 53 / `agent_profiles`). All new state is tenant-scoped,
additive, and recorded with a `document_json` blob plus indexed columns, following the
existing store convention. Schema **migration v54** (`r58_workspace_capability_binding`),
`CurrentSchemaVersion` 53 → 54.

Conventions: every table carries `tenant_id` (indexed, never cross-tenant joined),
`redaction_status`, and timestamps. Identifiers use typed prefixes consistent with the repo
(`ws_`, `bnd_`, `cvp_`, `brp_`).

---

## 1. Workspace  (`workspaces` table)

A persisted tenant-owned record used for binding identity, safe display, status, audit,
repair, and future context inputs. Grants no storage/filesystem access (FR-020). Default
personal workspace is one such record, lazily provisioned (FR-002, FR-025).

| Field | Type | Notes |
|-------|------|-------|
| `workspace_id` | string PK | `ws_*` |
| `tenant_id` | string, indexed | owner tenant |
| `display_name` | string | safe label |
| `status` | enum | `active` \| `archived` \| `disabled` |
| `is_default` | bool | exactly one `true` per tenant |
| `owner_principal_id` | string | creating/owning principal |
| `repair_status` | enum | `healthy` \| `disabled` \| `invalid` \| `stale` \| `unsupported` \| `needs_repair` |
| `redaction_status` | enum | `redacted` \| `not_required` |
| `created_at` / `updated_at` / `archived_at` | timestamp | |
| `document_json` | blob | full safe document |

- **Uniqueness**: PK `workspace_id`; partial unique index on `(tenant_id) WHERE is_default = 1`
  (guarantees single lazy default; FR-025 concurrency).
- **Validation**: non-empty safe `display_name`; tenant match; status transitions
  `active → archived|disabled` only (no hard delete while runtime evidence may reference it).
- **Provisioning**: `EnsureDefaultWorkspace(tenantID)` inserts-or-gets the default record on
  first binding/resolution access (mirror `EnsureDefaultAgentProfile`).

---

## 2. Binding Rule  (`binding_rules` table)

Tenant-owned configuration connecting a binding scope to a selected profile and/or workspace.
Covers **Channel Binding** and **Integration Account Default Binding** via `scope_kind`.

| Field | Type | Notes |
|-------|------|-------|
| `binding_id` | string PK | `bnd_*` |
| `tenant_id` | string, indexed | |
| `scope_kind` | enum | `channel` \| `integration_account` |
| `scope_ref` | string, indexed | tenant-owned channel identity (Roadmap 48) or integration-account identity (Roadmap 37) |
| `selected_profile_id` | string, nullable | references `agent_profiles` |
| `selected_profile_version_id` | string, nullable | pinned/active version |
| `selected_workspace_id` | string, nullable | references `workspaces`; channel scope only |
| `status` | enum | `active` \| `disabled` |
| `repair_status` | enum | same set as Workspace |
| `validation_status` | enum | `valid` \| `invalid` |
| `actor_principal_id` | string | last mutator |
| `audit_ref` / `event_ref` | string | evidence linkage |
| `created_at` / `updated_at` / `disabled_at` | timestamp | |
| `document_json` | blob | safe document incl. previous/resulting selection summaries |

- **Uniqueness**: at most one **active** rule per `(tenant_id, scope_kind, scope_ref)`
  (partial unique index on `status = 'active'`). Prevents ambiguous precedence at a single
  scope (edge case: deterministic resolution).
- **Relationships**: `selected_profile_id → workspaces`/`agent_profiles` are validated at
  create/update and re-checked at resolution; dangling references flip `repair_status`
  (FR-022) and resolution fails closed (FR-031).
- **Scope rules**: `integration_account` scope supplies profile default only (FR-005); it does
  not carry `selected_workspace_id`. `channel` scope may carry profile and/or workspace.
- **Validation** (FR-009): reject cross-tenant `scope_ref`/profile/workspace, unavailable
  channels, disconnected accounts, archived/disabled profiles, unavailable workspaces,
  unsupported connector binding surfaces, malformed values, and policy-conflicting selections,
  each with a safe user-visible reason.

---

## 3. Capability Visibility Policy  (`capability_visibility_policies` table)

Tenant-owned policy describing whether a capability is visible/hidden/disabled/default-enabled
for a binding scope. User-editable at **profile and workspace** scope only (FR-018); tenant +
connector limits apply as higher-level strictest-wins constraints at resolution (FR-017).

| Field | Type | Notes |
|-------|------|-------|
| `policy_id` | string PK | `cvp_*` |
| `tenant_id` | string, indexed | |
| `scope_kind` | enum | `profile` \| `workspace` |
| `scope_ref` | string, indexed | profile_id or workspace_id |
| `capability_id` | string, indexed | existing product capability/skill id |
| `visibility` | enum | `visible` \| `hidden` \| `disabled` \| `default_enabled` |
| `actor_principal_id` | string | |
| `validation_status` | enum | `valid` \| `invalid` |
| `created_at` / `updated_at` | timestamp | |
| `document_json` | blob | safe document |

- **Uniqueness**: unique `(tenant_id, scope_kind, scope_ref, capability_id)`.
- **Semantics**:
  - `default_enabled` influences what is offered by default but MUST NOT override hidden/
    disabled/unavailable/disallowed/unconfigured policy (FR-019).
  - Effective visibility = strictest of {tenant, connector} limits ∧ profile policy ∧
    workspace policy (FR-017). `hidden`/`disabled` from any applicable scope wins.
  - Channel and integration-account bindings do NOT carry capability-visibility policy in this
    phase (no `scope_kind` for them here); any capability limit they imply is enforced through
    the resolved profile/workspace policy plus tenant/connector limits (FR-017).
- **Enforcement**: hidden/disabled capabilities are absent from offered choices (FR-015) and
  cannot execute via direct request, agent choice, client call, connector payload, replay, or
  stale state (FR-016).

---

## 4. Effective Binding Selection  (resolved, not persisted as its own table)

Go type name: `EffectiveBindingSelection` (in `daemon/internal/bindings/binding.go`).

The resolved `(profile, profile_version, workspace, binding_scope, capability_visibility_set)`
computed at work-start. Materialized into Runtime Binding Evidence (below); not a standalone
durable table.

- **Precedence** (FR-006): channel binding → integration-account default → tenant default.
  First match for profile wins; workspace comes from the channel binding when present, else the
  tenant default workspace.
- **Outcomes**:
  - `resolved` — valid selection recorded.
  - `repair_required` — referenced profile/workspace invalid/unavailable → **fail closed**
    (FR-031); no silent substitution, no fall-through to lower precedence.
  - `default` — no explicit binding → tenant default profile + default workspace + default
    visibility (legacy-compatible; labeled default, FR-026).

---

## 5. Runtime Binding Evidence  (`binding_runtime_projections` table)

Durable evidence attached to run/thread/session/workflow/handoff/channel inspection showing
which binding selections influenced execution. Recorded in the same work-start pass as the
profile runtime projection.

**Canonical naming**: "Runtime Binding Evidence" is the canonical spec/operator term for this
entity. `binding_runtime_projections` is the storage table name and "binding projection" is
the internal builder name (`BuildRuntimeBindingEvidence`); all three refer to the same thing.

| Field | Type | Notes |
|-------|------|-------|
| `projection_id` | string PK | `brp_*` |
| `tenant_id` | string, indexed | |
| `resource_kind` | enum | `thread` \| `session` \| `run` \| `workflow` \| `handoff` \| `channel` |
| `resource_id` | string, indexed | |
| `selected_profile_id` | string | |
| `selected_profile_version_id` | string | |
| `selected_workspace_id` | string | |
| `binding_scope` | enum | `channel` \| `integration_account` \| `tenant_default` |
| `binding_id` | string, nullable | the rule that applied, if any |
| `capability_visibility_summary` | blob | safe per-capability visible/hidden/disabled/blocked summary |
| `classification` | enum | `applied_binding` \| `default_binding` \| `legacy_default` |
| `selection_reason` | string | safe reason / denied-capability safe reason (FR-014) |
| `occurred_at` | timestamp | |
| `redaction_status` | enum | |
| `document_json` | blob | |

- **Immutability** (FR-012): evidence is append-only; binding changes never rewrite prior
  projections. Historical inspection shows the selection that applied when the run started.
- **Classification** flips the planted `roadmap_58_deferred_binding_unapplied` marker: runs
  influenced by an explicit binding record `applied_binding`; otherwise `default_binding` /
  `legacy_default` (FR-026).
- **Denied capabilities** (FR-014): records safe policy reason + applicable binding scope,
  never secrets/raw payloads/cross-tenant details.

---

## 6. Binding Audit Event  (`tenant_audit` rows + `events` bus)

Tenant-scoped evidence for binding creation, update, disablement, removal, repair, validation
failure, permission denial, capability visibility change, and runtime selection outcomes.

- **Required fields** (FR-010): tenant, actor, scope, affected resource labels, previous
  selection summary, resulting selection summary, timestamp, outcome.
- **Fail-closed** (FR-011): mutation aborts and leaves state unchanged if audit/event evidence
  cannot be recorded (`Auditor.Require()` → `ErrAuditWriteFailed`).
- **Event names** (`events/workspace_capability_bindings.go`): `binding.created`,
  `binding.updated`, `binding.disabled`, `binding.removed`, `binding.repaired`,
  `binding.validation_failed`, `binding.permission_denied`, `capability_visibility.changed`,
  `binding.runtime_projected`.

---

## 7. Binding Repair Status  (column on Workspace / Binding Rule)

Safe user-facing health state: `healthy | disabled | invalid | stale | unsupported |
needs_repair` (FR-022). A binding referencing an inactive profile, unavailable workspace,
removed channel, disconnected account, or unsupported connector is marked needing repair and
cannot silently affect new work (User Story 4 scenario 2; FR-031).

---

## 8. Capability  (existing surface, referenced not redefined)

A tenant-visible action/integration surface enumerated from `daemon/internal/capabilities` +
`daemon/internal/skills`. Phase 58 controls **visibility + default enablement** via Capability
Visibility Policy; it does not create a plugin/community marketplace (Out of Scope / FR-029)
and does not guarantee a capability succeeds once invoked (Assumptions).

---

## Entity Relationship Summary

```text
Tenant
 ├─1:N─ Workspace            (exactly one is_default)
 ├─1:N─ BindingRule          (scope_kind: channel | integration_account)
 │        ├─ref→ AgentProfile (selected_profile_id, R57)
 │        └─ref→ Workspace    (selected_workspace_id; channel scope only)
 ├─1:N─ CapabilityVisibilityPolicy  (scope: profile | workspace) ─ref→ Capability (R-existing)
 └─1:N─ RuntimeBindingEvidence(append-only) ─ref→ {thread,session,run,workflow,handoff,channel}

Resolution(work-start):  channel BindingRule → integration_account BindingRule → tenant default
                         → EffectiveBindingSelection → {resolved | repair_required(fail closed) | default}
                         → append RuntimeBindingEvidence
```

## State Transitions

- **Workspace**: `active → archived` (retire), `active → disabled` (operational pause),
  `disabled → active` (re-enable). No hard delete while referenced.
- **Binding Rule**: `active → disabled → active`; remove is allowed only after disable or with
  explicit `bindings.manage`, leaving historical evidence intact (FR-012).
- **Capability Visibility Policy**: edits replace prior value in place (audited); higher-level
  policy limits are never overridden by edits (FR-018).
- **Repair**: any referenced-resource loss flips `repair_status` to `needs_repair`/`invalid`/
  `stale`/`unsupported`; recovery is an explicit `bindings.manage` repair action (FR-031).
