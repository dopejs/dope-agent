# Data Model: Sandbox Stronger Backends

## Implemented Projection Notes

- `BackendCapabilityProfile` is projected through `config.sandbox.backends` and
  `sandbox.profile.backendCapability`.
- `BackendSelectionRule` is projected through sandbox explain, sandbox executions, and
  sandbox decision events.
- Executable skill inspection now carries both `profile_id` and derived `backend_kind`.

## Overview

This slice extends the current sandbox control plane rather than replacing it. The design
adds:

- explicit backend capability truth on top of the existing profile model
- deterministic backend selection and unsupported semantics
- opt-in `docker` requirement support for executable skills
- migration-planning artifacts that identify what remains after the first stronger-backend
  rollout

## Entity: BackendCapabilityProfile

Represents the operator-visible capability and prerequisite summary for one sandbox
backend.

| Field | Type | Notes |
|-------|------|-------|
| `backend_kind` | enum | `subprocess` or `docker` |
| `display_name` | string | Operator-facing backend label |
| `filesystem_enforcement` | enum | Strength classification for filesystem isolation |
| `network_enforcement` | enum | Strength classification for network isolation |
| `env_injection_mode` | enum | Whether env injection is declarative or hard-bounded |
| `approval_behavior` | string | Summary of approval interaction |
| `restart_behavior` | string | Summary of recovery semantics |
| `host_prerequisites` | []string | Required host capabilities |
| `availability_status` | enum | `available`, `unavailable`, or `degraded` |
| `availability_reason` | string | Operator-visible reason when not available |

Validation rules:

- Every supported backend has exactly one capability profile.
- Strength claims must be truthful and not overstate hard isolation.
- `availability_status` reflects current host capability, not just static configuration.

## Entity: BackendSelectionRule

Represents the selection result for one execution request.

| Field | Type | Notes |
|-------|------|-------|
| `requested_backend_kind` | enum? | Backend explicitly selected or required by the consumer |
| `effective_backend_kind` | enum? | Backend actually chosen when supported |
| `selection_mode` | enum | `default`, `explicit`, `required` |
| `selection_outcome` | enum | `selected`, `unsupported`, `denied` |
| `mismatch_reason` | string | Reason no available backend can satisfy the request |
| `host_status` | enum | `ready`, `missing_prerequisite`, `runtime_unavailable` |

Validation rules:

- `docker`-required requests never degrade to `subprocess`.
- If `selection_outcome=unsupported`, `effective_backend_kind` is absent.
- Selection reasons remain operator-visible through explain and execution surfaces.

## Entity: ExecutableSkillBackendRequirement

Represents the stronger-backend selection state for one executable skill.

| Field | Type | Notes |
|-------|------|-------|
| `skill_id` | string | Stable skill identifier |
| `profile_id` | string | Sandbox profile referenced by the executable skill |
| `required_backend_kind` | enum? | `docker` when explicitly required |
| `selection_mode` | enum | `baseline` or `stronger_required` |
| `migration_status` | enum | `baseline`, `candidate`, `docker_enabled`, `deferred` |

Validation rules:

- Unmodified executable skills remain `baseline`.
- `docker_enabled` requires explicit declaration rather than implicit heuristics.
- `migration_status` must align with the committed migration inventory.

## Entity: StrongerBackendExecutionRecord

Represents one sandbox execution launched on `docker`.

| Field | Type | Notes |
|-------|------|-------|
| `execution_id` | string | Stable sandbox execution identifier |
| `backend_kind` | enum | `docker` for this slice |
| `profile_id` | string | Applied sandbox profile |
| `consumer_kind` | enum | Initial rollout focuses on `skill` |
| `consumer_id` | string | Executable skill identifier |
| `tool_call_id` | string? | Runtime linkage when launched through tool calls |
| `status` | enum | `pending`, `running`, `completed`, `failed`, `cancelled`, `denied`, `unsupported` |
| `failure_class` | string | Classified mismatch, launch, runtime, timeout, or recovery failure |
| `host_prerequisite_snapshot` | object | Availability facts relevant at launch time |

Validation rules:

- `backend_kind=docker` must remain explicit in operator-visible execution truth.
- `unsupported` is terminal and indicates backend/host mismatch before successful launch.
- Restart recovery keeps the same visibility guarantees as current sandbox executions.

## Entity: MigrationCandidateRecord

Represents the durable inventory entry for one local consumer family or specific consumer.

| Field | Type | Notes |
|-------|------|-------|
| `consumer_family` | enum | `skill`, `local_tool`, `mcp_server`, `managed_provider`, or other future local family |
| `consumer_id` | string? | Optional specific consumer identifier |
| `current_backend_posture` | string | Current execution boundary and backend posture |
| `roadmap20_status` | enum | `in_scope`, `candidate`, `deferred`, `out_of_scope` |
| `migration_reason` | string | Why it should or should not move to stronger isolation now |
| `host_prerequisites` | []string | Prerequisites that block migration |
| `next_action` | string | Planned follow-on step |

Validation rules:

- The inventory covers all currently known sandbox consumer families.
- Every deferred family has an explicit reason and next action.
- The first in-scope `docker` verification target is executable skills.

## Relationship Model

- One `BackendCapabilityProfile` exists per backend kind.
- One `ExecutableSkillBackendRequirement` references one backend selection posture.
- One execution request resolves through one `BackendSelectionRule`.
- One `StrongerBackendExecutionRecord` may link to one runtime tool call and one consumer.
- One `MigrationCandidateRecord` may describe a whole family or one named consumer target.

## State Transitions

### Backend Availability

`available`  
`unavailable`  
`degraded`

### Backend Selection

`selected`  
`unsupported`  
`denied`

### Migration Status

`baseline` → `candidate` → `docker_enabled`  
`baseline` → `deferred`

### Stronger-Backend Execution

`pending` → `running` → `completed`  
`pending` → `running` → `failed`  
`pending` → `running` → `cancelled`  
`pending` → `denied`  
`pending` → `unsupported`

## External Surface Mapping

- Sandbox profile list/detail surfaces project backend capability and prerequisite truth.
- Sandbox explain surfaces project selection outcome and mismatch classification.
- Executable-skill inspection surfaces project whether a skill remains baseline or
  explicitly requires `docker`.
- Runtime tool-call and sandbox execution resources preserve backend identity and
  unsupported semantics for executable-skill launches.
- Operator docs and migration artifacts project the full inventory of in-scope and deferred
  consumer families.

## Non-Goals For This Slice

- No automatic migration of all executable skills to `docker`
- No migration of all high-risk local tools, MCP servers, or managed providers
- No SSH, remote managed backend, or VM-grade backend in this roadmap
- No second execution control plane outside the current sandbox-owned contract
