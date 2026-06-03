# Quickstart: Workspace And Capability Binding (Roadmap 58)

Test-environment workflow for implementing and verifying Phase 58. Local work defaults to
`~/.dope-test` and `127.0.0.1:19192`; live connectors and prod tenants are not required.

## Prerequisites

```bash
make daemon-build
make daemon-run-test          # starts test daemon on :19192
make daemon-test-status       # health check
```

## Implementation order (matches staged rollout in plan.md)

1. **Permissions** — add `bindings.inspect` / `bindings.manage` to
   `daemon/internal/identity/types.go` (+ `AllSensitivePermissions`, role grants),
   `permissions_test.go`, and `schemas/api/tenant-permission-resource.schema.json`.
2. **Store + migration** — bump `CurrentSchemaVersion` 53 → 54 and add
   `r58_workspace_capability_binding` in `daemon/internal/store/store.go`; create
   `workspace_store.go` (with `EnsureDefaultWorkspace`), `binding_store.go`,
   `binding_projection.go` + tests.
3. **Bindings domain** — `daemon/internal/bindings/`: `binding.go`, `precedence.go`,
   `visibility.go`, `policy.go` (fail-closed), `projection.go`, `redaction.go` + tests.
4. **Tenancy guard** — `daemon/internal/store/tenancy/bindings.go` (`BindingAccessScope`).
5. **Events/audit** — `daemon/internal/events/workspace_capability_bindings.go` + tests.
6. **API** — `daemon/internal/api/workspace_bindings.go`, register routes in `server.go`,
   add additive `bindingProjection` to thread/run detail.
7. **Work-start resolution** — in `daemon/internal/chat/service.go`, resolve binding selection
   alongside `resolveActiveProfile` and record runtime binding evidence alongside
   `recordActiveProfileProjection`; **fail closed** on `repair_required`.
8. **Deferral flip** — in `daemon/internal/profiles/projection.go`, set the applied
   classification when an explicit binding influenced the run; update contract fixtures that
   reference `roadmap_58_deferred_binding_unapplied`.
9. **Schemas/contracts** — add `schemas/api/*` + `schemas/events/*`; add
   `daemon/internal/contracts/workspace_capability_binding_contracts_test.go`.
10. **Clients** — SDK types/methods (`sdk/ts/src/index.ts`), web feature
    `web/src/features/workspace-capability-bindings/`, TUI commands (`tui/src/cli.ts`).
11. **Docs** — `docs/runtime/workspace-capability-binding.md` (+ cross-links).

## Smoke walkthrough (manual, test env)

```bash
# 1. Lazy default workspace appears on first list
curl -s :19192/v1/workspaces -H "$AUTH" | jq '.workspaces[] | {workspaceId, isDefault, status}'

# 2. Bind a channel to a profile + workspace
curl -s -X POST :19192/v1/bindings -H "$AUTH" -d '{
  "scopeKind":"channel","scopeRef":"<channel-id>",
  "selectedProfileId":"<profile-id>","selectedWorkspaceId":"<workspace-id>"}'

# 3. Start new work from that channel, then inspect runtime evidence
curl -s :19192/v1/threads/<thread-id> -H "$AUTH" | jq '.bindingProjection
  | {bindingScope, selectedProfileId, selectedWorkspaceId, classification, capabilityVisibilitySummary}'
# expect classification == "applied_binding"

# 4. Hide a capability for the workspace; confirm it is not offered and cannot execute
curl -s -X PUT :19192/v1/capability-visibility -H "$AUTH" -d '{
  "scopeKind":"workspace","scopeRef":"<workspace-id>","capabilityId":"<cap>","visibility":"hidden"}'

# 5. Point a binding at an archived profile; confirm fail-closed repair-required (not silent fallback)
```

## Verification

```bash
# Daemon domain + store + api + chat + events
cd daemon && go test ./internal/bindings/... ./internal/store/... ./internal/store/tenancy/... \
  ./internal/api/... ./internal/chat/... ./internal/events/... ./internal/identity/...

# Contracts / schema fixtures (includes applied-binding classification)
make daemon-contract-test

# Full daemon + tidy
cd daemon && go test ./... && go mod tidy

# Clients
pnpm build && pnpm test:clients
```

## Acceptance gates (map to spec Success Criteria)

- SC-001/02 permissions + precedence; SC-003 workspace grants no FS access;
  SC-004/05 capability visibility + denial; SC-006/07 audit + fail-closed;
  SC-008/09 runtime + historical evidence; SC-010 restart recovery; SC-011 repair;
  SC-012 operator can explain a capability decision in ≤5 min; SC-013 compatibility;
  SC-014 redaction zero-exposure; SC-015 non-memory/non-filesystem.

## Failure modes & rollback

- **Audit/event write fails** → mutation fails closed, state unchanged (FR-011); verify via
  injected audit failure in `api/workspace_bindings_test.go`.
- **Invalid binding selection at work-start** → fail closed with safe repair-required evidence;
  no silent substitution (FR-031).
- **Rollback** → disable binding/workspace/visibility mutations; new work uses tenant-default
  profile + default workspace + default visibility; recorded state/audit/evidence preserved.

## Out of scope (do not implement in Phase 58)

Memory-backed workspace knowledge, per-tenant physical workspace storage migration, community
marketplace, autonomous capability selection beyond policy-visible capabilities, and any
filesystem grant from workspace binding alone (FR-021/29, SC-015).
