# Data Model: Skill And Local Tool Sandbox Execution

## Overview

This slice extends existing daemon truth instead of introducing a parallel execution
subsystem. The design separates:

- executable-skill declaration and availability state on top of the current skill registry
- runtime tool-call truth for both executable-skill launches and high-risk local tools
- sandbox execution and consumer-policy provenance for all in-scope launches
- additive restart-recovery linkage so interrupted executions end as `cancelled`

## Entity: ExecutableSkillManifest

Represents the executable declaration attached to one skill that can launch local work.

| Field | Type | Notes |
|-------|------|-------|
| `skill_id` | string | Stable skill identifier from the registry |
| `enabled` | bool | Whether the skill is executable in the current environment |
| `entrypoint` | string | Bundled executable or declared command target |
| `args` | []string | Fixed or declarative argument list |
| `working_dir` | string | Optional working directory relative to the skill root or explicit path policy |
| `profile_id` | string | Required sandbox profile |
| `read_roots` | []string | Declared read scope |
| `write_roots` | []string | Declared write scope |
| `network_mode` | enum | `deny`, `allow_list`, or `full` |
| `allowed_hosts` | []string | Explicit host allowlist when relevant |
| `allowed_ports` | []int | Explicit port allowlist when relevant |
| `secret_refs` | []string | Explicit secret identities required by the skill |
| `approval_mode` | enum | Declared approval posture; defaults to `ask` when absent |
| `timeout_ms` | integer | Requested timeout bound |
| `required_enforcement_strength` | string | Truthful minimum guarantee required |
| `availability_status` | enum | `available` or `unavailable` |
| `availability_reason` | string | Operator-visible reason when unavailable |

Validation rules:

- Every executable manifest belongs to exactly one existing `skill_id`.
- Missing approval mode resolves to `ask`.
- Secret refs resolve from the active daemon data dir via `skill-secrets.json`, not shared
  process-env inheritance.
- If the declared guarantee exceeds current backend capability, the manifest is
  `unavailable` for execution.
- Invalid, unsafe, or incomplete manifests remain visible as `unavailable` with a reason.

## Entity: SkillInspectionProjection

Represents the operator-facing skill view after executable-manifest parsing.

| Field | Type | Notes |
|-------|------|-------|
| `skill_id` | string | Stable skill identifier |
| `name` | string | Display name |
| `source` | enum | `home` or `data_dir` |
| `sandbox_declaration` | object | Consumer declaration payload projected from the executable manifest when present |
| `execution_manifest` | object? | Executable manifest projection when the skill is executable |
| `availability_status` | enum | `available`, `unavailable`, or `not_executable` |
| `availability_reason` | string | Explicit reason for `unavailable` |

Validation rules:

- Non-executable skills remain valid with `availability_status=not_executable`.
- Executable skills with invalid manifests remain listable with `availability_status=unavailable`.

## Entity: RuntimeToolCall

Represents one run/step-level execution attempt on the existing tool-call surface.

| Field | Type | Notes |
|-------|------|-------|
| `tool_call_id` | string | Stable runtime identifier |
| `run_id` | string | Owning run |
| `step_id` | string | Owning step |
| `invocation_kind` | enum | `local_tool` or `skill` |
| `capability_id` | string? | Existing capability id for high-risk local tools |
| `skill_id` | string? | Executable skill identifier when `invocation_kind=skill` |
| `tool_name` | string | Operator-facing execution target label |
| `status` | enum | `requested`, `running`, `completed`, `failed`, `cancelled`, `denied` |
| `approval_id` | string? | Linked approval when required |
| `sandbox_execution_id` | string? | Linked sandbox execution when a subprocess launched |
| `policy_record_id` | string? | Linked consumer policy record for preflight or execution truth |
| `failure_class` | string | Classified terminal or blocking result |
| `input` | object | Request payload |
| `output` | object | Result payload |
| `created_at` | timestamp | Creation time |
| `updated_at` | timestamp | Last state change |

Validation rules:

- Exactly one of `capability_id` or `skill_id` is present.
- `sandbox_execution_id` is required only when the request reached subprocess launch.
- Rejected or unsupported requests may still have a `policy_record_id` with no
  `sandbox_execution_id`.
- If daemon restart interrupts a running tool call, terminal recovery state becomes
  `cancelled`.

## Reused Existing Entities With New Phase 19 Projection

This roadmap extends existing shared entities rather than replacing them:

- `ConsumerRequirementDeclaration`
- `ConsumerPolicyRecord`
- `Execution`
- `Approval` / `Decision`

Additional phase-19-specific rules:

- Executable-skill launches use `consumer_kind=skill` and an execution-oriented
  `operation_kind` rather than the current `skill_selection` declaration-only path.
- High-risk local tools continue to use `consumer_kind=local_tool`, but now link to real
  sandbox execution instead of approval truth alone.
- `ConsumerPolicyRecord.tool_call_id` becomes the common runtime linkage for both skill and
  local-tool execution.
- Approval and decision inspection must preserve the applied declaration snapshot carried by
  the persisted consumer view.

## Relationship Model

- One `SkillInspectionProjection` may include zero or one `ExecutableSkillManifest`.
- One `ExecutableSkillManifest` resolves to one execution-oriented
  `ConsumerRequirementDeclaration`.
- One `RuntimeToolCall` links to exactly one execution-oriented consumer declaration.
- One `RuntimeToolCall` may link to zero or one sandbox execution and zero or one
  approval, but always links to zero or one durable `ConsumerPolicyRecord`.
- One sandbox execution may be attached to one runtime tool call for this slice.

## State Transitions

### Executable Skill Availability

`not_executable`  
`available`  
`unavailable`

### Runtime Tool Call

`requested` → `running` → `completed`  
`requested` → `running` → `failed`  
`requested` → `running` → `cancelled`  
`requested` → `denied`  
`requested` → `failed`

### Restart Recovery

`running` → `cancelled`

The restart-recovery transition applies when the daemon restarts before the sandbox launch
finishes and the daemon cannot guarantee continued execution.

## External Surface Mapping

- Skill registry and detail routes project executable manifest and availability truth.
- Existing runtime tool-call routes carry additive invocation-kind, skill-id, approval,
  policy-record, and sandbox-execution linkage.
- Approval and decision surfaces continue to expose pending, approved, or rejected state
  with additive skill/local-tool consumer provenance.
- Sandbox execution resources and events project the same consumer and tool-call linkage
  used by runtime history.

## Non-Goals For This Slice

- No new top-level runtime execution resource beside tool calls
- No broader migration of every capability or arbitrary local tool
- No automatic skill inference or planner/orchestration graph
- No stronger backend capability implementation beyond the current subprocess backend
