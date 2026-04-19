# Data Model: Sandbox Managed Provider Convergence

## Overview

This slice does not require a brand-new public resource family, but it does require a clear
logical model so provider-owned local state can be governed by sandbox requirements even
when no subprocess is launched.

## Entity: ManagedProviderRequirementDeclaration

Represents the declared sandbox requirement baseline for one managed-provider action.

| Field | Type | Notes |
|-------|------|-------|
| `provider_id` | string | `claude_managed` or `codex_managed` |
| `action_kind` | enum | `auth_status`, `logout`, `prompt_execution` |
| `profile_id` | string | Built-in sandbox profile or provider-specific profile id |
| `backend_kind` | enum | Fixed to `subprocess` in this slice |
| `read_roots` | []string | Declared local roots required by the action |
| `write_roots` | []string | Declared write roots required by the action |
| `network_mode` | enum | `deny`, `allow_list`, or `full` according to the action baseline |
| `allowed_hosts` | []string | Optional host declaration |
| `allowed_ports` | []int | Optional port declaration |
| `approval_mode` | enum | Baseline `allow` for declared in-scope access in this slice |
| `sensitive_state_classes` | []string | Redacted classes such as auth file, settings file, model cache, config file |
| `enforcement_strength` | string | Must describe actual backend guarantee, e.g. `declared_only` for subprocess network controls |
| `active` | bool | Whether the declaration is currently usable |

Validation rules:

- One active declaration per `(provider_id, action_kind)`.
- Declared roots must remain a subset of the provider profile's allowed roots.
- `backend_kind` must remain `subprocess` for this slice.
- Sensitive state classes must always carry a redaction rule in operator-visible output.

## Entity: ManagedProviderOperation

Represents one logical managed-provider attempt, regardless of whether it launches a CLI
process.

| Field | Type | Notes |
|-------|------|-------|
| `operation_id` | string | Stable identifier for operator-visible provenance |
| `provider_id` | string | Managed provider family |
| `action_kind` | enum | `auth_status`, `logout`, `prompt_execution` |
| `requested_by` | string | Consumer attribution such as daemon-managed provider API path |
| `requirement_profile_id` | string | Effective sandbox profile used for policy evaluation |
| `decision` | enum | `allow`, `deny`, or `ask` (although `ask` is not expected for declared baseline access here) |
| `approval_status` | enum | `not_applicable`, `pending`, `approved`, `rejected` |
| `failure_class` | string | `policy_denied`, `missing_local_state`, `provider_auth_failed`, `process_failed`, `timeout`, `cancelled`, etc. |
| `enforcement_strength` | string | Truthful summary of actual backend enforcement |
| `sensitive_state_classes` | []string | Redacted classes involved in the operation |
| `execution_id` | string? | Optional link to sandbox execution when a subprocess is launched |
| `started_at` | timestamp | Operation start |
| `completed_at` | timestamp? | Terminal timestamp |
| `status` | enum | `pending`, `denied`, `local_state_inspection`, `running`, `completed`, `failed`, `cancelled` |

Validation rules:

- Every in-scope managed-provider workflow must create exactly one logical operation.
- `execution_id` is optional because auth-state inspection may not launch a CLI process.
- If the operation needs access outside the declaration, it must transition to `denied`
  with `failure_class=policy_denied`.

## Entity: SensitiveLocalStateAccessSummary

Represents the audit summary for provider-owned local state touched during a managed
provider operation.

| Field | Type | Notes |
|-------|------|-------|
| `provider_id` | string | Provider family |
| `action_kind` | enum | Logical operation that touched the state |
| `state_class` | string | e.g. `auth_file`, `settings_file`, `models_cache`, `config_file`, `temp_output` |
| `access_mode` | enum | `read` or `write` |
| `path_summary` | string | Redacted or class-based summary; never a raw sensitive path/value dump |
| `declared` | bool | Whether the access was declared by the requirement baseline |
| `sensitive` | bool | Must be true for credential-bearing state |
| `redaction_rule` | string | How the state is hidden from operator-visible surfaces |

Validation rules:

- Sensitive entries must never expose secret values or raw credential material.
- Any undeclared access summary is terminally invalid for in-scope workflows and must map
  to a fail-closed denial.

## Relationship Model

- One `ManagedProviderRequirementDeclaration` covers one `(provider_id, action_kind)` pair.
- One `ManagedProviderOperation` resolves exactly one declaration.
- One `ManagedProviderOperation` can reference zero or one `sandbox.Execution`.
- One `ManagedProviderOperation` can include zero or more
  `SensitiveLocalStateAccessSummary` records.

## External Surface Mapping

This plan keeps the external contract minimal by mapping the logical model onto existing
surfaces:

- Auth-state-oriented workflows can project their latest operation summary through
  `providers.AuthState.Metadata`.
- Subprocess-backed workflows can project operation provenance through
  `sandbox.Execution.Metadata` and `sandbox.Result.BackendMetadata`.
- Event payloads must carry the same logical identifiers and failure classification needed
  for operator debugging.

## State Transitions

### Auth Status

`pending` → `local_state_inspection` → `completed`  
`pending` → `denied`  
`pending` → `local_state_inspection` → `failed`

### Logout / Prompt Execution

`pending` → `running` → `completed`  
`pending` → `running` → `failed`  
`pending` → `running` → `cancelled`  
`pending` → `denied`

## Non-Goals For This Slice

- No generic secret-ref entity
- No Docker/SSH/remote backend entity expansion
- No requirement to persist every file read or write as a separate sandbox execution row
