# Phase 1 Data Model: Hosted Secrets, Integrations, And Connector Isolation

This document captures the entities, storage rules, and lifecycle transitions introduced
by Roadmap 37. The handoff table in
[`contracts/r37-handoff-table.md`](./contracts/r37-handoff-table.md) is the authoritative
cross-resource checklist.

## 1. Entities

### 1.1 Tenant Secret

Tenant-owned metadata for operator-owned credential material.

- `secret_id` — stable internal identifier.
- `tenant_id` — owning tenant. Required for every row and immutable after creation.
- `secret_ref` — tenant-local reference used by connector, MCP, sandbox, integration, or
  skill configuration.
- `display_name` — operator-visible label; not secret.
- `status` — `active`, `disabled`, or `pending_remediation`.
- `active_version_id` — current version used by new resolutions.
- `created_at`, `updated_at`, `rotated_at`, `disabled_at`.
- `document_json` — redacted metadata only; no raw value or derived credential material.

Validation rules:

- `(tenant_id, secret_ref)` is unique.
- Read APIs return metadata and redacted status only.
- Mutation requires `secrets.manage`.
- Tenant-scoped operator inspection can see redacted ownership/status only with
  `credentials.inspect` for the active tenant.
- Viewers cannot inspect or mutate.

### 1.2 Secret Version

Version metadata for a tenant secret. The value is stored in the local/test secret value
backend and is never included in API, event, log, fixture, replay, evaluation, diagnostic,
or contract output.

- `secret_version_id` — stable version identifier.
- `secret_id`, `tenant_id`, `secret_ref`.
- `version_number` — monotonically increasing per `(tenant_id, secret_ref)`.
- `status` — `active`, `superseded`, `disabled`, or `pending_remediation`.
- `value_backend_ref` — internal pointer to the local value backend.
- `created_at`, `activated_at`, `superseded_at`.

State transitions:

```text
pending_remediation -> active
active -> superseded       # rotation creates a new active version
active -> disabled
superseded -> disabled     # cleanup only; not reactivated by this roadmap
```

Resolution rule:

- New work resolves the current `active_version_id`.
- Already-started work keeps the version snapshot resolved at start until completion.

### 1.3 Secret Reference

A tenant-scoped handle used by configuration and runtime policy.

- `tenant_id` — resolved from active tenant context; never inferred globally.
- `secret_ref` — user-facing reference string.
- `consumer_kind` — e.g. `integration`, `connector`, `mcp_server`, `mcp_tool`,
  `sandbox_policy`, `skill`.
- `consumer_id` — tenant-scoped resource id.
- `resolution` — `resolved`, `unavailable`, `denied`, or `not_applicable`.
- `redaction_rule` — how to display the reference in operator-visible output.

`secret_scope_bindings` already carries tenant ownership from R35 and gains the runtime
credential semantics above.

### 1.4 Integration Account Binding

Tenant-owned external account binding from Roadmap 27, extended with hosted credential
semantics.

- `tenant_id`.
- `integration_id`.
- `domain_kind`, `display_name`, `environment_scope`.
- `account_key`, `external_account_id`, `account_label` when known.
- `readiness_status`, `auth_state`, `health_state`.
- `provider_auth_state_id` or equivalent redacted relationship.
- `disabled_reason` when disconnect or unsafe bridge disables dependent use.

Rules:

- The same human user can connect the same external account in multiple tenants; each
  binding remains tenant-local.
- Disconnect revokes provider auth, disables dependent connector/MCP uses, and preserves
  redacted configuration for reconnect.

### 1.5 Provider Authorization State

Tenant-owned authorization lifecycle state for external providers.

- `tenant_id`.
- `provider_id`, `family`, `auth_mode`, `status`.
- `account_label`, `account_id`, `plan`, `auth_method` when non-secret.
- `last_checked_at`, `last_authenticated_at`, `last_error`.
- `disabled_reason`, `revoked_at`, `expires_at` where available.
- `metadata_json`, `sandbox_json` redacted.

State transitions:

```text
login_required -> pending_login -> authenticated
authenticated -> expired
authenticated -> revoked
authenticated -> disabled
expired -> pending_login -> authenticated
revoked -> pending_login -> authenticated
```

No runtime path may fall back to global, previous-tenant, or other-tenant provider state.

### 1.6 Connector Configuration

Tenant-owned connector configuration and lifecycle status.

- `tenant_id`.
- `connector_id`, `kind`, `display_name`.
- `status` — existing statuses plus `disabled` for credential disconnect/bridge behavior.
- `secret_refs` or secret binding references, redacted on read.
- `disabled_reason`, `last_failure_reason`, restart/backoff fields.
- `created_at`, `updated_at`.

