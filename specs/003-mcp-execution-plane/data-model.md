# Data Model: MCP Execution Plane

## Overview

This slice introduces a daemon-managed MCP subsystem on top of the existing sandbox
declaration, secret-scope, and policy-record foundation. The new model separates:

- persisted MCP server registry truth
- runtime lifecycle state and restart behavior
- discovered tool inventory plus exposure policy
- additive linkage back to sandbox executions, secret-scope bindings, and policy records

## Entity: MCPServer

Represents one daemon-managed MCP server definition.

| Field | Type | Notes |
|-------|------|-------|
| `server_id` | string | Stable operator-facing identifier |
| `display_name` | string | Human-readable label |
| `source` | enum | `api`, `config`, or `builtin` |
| `enabled` | bool | Whether the daemon should manage lifecycle for this server |
| `sandbox_profile_id` | string | Required sandbox profile |
| `declaration_id` | string | Effective consumer declaration used for lifecycle starts |
| `transport_kind` | enum | `stdio` for this slice |
| `command` | string | Launch entrypoint |
| `args` | []string | Launch arguments |
| `working_dir` | string | Optional working directory |
| `secret_refs` | []string | Declared credential identities |
| `auto_restart` | bool | Enabled servers restart automatically after daemon restart |
| `created_at` | timestamp | Creation time |
| `updated_at` | timestamp | Last config mutation time |

Validation rules:

- `server_id` must be unique.
- Only `transport_kind=stdio` is valid in this slice.
- `enabled=true` requires an active sandbox profile and declaration.
- `auto_restart=true` is valid only when `enabled=true`.
- `secret_refs` must be explicit; ambient env inheritance is invalid.

## Entity: MCPServerState

Represents the current daemon-visible runtime and health state for one MCP server.

| Field | Type | Notes |
|-------|------|-------|
| `server_id` | string | Owning MCP server |
| `status` | enum | `disabled`, `stopped`, `starting`, `healthy`, `degraded`, `backing_off`, `failed`, `stopping`, `denied`, `unsupported` |
| `health_reason` | string | Latest operator-visible reason for degraded or blocked state |
| `failure_count` | integer | Restart/backoff counter |
| `restart_count` | integer | Successful restart attempts |
| `last_started_at` | timestamp? | Last sandbox-backed start |
| `last_stopped_at` | timestamp? | Last stop completion |
| `last_heartbeat_at` | timestamp? | Last confirmed healthy transport activity |
| `next_restart_at` | timestamp? | Backoff target when restarting later |
| `last_execution_id` | string? | Linked sandbox execution when present |
| `last_policy_record_id` | string? | Linked consumer policy record for latest decision |
| `updated_at` | timestamp | Latest state transition |

Validation rules:

- `status=healthy` requires a successfully started server and current health evidence.
- `status=denied` or `unsupported` must include `health_reason`.
- `next_restart_at` is required only for `backing_off`.
- `last_execution_id` is optional because a restart attempt may be blocked before launch.

## Entity: MCPToolCatalogEntry

Represents one tool reported by an MCP server and tracked by the daemon.

| Field | Type | Notes |
|-------|------|-------|
| `server_id` | string | Owning MCP server |
| `tool_name` | string | Canonical tool identifier from the server |
| `title` | string | Human-readable display name |
| `description` | string | Operator-visible summary |
| `schema_fingerprint` | string | Stable fingerprint of the tool contract for change detection |
| `discovery_status` | enum | `discovered`, `stale`, `unavailable` |
| `last_discovered_at` | timestamp? | Last successful catalog sync |
| `updated_at` | timestamp | Last metadata refresh |

Validation rules:

- `(server_id, tool_name)` must be unique.
- `discovery_status=discovered` requires `last_discovered_at`.
- A tool may remain cataloged while `discovery_status=stale` so policy can stay inspectable
  even when the server is currently unavailable.

## Entity: MCPToolExposureRule

Represents operator-visible availability policy for one MCP tool on one runtime surface.

| Field | Type | Notes |
|-------|------|-------|
| `server_id` | string | Owning MCP server |
| `tool_name` | string | Tool being governed |
| `runtime_surface` | enum | Current daemon/runtime surface identifier |
| `exposure_mode` | enum | `blocked`, `allow`, `approval_required` |
| `active` | bool | Whether this rule is in force |
| `reason` | string | Latest operator-visible explanation |
| `updated_at` | timestamp | Last policy mutation |

Validation rules:

- `(server_id, tool_name, runtime_surface)` must be unique.
- Tools without an active rule for a runtime surface remain unexposed by default.
- `exposure_mode=allow` or `approval_required` is effective only while the owning server is
  enabled, policy-allowed, and healthy.

## Reused Existing Entities With New MCP Projection

The previous roadmap already introduced these shared entities. This slice reuses them by
adding `consumer_kind=mcp_server` support:

- `ConsumerRequirementDeclaration`
- `SecretScopeBinding`
- `ConsumerPolicyRecord`

Additional MCP-specific validation:

- Secret scope is authorized per `server_id`, not per profile.
- Tool-level approval records link back to the MCP server and tool exposure rule that
  required approval.
- Lifecycle policy records remain valid when no subprocess launches because restart was
  denied or unsupported.

## Relationship Model

- One `MCPServer` has exactly one current `MCPServerState`.
- One `MCPServer` resolves to one active `ConsumerRequirementDeclaration` for lifecycle
  operations.
- One `MCPServer` references zero or more `SecretScopeBinding` records.
- One `MCPServer` has zero or more `MCPToolCatalogEntry` records.
- One `MCPToolCatalogEntry` has zero or more `MCPToolExposureRule` records, one per
  runtime surface.
- One lifecycle transition may link to zero or one sandbox execution and zero or one
  consumer policy record.

## State Transitions

### MCP Server Lifecycle

`disabled` → `stopped`  
`stopped` → `starting` → `healthy`  
`starting` → `degraded`  
`starting` → `backing_off`  
`starting` → `denied`  
`starting` → `unsupported`  
`healthy` → `degraded`  
`healthy` → `stopping` → `stopped`  
`degraded` → `backing_off`  
`backing_off` → `starting`  
`backing_off` → `failed`

### Tool Exposure Effectiveness

`blocked`  
`allow`  
`approval_required`

Effective exposure is the intersection of:

1. active exposure rule
2. owning server `enabled=true`
3. server state healthy enough to serve requests
4. current policy and secret resolution still valid

## Non-Goals For This Slice

- No multi-backend placement beyond the current subprocess backend
- No generic planner/orchestration model for arbitrary tool invocation
- No server-wide "allow everything" exposure default
- No credential sharing based only on sandbox profile or server kind
