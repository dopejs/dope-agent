# Contract: Workspace And Capability Binding (Roadmap 58)

Planning gate. Implementation is incomplete if any binding can affect runtime behavior,
capability availability, operator inspection, or historical evidence without a row here and a
proving test. Each row maps a behavior → surface → contract artifact → proving test (B1–B24).
All responses are tenant-scoped, redacted (FR-028), and backward compatible for clients that
do not understand binding fields (FR-024).

## Permissions

| Behavior | Gate | Denied response | Proving test |
|----------|------|-----------------|--------------|
| Read workspace/binding/visibility + runtime binding evidence | `bindings.inspect` | 403, no leak of inaccessible binding existence/details (FR-007) | `api/workspace_bindings_test.go`, `identity/permissions_test.go` |
| Create/update/disable/remove/repair binding; change capability visibility/default | `bindings.manage` | 403, binding state unchanged (FR-008) | `api/workspace_bindings_test.go` |

`bindings.inspect` / `bindings.manage` added to `identity/types.go` constants +
`AllSensitivePermissions` + role-derived grants (owners/admins default both).
Schema: `schemas/api/tenant-permission-resource.schema.json` gains the two permission ids.

## API Routes

Registered in `daemon/internal/api/server.go` via `protected(...)`; detail routes use
`withByIDTenantGuard(...)`.

| Method + Path | Purpose | Request schema | Response schema |
|---------------|---------|----------------|-----------------|
| `GET /v1/workspaces` | List tenant workspaces (default flagged) | — | `workspace-list.response.schema.json` |
| `GET /v1/workspaces/{id}` | Workspace detail + repair status | — | `workspace-resource.schema.json` |
| `POST /v1/workspaces` | Create workspace (`bindings.manage`; FR-032) | `create-workspace.request.schema.json` | `workspace-resource.schema.json` |
| `PATCH /v1/workspaces/{id}` | Archive/disable workspace (`bindings.manage`; no hard delete while referenced; FR-032) | `update-workspace.request.schema.json` | `workspace-resource.schema.json` |
| `GET /v1/bindings` | List channel + account bindings (safe labels, scope, status, selected profile/workspace, capability summary, last change) | — | `binding-list.response.schema.json` |
| `GET /v1/bindings/{id}` | Binding detail + repair status | — | `binding-rule-resource.schema.json` |
| `POST /v1/bindings` | Create channel/account binding | `create-binding.request.schema.json` | `binding-rule-resource.schema.json` |
| `PATCH /v1/bindings/{id}` | Update / disable binding | `update-binding.request.schema.json` | `binding-rule-resource.schema.json` |
| `DELETE /v1/bindings/{id}` | Remove binding (history preserved) | — | 204 |
| `POST /v1/bindings/{id}/repair` | Explicit safe repair | `update-binding.request.schema.json` | `binding-rule-resource.schema.json` |
| `GET /v1/capability-visibility` | List visibility policies for a profile/workspace scope | — (query: scope_kind, scope_ref) | `capability-visibility-policy.schema.json` (list) |
| `PUT /v1/capability-visibility` | Set capability visibility/default (profile/workspace scope) | `update-capability-visibility.request.schema.json` | `capability-visibility-policy.schema.json` |

Runtime evidence is surfaced additively (not new top-level routes):
`thread-detail.response.schema.json` and `run-resource.schema.json` gain an optional
`bindingProjection` block (`binding-runtime-evidence.schema.json`).

## Resource Shapes (JSON Schemas under `schemas/api/`)

| Schema | Key fields |
|--------|-----------|
| `workspace-resource.schema.json` | workspaceId, tenantId, displayName, status, isDefault, repairStatus, redactionStatus, timestamps |
| `binding-rule-resource.schema.json` | bindingId, tenantId, scopeKind(`channel`\|`integration_account`), scopeRef(safe label), selectedProfileId, selectedProfileVersionId, selectedWorkspaceId, status, repairStatus, validationStatus, previousSelectionSummary, resultingSelectionSummary, lastMaterialChangeAt |
| `capability-visibility-policy.schema.json` | policyId, tenantId, scopeKind(`profile`\|`workspace`), scopeRef, capabilityId, visibility(`visible`\|`hidden`\|`disabled`\|`default_enabled`), validationStatus |
| `effective-binding-selection.schema.json` | selectedProfileId/Version, selectedWorkspaceId, bindingScope, outcome(`resolved`\|`repair_required`\|`default`), capabilityVisibilitySummary |
| `binding-runtime-evidence.schema.json` | projectionId, resourceKind, resourceId, selectedProfileId/Version, selectedWorkspaceId, bindingScope, bindingId?, classification(`applied_binding`\|`default_binding`\|`legacy_default`), capabilityVisibilitySummary, selectionReason, occurredAt, redactionStatus |
| `binding-repair-status.schema.json` | enum `healthy`\|`disabled`\|`invalid`\|`stale`\|`unsupported`\|`needs_repair` |

## Behaviors → Proving Tests

