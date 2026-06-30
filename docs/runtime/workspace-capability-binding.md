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
that resolves the active profile (`daemon/internal/chat/service.go`), delegating to
`store.ResolveBindingSelection`, which performs every read (default workspace, channel
binding, integration-account binding, profile/workspace availability, capability visibility)
inside a **single transaction**. A concurrent binding or capability-visibility mutation can
therefore never interleave between those reads: each work item records exactly one coherent
selection with no mixed pre/post-change state (FR-033).

Precedence (FR-006): **channel binding → integration-account default → tenant default**. The
originating channel identity (`ChannelScopeRef`) and integration-account identity
(`AccountScopeRef`) are carried on the work item; for connector traffic they are derived from
the inbound message (`daemon/internal/im/loop.go`: connector-qualified channel id and
connector account id). An explicit channel binding always wins over an account default, which
wins over the tenant default.

Outcomes:

- `resolved` — an explicit binding produced a valid selection.
- `default` — no explicit binding; tenant default profile + default workspace apply.
- `repair_required` — the selected profile or workspace is archived/disabled/removed/
  unavailable. The daemon **fails closed**: new work is blocked with safe repair-required
  evidence and is never silently substituted with a different profile, workspace, or
  lower-precedence binding (FR-031). Recovery is an explicit `bindings.manage` repair action.

Capability visibility is resolved strictest-wins across tenant/connector limits and the
profile/workspace policies. A `hidden` or `disabled` capability is neither offered to the
agent nor executable, even on direct request, replay, or stale state (FR-016). Enforcement
happens at two points: at chat work-start over explicitly named skills, and — critically — at
the runtime tool-call execution gate (`enforceRunCapabilityVisibility`, consumed by every
skill/capability tool-call creation path in `daemon/internal/api`). The execution gate uses
the **run's** recorded binding evidence (its resolved profile + workspace) when present, so it
matches the work-start decision including a channel/account binding; absent run evidence (e.g.
a direct API tool-call with no chat work-start) it falls back to the tenant default. A
non-executable decision returns `403` and the capability never runs.

Channel and integration-account bindings do not carry capability-visibility policy in this
phase; any limit they imply is enforced through the resolved profile/workspace policy
(FR-017). The resolver (`bindings.ResolveCapabilityVisibility`) and the store accept a
higher-level tenant/connector `limits` set and apply it strictest-wins, but **no phase-58 data
source populates those limits** — tenant/connector capability policy is not user-editable in
this phase and has no backing table yet, so the integration point is present and correct while
the (a) side of FR-017 is currently a no-op by design (see Residual items).

## Runtime evidence

Each run records durable, append-only **runtime binding evidence**
(`binding_runtime_projections`, `brp_*`) with the selected profile/version, workspace,
binding scope, binding id, a capability visibility summary, and a classification:

- `applied_binding` — an explicit binding influenced the run. This flips the profile
  projection's previously planted `roadmap_58_deferred_binding_unapplied` marker to
  `roadmap_58_applied_binding`.
- `default_binding` — tenant defaults applied.
- `legacy_default` — runs predating binding support.

Evidence is recorded per resource the selection influenced: the thread, and additionally the
runtime **run** when one is linked, so the selection is inspectable from either surface and
the runtime execution gate can resolve it (SC-008). Fail-closed `repair_required` work also
records evidence so blocked work is explainable (FR-031). Binding changes never rewrite prior
evidence (FR-012). Operators inspect a thread or a run to see the additive `bindingProjection`
field (requires `bindings.inspect`).

## Permissions

- `bindings.inspect` — read workspaces, bindings, capability visibility, and runtime binding
  evidence. Unauthorized reads are denied without revealing inaccessible binding existence.
- `bindings.manage` — create/update/disable/remove/repair bindings, create/archive/disable
  workspaces, and change capability visibility.

Both are granted to owner and admin by default. Mutations fail closed if their audit/event
evidence cannot be recorded (FR-011).

## Product surfaces

- API: `/v1/workspaces`, `/v1/workspaces/{id}`, `/v1/bindings`, `/v1/bindings/{id}`,
  `/v1/bindings/{id}/repair`, `/v1/capability-visibility`. Thread detail and run detail each
  carry an additive `bindingProjection`.
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

## Residual items / known limitations

These are intentional, documented limits of the current implementation:

- **Tenant/connector capability limits (FR-017 side a)** — the visibility resolver and
  `store.EffectiveCapabilityVisibility` accept a higher-level `limits` set and apply it
  strictest-wins, but no phase-58 data model/table populates it. Tenant/connector capability
  policy is not user-editable in this phase, so the limit set is empty and imposes no
  restriction beyond the profile/workspace policies. Adding the data source is future work; no
  code change to the resolver is needed to honor it once a source exists.
- **Lifecycle event persistence (FR-011)** — binding mutations write their authoritative
  **audit** row in the *same transaction* as the state change and fail closed if it cannot be
  recorded (verified by test). The mirrored `events` row and bus fan-out are published
  *after* commit, consistent with the daemon-wide event convention; a post-commit event-write
  failure surfaces an error to the caller but the audit row remains the durable record of the
  change. Folding the event row into the mutation transaction is deferred to avoid diverging
  the shared event path.
- **Capability-visibility execution gate coverage** — enforced at chat work-start and at the
  skill/capability runtime tool-call paths. MCP-tool, computer-use, and integration tool-call
  paths are not yet gated on binding visibility (they have separate authorization), and a
  direct API tool-call with no run evidence is enforced at tenant-default scope.

## Verification

Go: `daemon/internal/{bindings,store,store/tenancy,api,chat,events,identity,contracts}`.
Contracts/schemas: `make daemon-contract-test`. Clients: `pnpm test:clients`. Default local
verification uses `~/.dope-test` on `127.0.0.1:19192`.