Rules:

- Mutate/invoke requires tenant permission.
- Tenant-scoped operators can inspect redacted ownership/status only with
  `credentials.inspect` for the active tenant.
- Cross-tenant connector id collisions are allowed when scoped by tenant.

### 1.7 MCP Install And Exposure State

Tenant-owned MCP server, server state, discovered tools, and exposure rules.

- `tenant_id`.
- `server_id`, `tool_name` where applicable.
- install fields from existing MCP resources.
- lifecycle/discovery/exposure statuses.
- redacted `secret_refs`, websocket auth summary, and secret summary.
- disabled state when integration disconnect or unsafe bridge invalidates credentials.

Rules:

- MCP server/tool/exposure uniqueness is tenant-scoped.
- Tool invocation resolves credential state through the active tenant only.
- `mcp_tool_exposure_rules` already carries tenant ownership and gains permission and
  credential enforcement.

### 1.8 Sandbox Policy Or Profile With Secrets

Tenant-owned sandbox policy/profile state that references secrets.

- `tenant_id`.
- consumer identifiers from existing sandbox/policy records.
- `secret_scope` redacted outcomes.
- resolution status per secret reference.

Rules:

- Sandbox preparation fails closed when tenant context is missing, unauthorized, or
  mismatched.
- Secret values are injected only into the prepared execution environment for the active
  tenant and are never persisted in result artifacts.

### 1.9 Credential Audit Event

Tenant-owned audit-visible record persisted through existing tenant audit/event surfaces.

- `audit_event_id`.
- `tenant_id`, `principal_id` where available.
- `event_kind`.
- `resource_kind`, `resource_id` when safe.
- `action`.
- `outcome`, `reason_code`.
- `secret_ref_count` or redacted secret reference summary.
- `created_at`.
- redacted `document_json`.

Successful runtime secret use emits one record per credential-bearing run, connector
invocation, MCP invocation, or sandbox preparation, not per repeated internal resolution.

### 1.10 Disabled Bridged Credential Resource

A credential-bearing resource discovered while bridging local state that cannot safely be
made active.

- original resource kind and id.
- owning default personal tenant.
- redacted metadata.
- `status = pending_remediation` or equivalent disabled state.
- remediation reason.

It cannot invoke credential-bearing behavior until operator remediation succeeds.

## 2. Schema Deltas

Expected additive storage changes:

- Add `tenant_secrets` metadata table.
- Add `tenant_secret_versions` metadata table.
- Add tenant ownership and tenant-aware indexes/uniqueness to R37 Group B tables:
  `provider_auth_states`, `mcp_servers`, `mcp_server_states`, `mcp_tools`, and
  `connectors`.
- Extend or verify tenant-aware uniqueness for `mcp_tool_exposure_rules` and
  `secret_scope_bindings`.
- Add disabled/remediation fields only where an existing domain cannot represent disabled
  dependent or unsafe bridged state in its current document/status shape.

Rollback is backup-restore for storage changes plus disabling the hosted credential APIs
and restoring pre-R37 local credential configuration from the operator backup.

## 3. Redaction Rules

- Raw secret values, OAuth codes, access tokens, refresh tokens, provider tokens, local
  CLI auth material, and derived credential material are forbidden in read responses,
  logs, events, replay fixtures, evaluation artifacts, diagnostics, contract fixtures, and
  test failure output.
- Redacted outputs may include tenant id, resource kind, resource id when safe,
  `secret_ref`, version id, status, resolution, and remediation reason.
- Cross-tenant denials must not reveal target tenant secret values or token material and
  should not require callers to parse raw error text.

## 4. State Transitions

### Secret Rotation

```text
secret active(version N)
  -> rotate
secret active(version N+1), version N superseded
```

Already-started work keeps `version N`; new work resolves `version N+1`.

### Integration Disconnect

```text
provider authenticated
  -> disconnect
provider revoked + dependent connector/MCP use disabled + redacted config preserved
  -> reconnect
provider authenticated + dependent use eligible again
```

### Unsafe Bridge

```text
legacy local credential state
  -> bridge safe
default personal tenant active resource

legacy local credential state
  -> bridge unsafe/ambiguous
default personal tenant disabled bridged resource
  -> operator remediation
active resource
```

## 5. Out-of-Scope Data Model Changes

- No external enterprise secret manager schema.
- No cross-tenant shared service account model.
- No billing usage counters.
- No marketplace distribution model.
- No broad cross-tenant admin projection.
