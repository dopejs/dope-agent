# Quickstart: Agent Profile And Persona Configuration

## Scope

Use this guide to validate the Roadmap 57 implementation in the local test environment.
Do not use production tenants, live connector credentials, or `~/.dope` by default.

## Prerequisites

- Worktree on branch `042-agent-profile-persona`.
- Daemon test environment only: `~/.dope-test`, `127.0.0.1:19192`.
- No live connector accounts required.

## Environment

```bash
make daemon-run-test
make daemon-test-status
```

Expected result:

- The daemon starts against the test environment.
- Schema migration v53 is applied.
- One default profile is available for the default test tenant.
- No production paths or secrets are printed.

## Manual Smoke

1. List profiles for the active test tenant.
   - Expected: request with `profiles.inspect` returns the default profile and current
     tenant-default active selection.
   - Expected: request without `profiles.inspect` returns a stable permission denial
     without profile detail.

2. Create a profile with display identity, persona, provider defaults, safety defaults,
   and one explicit overlay reference.
   - Expected: profile is created with version 1.
   - Expected: overlay reference shows validation state and safe display label.
   - Expected: audit/event evidence is recorded.

3. Update the profile.
   - Expected: a new version is created.
   - Expected: previous version remains inspectable.
   - Expected: unsafe raw profile or overlay content is not exposed in summaries.

4. Activate the profile as the tenant-default active profile.
   - Expected: new work resolves this profile and version.
   - Expected: no channel/workspace/account/capability scoped binding is created.

5. Start representative chat/run/thread work in the test environment.
   - Expected: thread/session/run/workflow/handoff evidence includes active profile projection with
     profile ID, version ID, selection scope `tenant_default`, safe display name, and
     redaction status.
   - Expected: later profile edits do not rewrite this historical projection.

6. Roll back to a prior eligible version.
   - Expected: rollback creates a new active version derived from the prior version.
   - Expected: rollback is denied if current provider, safety, overlay, or policy
     validation fails.

7. Archive or disable a non-default profile.
   - Expected: retired profile remains inspectable with versions and audit evidence.
   - Expected: retired profile cannot be selected for new work.
   - Expected: no hard delete route exists for Phase 57.

8. Restart the daemon and re-check profile state.
   - Expected: profiles, versions, active selection, overlay validation state, audit
     evidence, and runtime profile projections persist.

## Automated Verification

Recorded local verification on 2026-05-12:

- `make daemon-contract-test`: passed.
- `go test ./...` from `daemon/`: passed.
- `pnpm test:clients`: passed.
- `pnpm build`: passed.
- `go mod tidy` from `daemon/`: completed with no `go.mod` or `go.sum` changes.

Run daemon tests:

```bash
cd daemon
go test ./...
go mod tidy
```

Run contract validation:

```bash
make daemon-contract-test
```

Run client tests and build:

```bash
pnpm test:clients
pnpm build
```

Required targeted coverage:

- `daemon/internal/identity`: `profiles.inspect` and `profiles.manage` permission grants
  and denials.
- `daemon/internal/profiles`: validation, versioning, rollback, archive/disable,
  redaction, non-memory scope.
- `daemon/internal/store`: schema v53 migration, CRUD persistence, version retention,
  active selection, runtime projection, restart recovery.
- `daemon/internal/store/tenancy`: tenant-safe profile reads and mutations.
- `daemon/internal/api`: profile routes, permission denials, invalid requests, runtime
  projection in thread/session/run/workflow/handoff evidence.
- `daemon/internal/chat` and runtime startup paths: tenant-default profile resolution at
  work start.
- `daemon/internal/events`: lifecycle/version/runtime projection event schemas.
- `daemon/internal/contracts`: schema and fixture compatibility.
- `sdk/ts/src`: profile client methods and runtime projection types.
- `web/src`: profile editor/history/activation and runtime evidence display.
- `tui/src`: profile and runtime projection inspection.

## Release And Rollback Checks

Before enabling profile edits broadly:

- Confirm every eligible tenant has a safe default profile or a safe fallback resolution.
- Confirm legacy prompt/config behavior is bridged to explicit overlay references or
  marked partial with safe reason codes.
- Confirm profile mutations fail closed when audit/version evidence cannot be written.
- Confirm profile runtime projections never expose unsafe overlay content or secrets.

Rollback procedure:

1. Disable profile create/update/activate/rollback/archive/disable routes or UI actions.
2. Continue serving read-only profile/version/runtime projection evidence to authorized
   users.
3. Preserve profile tables and seeded metadata; do not drop schema v53 data during
   operational rollback.
4. Existing provider, chat, thread, session, run, workflow, handoff, and prompt/config behavior continues
   through the prior compatible paths.

Residual risks:

- Scoped overlay bindings remain intentionally deferred to Roadmap 58 and fail closed via
  explicit validation.
- Legacy prompt/config bridging is represented as partial redacted evidence until each
  legacy source can be explicitly converted into a profile-scoped overlay reference.

## Non-Memory Guardrail

Verification must prove that repeated user preferences in conversation do not mutate
profiles, create learned preferences, trigger memory retrieval, or alter future profile
selection unless an authorized user explicitly changes profile configuration.
