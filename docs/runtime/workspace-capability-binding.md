# Workspace And Capability Binding (Roadmap 58)

Status: implemented. This document describes the runtime and operator behavior of
tenant-owned workspace and capability bindings. The authoritative upstream spec is
`docs/specs/043-workspace-and-capability-binding.md`; the feature spec, plan, and tasks
live under `specs/043-workspace-capability-binding/`.

## What bindings are

Bindings are explicit, tenant-owned product configuration — not memory, not learned
preferences, not hidden prompt truth. They connect a binding scope (a tenant-owned channel
or an integration account) to a selected agent profile and/or workspace, and they control
capability visibility at profile and workspace scope.

- **Workspace**: a persisted tenant-owned record (`ws_*`) used for binding identity, safe
  display, status, audit, and repair. One default personal workspace is provisioned lazily
  and idempotently on first access. A workspace grants no filesystem, repository, document,
  connector, or knowledge access by itself.
- **Binding rule** (`bnd_*`): `channel` or `integration_account` scope → selected profile
  and/or workspace. Channel bindings may carry a workspace; integration-account defaults
  supply a profile default only.
- **Capability visibility policy** (`cvp_*`): per-capability `visible` / `hidden` /
  `disabled` / `default_enabled` at `profile` or `workspace` scope.

## Resolution at work-start

When new work starts, the daemon resolves the effective binding selection in the same pass
that resolves the active profile (`daemon/internal/chat/service.go`).

Precedence (FR-006): **channel binding → integration-account default → tenant default**.

Outcomes:

- `resolved` — an explicit binding produced a valid selection.
- `default` — no explicit binding; tenant default profile + default workspace apply.
- `repair_required` — the selected profile or workspace is archived/disabled/removed/
  unavailable. The daemon **fails closed**: new work is blocked with safe repair-required
  evidence and is never silently substituted with a different profile, workspace, or
  lower-precedence binding (FR-031). Recovery is an explicit `bindings.manage` repair action.

Capability visibility is resolved strictest-wins across tenant/connector limits and the
profile/workspace policies. A `hidden` or `disabled` capability is neither offered to the
agent nor executable, even on direct request, replay, or stale state (FR-016). Channel and
integration-account bindings do not carry capability-visibility policy in this phase; any
limit they imply is enforced through the resolved profile/workspace and tenant/connector
policy (FR-017).

## Runtime evidence

Each run records durable, append-only **runtime binding evidence**
(`binding_runtime_projections`, `brp_*`) with the selected profile/version, workspace,
binding scope, binding id, a capability visibility summary, and a classification:

- `applied_binding` — an explicit binding influenced the run. This flips the profile
  projection's previously planted `roadmap_58_deferred_binding_unapplied` marker to
  `roadmap_58_applied_binding`.
- `default_binding` — tenant defaults applied.
- `legacy_default` — runs predating binding support.

Binding changes never rewrite prior evidence (FR-012). Operators inspect a thread to see the
`bindingProjection` field (requires `bindings.inspect`).

## Permissions

- `bindings.inspect` — read workspaces, bindings, capability visibility, and runtime binding
  evidence. Unauthorized reads are denied without revealing inaccessible binding existence.
- `bindings.manage` — create/update/disable/remove/repair bindings, create/archive/disable
  workspaces, and change capability visibility.

Both are granted to owner and admin by default. Mutations fail closed if their audit/event
evidence cannot be recorded (FR-011).

## Product surfaces

- API: `/v1/workspaces`, `/v1/workspaces/{id}`, `/v1/bindings`, `/v1/bindings/{id}`,
  `/v1/bindings/{id}/repair`, `/v1/capability-visibility`. Thread detail carries an additive
  `bindingProjection`.
- SDK (`@dope/client`): `listWorkspaces`, `createWorkspace`, `updateWorkspace`,
  `listBindings`, `createBinding`, `updateBinding`, `removeBinding`, `repairBinding`,
  `listCapabilityVisibility`, `setCapabilityVisibility`.
- Web: `web/src/features/workspace-capability-bindings/`.
- TUI: `dope-chat --bindings`, `dope-chat --workspaces`.

Older clients that do not understand binding fields keep working: new work without an
explicit binding resolves to the default profile + default workspace, and new response
fields are additive/optional (FR-024).

## Explicitly out of scope (Roadmap 58)

This phase does **not** create memory-backed workspace knowledge, per-tenant physical
workspace storage migration, a plugin/community marketplace, autonomous capability selection
beyond policy-visible capabilities, or filesystem access from workspace binding alone
(FR-021, FR-029, SC-015). Binding state is identity and policy only.

## Migration and rollback

- Additive schema migration **v54** (`r58_workspace_capability_binding`). No destructive
  rewrite of existing evidence.
- Default workspace is provisioned lazily; there is no bulk tenant migration.
- Rollback disables binding/workspace/visibility mutations while preserving recorded binding
  state, audit, repair status, and runtime evidence for inspection; new work falls back to
  tenant-default behavior.

## Verification

Go: `daemon/internal/{bindings,store,store/tenancy,api,chat,events,identity,contracts}`.
Contracts/schemas: `make daemon-contract-test`. Clients: `pnpm test:clients`. Default local
verification uses `~/.dope-test` on `127.0.0.1:19192`.
