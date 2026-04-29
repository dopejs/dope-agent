# Quickstart: Live Validation And Side-Effect Replay

## Default Environment

Use the isolated test environment by default:

```bash
make daemon-run-test
make daemon-test-status
```

Do not use `~/.dope`, production tenants, or live connectors unless running the explicit
opt-in real-account smoke path.

## Planning Artifact Checks

Before `/speckit.tasks`, review:

```bash
sed -n '1,220p' specs/025-live-validation-replay/contracts/replay-support-matrix.md
sed -n '1,220p' specs/025-live-validation-replay/contracts/live-validation-surfaces.md
sed -n '1,220p' specs/025-live-validation-replay/contracts/side-effect-ledger.md
```

Required planning gates:

- every reachable tool-call class has a support matrix row or explicit unsupported row,
- missing rows are treated as unsupported,
- live validation start path uses permission, quota, kill-switch, support, and approval
  gates,
- side-effect ledger outcomes and reconciliation states have schema/contract coverage,
- fake-backend tests cover all supported side-effect paths.

## Targeted Verification During Implementation

Run package-level tests as the implementation lands:

```bash
cd daemon
go test ./internal/livevalidation ./internal/evaluation ./internal/api ./internal/store ./internal/identity ./internal/billing
go test ./internal/integrations ./internal/calendar ./internal/mail ./internal/delivery ./internal/connectors ./internal/mcp
go test ./internal/audit ./internal/events ./internal/contracts
```

Required targeted scenarios:

- permission denial for missing `live_validation.execute`,
- quota denial and hosted fail-closed unavailable state,
- tenant/global kill-switch denial before start,
- kill switch during running attempt aborts pending/future side effects,
- approval granularity for scope-level and per-action approvals,
- mixed supported/unsupported candidate explicit exclusion,
- every ledger outcome: attempted, skipped, completed, failed, aborted, denied,
  operator-action-needed,
- timeout-after-submit,
- daemon restart after submit,
- duplicate retry,
- reminder lifecycle fake-backend side effects,
- delivery dispatch and connector message-send fake-backend side effects,
- ambiguous commit and reconciliation,
- unauthorized reconciliation denial,
- retention default indefinite,
- matrix completeness.

## Contract And Client Verification

Run contract and client checks whenever API, event, schema, SDK, web, or TUI surfaces
change:

```bash
make daemon-contract-test
pnpm test:clients
pnpm build
```

Expected contract coverage:

- live validation attempt resource,
- side-effect ledger resource and list response,
- replay support matrix resource,
- kill-switch resource,
- reconciliation resolution resource,
- live validation comparison resource,
- stable denial/error payloads,
- event schemas for live validation and side-effect ledger outcomes.

## Full Daemon Verification

After the implementation is complete:

```bash
cd daemon
go test ./...
go mod tidy
cd ..
make daemon-contract-test
make daemon-run-test
make daemon-test-status
pnpm test:clients
pnpm build
```

Manual smoke in `DOPE_ENV=test` must demonstrate from structured evidence:

- one successful supported side-effect replay,
- one denied live validation request,
- one unsupported tool class,
- one ambiguous-commit reconciliation path,
- one kill-switch abort of pending/future side effects,
- live validation history and ledger inspection after daemon restart.

Useful inspection routes during the smoke:

```bash
curl -H "Authorization: Bearer $DOPE_TOKEN" -H "X-Dope-Tenant-ID: $DOPE_TENANT_ID" \
  http://127.0.0.1:19192/v1/live-validations/$VALIDATION_ID/ledger

curl -X POST -H "Authorization: Bearer $DOPE_TOKEN" -H "X-Dope-Tenant-ID: $DOPE_TENANT_ID" \
  http://127.0.0.1:19192/v1/live-validations/$VALIDATION_ID/compare

curl -X POST -H "Authorization: Bearer $DOPE_TOKEN" -H "X-Dope-Tenant-ID: $DOPE_TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{"resolution":"confirmed_committed","reason":"provider state verified"}' \
  http://127.0.0.1:19192/v1/live-validations/$VALIDATION_ID/reconciliations/$AMBIGUOUS_COMMIT_ID/resolve
```

## Optional Real-Account Smoke

Real-account smoke is opt-in only:

```bash
make daemon-run-test-live
```

Rules:

- operator chooses the tenant and side-effect scope explicitly,
- no raw secrets are logged or copied into fixtures,
- non-idempotent mutations require per-action approval,
- ambiguous commit states are reconciled by tenant owner/admin or explicit
  reconciliation permission holder,
- skip is acceptable when safe credentials are unavailable and fake-backend coverage
  passes.

## Verification Log

2026-04-29 implementation verification:

- `cd daemon && go test ./internal/livevalidation ./internal/evaluation ./internal/billing ./internal/identity ./internal/store ./internal/api ./internal/integrations ./internal/calendar ./internal/mail ./internal/delivery ./internal/connectors ./internal/reminders ./internal/events ./internal/audit ./internal/contracts` passed.
- `cd daemon && go test ./...` passed after reconciling Roadmap 40 schema inventory rows and the intentional R37 boundary signature golden drift for live-validation helper exports.
- `cd daemon && go mod tidy` completed with no `daemon/go.mod` or `daemon/go.sum` diff.
- `make daemon-contract-test` passed (`go test ./internal/contracts/...`).
- `pnpm test:clients` passed: SDK 15 tests, web 13 tests, TUI 4 tests, and roadmap 7 node smoke.
- `pnpm build` passed for SDK, web, and TUI clients.
- `make daemon-run-test` started the `DOPE_ENV=test` daemon on `127.0.0.1:19192`; `make daemon-test-status` returned `{"ok":true,"service":"dope"}`. The daemon was then stopped with Ctrl-C, and a follow-up health check failed to connect, confirming shutdown.
- Manual `DOPE_ENV=test` live-validation acceptance evidence is represented by the deterministic API, manager, ledger, fake-backend, restart, reconciliation, kill-switch, and non-live replay test suites above. These cover success, denial, unsupported tool class, ambiguous commit, reconciliation, kill-switch abort, restart inspection, and compatibility without requiring real external side effects.
- Optional `make daemon-run-test-live` real-account smoke was skipped because no explicit safe live credentials or operator-selected side-effect scope were provided. Fake-backend coverage passed for integrations, calendar, mail, delivery, connectors, and reminders.