| # | Behavior (FR/SC) | Surface | Proving test |
|---|------------------|---------|--------------|
| B1 | Binding CRUD lifecycle within tenant (FR-004/05, SC-001) | api+store | `api/workspace_bindings_test.go`, `store/binding_store_test.go` |
| B2 | Tenant isolation; no existence leak on denial (FR-007/08, SC-001) | api+tenancy | `store/tenancy/bindings_test.go`, `api/workspace_bindings_test.go` |
| B3 | Precedence channel → account → tenant default; resolved selection recorded (FR-006, SC-002) | bindings+chat | `bindings/precedence_test.go`, `chat/binding_projection_test.go` |
| B4 | Channel binding stable when account default later changes (US1 sc3) | bindings | `bindings/precedence_test.go` |
| B5 | Invalid profile/workspace → **fail closed**, no silent substitution (FR-031, SC-011) | bindings+chat | `bindings/policy_test.go`, `chat/binding_projection_test.go` |
| B6 | Lazy default workspace; one per tenant under concurrency (FR-002/25, SC-013) | store | `store/workspace_store_test.go` |
| B7 | Workspace binding grants no filesystem/connector/knowledge access (FR-020, SC-003/15) | bindings+chat | `bindings/binding_test.go`, `chat/binding_projection_test.go` |
| B8 | Capability visibility resolved strictest-wins across tenant/connector limits ∧ profile ∧ workspace; channel/account carry no capability policy (FR-015/17, SC-004) | bindings | `bindings/visibility_test.go` |
| B9 | Hidden/disabled capability cannot execute via direct/agent/client/connector/replay/stale (FR-016, SC-005) | bindings+policy | `bindings/visibility_test.go`, policy gate test |
| B10 | `default_enabled` cannot override hidden/disabled/unavailable (FR-019) | bindings | `bindings/visibility_test.go` |
| B11 | Audit/event on every change with required fields (FR-010, SC-006) | events+audit | `events/workspace_capability_bindings_test.go` |
| B12 | Audit-write failure → mutation fails closed, state unchanged (FR-011, SC-007) | api+store | `api/workspace_bindings_test.go` |
| B13 | Runtime binding evidence on new work; profile+workspace+scope+capability summary (FR-013, SC-008) | chat | `chat/binding_projection_test.go` |
| B14 | Denied-capability evidence has safe reason, no secrets/cross-tenant (FR-014, SC-014) | bindings+chat | `bindings/redaction_test.go`, `chat/binding_projection_test.go` |
| B15 | Historical evidence not rewritten on binding change (FR-012, SC-009) | store | `store/binding_store_test.go` |
| B16 | Restart recovery of bindings/visibility/audit/repair/evidence (FR-027, SC-010) | store | `store/binding_restart_test.go` |
| B17 | Repair status for unavailable profile/workspace/channel/account/connector/capability (FR-022, SC-011) | bindings+api | `bindings/policy_test.go`, `api/workspace_bindings_test.go` |
| B18 | Validation rejects cross-tenant/unavailable/malformed/policy-conflict with safe reason (FR-009) | bindings+api | `bindings/policy_test.go` |
| B19 | Backward-compatible defaults for older clients (FR-024, SC-013) | sdk+web | `sdk/ts/src/workspace-capability-binding.test.ts` |
| B20 | Redaction: zero exposed secrets/tokens/payloads/bodies/cross-tenant (FR-028, SC-014) | all | `bindings/redaction_test.go`, contract fixtures |
| B21 | Non-memory / non-marketplace / non-autonomous-selection guarantee (FR-021/29, SC-015) | bindings+docs | `bindings/binding_test.go`, doc assertion |
| B22 | Applied-binding classification flips planted deferral marker; contract fixtures updated (FR-026) | contracts | `contracts/workspace_capability_binding_contracts_test.go`, `contracts/agent_profile_contracts_test.go` |
| B23 | Workspace lifecycle: create/archive/disable under `bindings.manage`; no hard delete while referenced (FR-032) | api+store | `api/workspace_bindings_test.go`, `store/workspace_store_test.go` |
| B24 | Concurrent policy change racing work-start records exactly one resolved selection, no partial state (FR-033) | chat | `chat/binding_projection_test.go` |

## Events (`schemas/events/`)

| Schema | Trigger |
|--------|---------|
| `binding-lifecycle.event.schema.json` | created / updated / disabled / removed / repaired / validation_failed |
| `capability-visibility-changed.event.schema.json` | visibility or default-enablement change |
| `binding-runtime-projected.event.schema.json` | runtime binding evidence recorded on new work |
| `tenant-permission-denied.event.schema.json` (reuse) | `bindings.inspect`/`bindings.manage` denial |

## Compatibility, Migration, Rollback

- **Compatibility**: clients without binding awareness keep current channel/profile flows;
  new work with no explicit binding resolves to default profile + default workspace + default
  visibility (FR-024/25). New schema fields are additive/optional.
- **Migration**: additive v54 (`r58_workspace_capability_binding`); lazy default workspace; no
  bulk tenant migration; no destructive rewrite.
- **Rollback**: disable binding/workspace/visibility mutations; new work falls back to
  tenant-default behavior; recorded binding state, audit, repair, and runtime evidence
  preserved for inspection.

## Redaction Rule (applies to every surface above)

Binding inspection, runtime evidence, events, audit, logs, tests, and fixtures expose only
safe labels/scope/status/summaries via `bindings/redaction.go`. Never expose secrets, tokens,
raw provider payloads, unsafe capability inputs, sensitive message bodies, or cross-tenant
identifiers (FR-028, SC-014).
