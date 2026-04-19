# Data Model: Sandbox Requirement Declaration Contract

## Overview

This slice introduces a shared consumer-owned contract model that can cover subprocess
executions, access-only preflight decisions, and declaration-bearing consumer surfaces
without pretending every operation is the same kind of runtime activity.

## Entity: ConsumerRequirementDeclaration

Represents the declared sandbox requirement baseline for one current consumer instance and
operation kind.

| Field | Type | Notes |
|-------|------|-------|
| `declaration_id` | string | Stable identifier for audit and projection |
| `consumer_kind` | enum | `managed_provider`, `skill`, or `local_tool` |
| `consumer_id` | string | Consumer instance identifier such as provider id, skill id, or capability/tool id |
| `operation_kind` | string | Logical operation such as `auth_status`, `prompt_execution`, `skill_selection`, or `tool_call.execute` |
| `profile_id` | string | Required sandbox profile when subprocess execution is allowed |
| `execution_mode` | enum | `subprocess`, `access_only`, or `declaration_only` |
| `allowed_backend_kinds` | []enum | Ordered backend intents; currently resolves only to `subprocess` |
| `filesystem_access` | object | Declared read and write scope |
| `network_access` | object | Declared mode, hosts, ports, and loopback expectations |
| `secret_refs` | []string | Explicit secret identities required by the consumer |
| `approval_mode` | enum | `allow`, `ask`, or `deny` according to declared baseline |
| `required_enforcement_strength` | string | Truthful minimum guarantee required by the consumer |
| `active` | bool | Whether the declaration is currently valid |
| `source` | string | Built-in, config-backed, or consumer-owned declaration source |

Validation rules:

- One active declaration per `(consumer_kind, consumer_id, operation_kind)`.
- `execution_mode=declaration_only` is valid only for consumer surfaces that do not launch a
  process today, such as current skill selection support.
- Any declaration whose `required_enforcement_strength` exceeds current backend capability
  is invalid for execution and must resolve as `unsupported` or `denied`.
- `secret_refs` must be explicit; ambient inheritance alone is not a valid declaration.

## Entity: SecretScopeBinding

Represents the authorization and redaction policy for one secret reference as exposed to a
consumer instance.

| Field | Type | Notes |
|-------|------|-------|
| `binding_id` | string | Stable identifier for auditability |
| `consumer_kind` | enum | `managed_provider`, `skill`, or `local_tool` |
| `consumer_id` | string | Addressed consumer instance |
| `default_source` | enum | `kind_default` or `instance_override` |
| `environment_scope` | enum | `test`, `prod`, or `both` |
| `secret_ref` | string | Declared secret identifier |
| `delivery_kind` | enum | `env_injection`, `config_projection`, `local_state_access`, or other declared delivery form |
| `redaction_rule` | string | Operator-visible redaction behavior |
| `active` | bool | Whether the binding is usable |

Validation rules:

- Authorization is evaluated per consumer instance even when `default_source=kind_default`.
- `environment_scope=test` secrets must never resolve in production, and vice versa.
- Every active binding must define a redaction rule before it can be exposed through any
  operator-visible surface.

## Entity: ConsumerPolicyRecord

Represents one durable policy and provenance record for a meaningful consumer decision or
execution outcome, including paths where no process launched.

| Field | Type | Notes |
|-------|------|-------|
| `policy_record_id` | string | Durable identifier for inspection and replay |
| `consumer_kind` | enum | `managed_provider`, `skill`, or `local_tool` |
| `consumer_id` | string | Requesting consumer instance |
| `operation_kind` | string | Logical operation being evaluated |
| `declaration_id` | string | Effective declaration that applied |
| `requested_by` | string | Actor or daemon surface that initiated the attempt |
| `decision` | enum | `allow`, `ask`, `deny`, or `unsupported` |
| `approval_status` | enum | `not_applicable`, `pending`, `approved`, `rejected` |
| `secret_resolution` | enum | `resolved`, `denied`, `unavailable`, or `not_applicable` |
| `enforcement_strength` | string | Truthful summary of the actual backend guarantee |
| `failure_class` | string | `policy_denied`, `approval_required`, `approval_rejected`, `unsupported_backend_guarantee`, `secret_unavailable`, `process_failed`, `timeout`, `cancelled`, etc. |
| `sandbox_execution_id` | string? | Link to a launched sandbox execution when present |
| `tool_call_id` | string? | Link to runtime tool-call truth when present |
| `provider_operation_id` | string? | Link to managed-provider logical operation when present |
| `started_at` | timestamp | Record creation time |
| `completed_at` | timestamp? | Terminal time |
| `status` | enum | `preflight_allowed`, `approval_pending`, `running`, `completed`, `failed`, `cancelled`, `denied`, `unsupported` |

Validation rules:

- Every meaningful policy evaluation creates exactly one durable `ConsumerPolicyRecord`.
- `sandbox_execution_id` is required only when a subprocess is actually launched.
- Denied, unsupported, and preflight-only records must remain valid with no
  `sandbox_execution_id`.
- If `tool_call_id` or `provider_operation_id` is present, the linked family-specific
  surface must reflect the same terminal outcome and provenance identifiers.

## Relationship Model

- One `ConsumerRequirementDeclaration` governs one current consumer instance and operation
  kind.
- One `ConsumerRequirementDeclaration` can reference zero or more `SecretScopeBinding`
  records.
- One `ConsumerPolicyRecord` resolves exactly one `ConsumerRequirementDeclaration`.
- One `ConsumerPolicyRecord` may link to zero or one sandbox execution, zero or one
  runtime tool call, and zero or one managed-provider logical operation.

## External Surface Mapping

- Managed-provider auth-state and provider-auth events can project declaration and policy
  record metadata for access-only or auth-state-style operations.
- Sandbox execution resources and sandbox lifecycle events can project declaration,
  policy-record, and secret-scope summaries for launched subprocess work.
- Tool-call resources, approval/decision surfaces, and runtime events can project the same
  consumer ids and policy-record linkage for daemon-owned local tools.
- Skill registry, skill detail, and explicit skill-selection surfaces can expose
  declaration-bearing metadata for current skills without claiming bundled-script execution
  already exists.

## State Transitions

### Access-Only Or Declaration-Only Paths

`preflight_allowed`  
`denied`  
`unsupported`

### Approval-Gated Local Tool Paths

`approval_pending` → `running` → `completed`  
`approval_pending` → `running` → `failed`  
`approval_pending` → `denied`  
`approval_pending` → `unsupported`

### Managed Provider Subprocess Paths

`preflight_allowed` → `running` → `completed`  
`preflight_allowed` → `running` → `failed`  
`preflight_allowed` → `running` → `cancelled`  
`preflight_allowed` → `denied`  
`preflight_allowed` → `unsupported`

## Non-Goals For This Slice

- No generic executable-skill subprocess runner
- No MCP server lifecycle or transport record model
- No stronger backend capability implementation beyond the current subprocess backend
- No requirement that every operator-visible consumer surface gain a brand-new top-level
  route
