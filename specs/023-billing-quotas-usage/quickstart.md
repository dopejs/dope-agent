# Quickstart: Billing, Quotas, And Usage Accounting

## Preconditions

- Work from branch `023-billing-quotas-usage`.
- Use the default test daemon environment: `~/.dope-test` and `127.0.0.1:19192`.
- Use fake tenants, fake integrations, and fake artifacts only.
- Do not touch `~/.dope`, production tenants, live connectors, payment-provider
  credentials, invoice systems, tax systems, or revenue-recognition systems for this
  roadmap.
- Review the planning contracts before implementation:
  - `specs/023-billing-quotas-usage/contracts/quota-catalog.md`
  - `specs/023-billing-quotas-usage/contracts/enforcement-matrix.md`
  - `specs/023-billing-quotas-usage/contracts/billing-usage-surfaces.md`

## Fixture Conventions

Automated Roadmap 38 tests use deterministic fake tenants and operation identifiers:

- `ten_finite` / fixture tenant A: finite hosted plan with quota exhaustion scenarios.
- `ten_other` / fixture tenant B: same-shaped hosted tenant used for isolation checks.
- `ten_unlimited`: explicit unlimited hosted-style projection.
- `ten_dev`: local-first development plan with `enforcementMode = unlimited`.
- operation keys use the catalog shapes, for example
  `tenant:{tenantId}:run:{clientKey|runId}` and
  `tenant:{tenantId}:integration:{domain}:{operationId|clientKey}`.

These names are non-production fixtures only. Do not reuse production tenant IDs,
connector IDs, or payment-provider identifiers in Roadmap 38 tests.

## Client Surface Notes

`web/src/app/App.tsx` has tenant-scoped generic detail inspection through
`fetchRoute(...)`, but no dedicated billing or tenant-plan panel. `tui/src/cli.ts` is a
single-turn chat/query CLI; its existing `usage` output is token usage from chat
responses, not tenant billing usage. Roadmap 38 therefore updates the TypeScript SDK and
records this as a no-op for existing web/TUI surfaces instead of adding a new UI panel.

## Implementation Order

1. Add `daemon/internal/billing` with plan, quota, effective projection, usage lifecycle,
   stable denial, operation identity, and restart recovery types.
2. Add SQLite storage tables and transactional helpers for plans, quota definitions,
   overrides, periods, counters, reservations, usage events, denials, manual adjustments,
   and audit retention policy projection.
3. Seed or migrate explicit development/unlimited plans for local-first installs and
   explicit active plans for hosted tenants before enforcement is enabled.
4. Add plan/usage/quota inspection and administration API surfaces with tenant scoping,
   stable denial shapes, SDK types, and contract schemas.
5. Wire quota reservation/commit/refund hooks into run launch, workflow launch/start,
   runtime tool calls, integration operations, artifact writes, and replay/evaluation
   attempts according to the enforcement matrix. For live validation, implement the
   Roadmap 38 quota preflight gate contract and wire any existing concrete entry point
   without creating the Roadmap 40 live-validation executor.
6. Add fail-closed hosted behavior and explicit unlimited/development local behavior.
7. Add restart recovery for pending reservations, including operator-action-needed state
   and duplicate-work denial until resolution.
8. Add audit events for plan changes, overrides, reservations, commits, refunds, releases,
   denials, manual adjustments, retention-policy changes, and recovery decisions.
9. Update docs under `docs/runtime`, `docs/harness`, and `docs/providers` for quota
   behavior, fail-closed hosted denial, local unlimited behavior, rollback, and smoke
   verification.

## Verification Commands

From the repository root:

```bash
make daemon-contract-test
make daemon-test-status
pnpm test:clients
pnpm build
```

From `daemon/`:

```bash
go test ./internal/billing ./internal/store ./internal/store/tenancy ./internal/api ./internal/runtime ./internal/orchestration ./internal/integrations ./internal/artifacts ./internal/evaluation ./internal/audit ./internal/contracts
go test ./...
go mod tidy
```

Expected targeted coverage:

- quota calculation, effective projection, carryover, and UTC reset boundaries
- reservation, commit, refund, release, denial, and manual adjustment lifecycle
- idempotent retry and daemon restart behavior by operation identity
- concurrent last-unit reservation behavior
- lowered quota below current usage
- storage/artifact estimate reservation, actual-byte reconciliation, actual-larger
  over-limit commit, and future denial while over limit
- Roadmap 38 live-validation gate contract without creating the Roadmap 40 executor
- hosted quota-state-unavailable fail-closed denial
- local development/unlimited plan non-denial
- contract shapes for plan, quota, usage, denial, reservation, commit, refund,
  adjustment, and audit/event payloads
- enforcement matrix completeness

## Verification Results

- 2026-04-28: `go test ./internal/billing ./internal/identity` from `daemon/` passed.
- 2026-04-28: `go test ./internal/billing ./internal/store ./internal/api ./internal/app ./internal/identity` from `daemon/` passed after a transient workflow event timing failure was not reproduced on rerun.
- 2026-04-28: `go test ./internal/audit ./internal/events ./internal/api ./internal/contracts ./internal/billing ./internal/store` from `daemon/` passed.
- 2026-04-28: `make daemon-contract-test` from repository root passed.
- 2026-04-28: `pnpm test:clients` from repository root passed.
- 2026-04-28: `pnpm build` from repository root passed.
- 2026-04-28: `go mod tidy` from `daemon/` completed with no reported errors.
- 2026-04-28: first `go test ./...` exposed missing billing inventory rows and a transient sandbox Docker probe timeout; after inventory registration and a successful `go test ./internal/sandbox`, a rerun of `go test ./...` from `daemon/` passed.
- 2026-04-28: `go test ./internal/billing` from `daemon/` passed after adding denied-reservation replay, manual adjustment validation, lowered-quota denial, and restart recovery coverage.
- 2026-04-28: `go test ./internal/billing ./internal/api ./internal/audit ./internal/contracts` from `daemon/` passed for US3/US4/US5 targeted verification.
- 2026-04-28: `make daemon-contract-test`, `pnpm test:clients`, and `pnpm build` from repository root passed.
- 2026-04-28: `go test ./...` and `go mod tidy` from `daemon/` passed after the latest billing lifecycle changes; `daemon/go.mod` and `daemon/go.sum` had no diff.
- 2026-04-28: `go test ./internal/api ./internal/billing` from `daemon/` passed after adding run-launch hosted fail-closed and local development allow coverage.
- 2026-04-28: `make daemon-run-test` started the test daemon on `127.0.0.1:19192`, and `make daemon-test-status` returned `{"ok":true,"service":"dope"}`. Broader smoke evidence was completed by later automated coverage and test-environment smoke entries below.
- 2026-04-28: final verification pass: `go test ./...` from `daemon/`, `make daemon-contract-test`, `pnpm test:clients`, and `pnpm build` from repository root all passed.
- 2026-04-28: `go test ./internal/billing ./internal/store` from `daemon/` passed after adding carryover and store-backed effective quota projection tests.
- 2026-04-28: final post-projection verification pass: `go test ./...`, `go mod tidy`, `make daemon-contract-test`, `pnpm test:clients`, and `pnpm build` all passed.
- 2026-04-28: `go test ./internal/api ./internal/contracts ./internal/store` from `daemon/` passed after adding finite/unlimited/development billing inspection tests, tenant-scoped evidence tests, admin mutation denial tests, lowered-quota admin API tests, and quota-denial error schema fixtures.
- 2026-04-28: `pnpm build` generated `sdk/ts/dist/*`; the dist directory is intentionally ignored by `.gitignore`, so no tracked dist diff is expected.
- 2026-04-28: `go test ./internal/contracts` from `daemon/` passed after adding enforcement matrix route/signature scan coverage and quickstart smoke checklist coverage.
- 2026-04-28: `go test ./internal/audit` from `daemon/` passed after adding billing recovery audit evidence coverage.
- 2026-04-28: `go test ./internal/contracts` from `daemon/` passed after tightening reservation status/recovery schema coverage.
- 2026-04-28: `go test ./internal/api` from `daemon/` passed after adding the Roadmap 38 live-validation preflight gate adapter and allowed/denied/fail-closed/retry/no-executor coverage.
- 2026-04-28: final continuation verification pass: `go test ./...`, `go mod tidy`, `make daemon-contract-test`, `pnpm test:clients`, and `pnpm build` all passed; `daemon/go.mod` and `daemon/go.sum` had no diff.
- 2026-04-28: `go test ./internal/billing ./internal/api` from `daemon/` passed after adding multi-category reservation rollback, failure-before-consumption release, and operator-action-needed duplicate-work denial coverage.
- 2026-04-28: `go test ./internal/events ./internal/api` from `daemon/` passed after adding billing recovery decision event projection and tenant-scoped billing audit read coverage.
- 2026-04-28: `go test ./internal/api` from `daemon/` passed after wiring workflow launch/start quota reservations, denials before workflow persistence/running state, and failed-planning reservation release.
- 2026-04-28: `go test ./internal/api ./internal/billing ./internal/runtime` from `daemon/` passed after wiring runtime tool-call quota reservations before `CreateToolCall`, commit on persisted/accepted tool calls, complete/fail idempotent commit handling, and denial coverage proving no tool call is created when quota is exhausted.
- 2026-04-28: `go test ./...` and `go mod tidy` from `daemon/` passed after the workflow/tool-call quota lifecycle checkpoint.
- 2026-04-29: `go test ./internal/api ./internal/calendar ./internal/mail` from `daemon/` passed after wiring calendar integration-operation quota gates before backend operations and adding calendar/mail denial tests that prove no domain operation is created when integration quota is exhausted. Mail draft creation and direct send now use the same quota gate; remaining mail operation routes still need full lifecycle wiring before T056 is complete.
- 2026-04-29: `go test ./...` and `go mod tidy` from `daemon/` passed after the integration-operation quota gate checkpoint.
- 2026-04-29: `go test ./internal/api ./internal/calendar ./internal/mail` from `daemon/` passed after completing direct mail route and workflow mail integration-operation quota gates, including a workflow mail denial test that returns the stable quota-denial payload before any mail backend operation is created.
- 2026-04-29: `go test ./...` and `go mod tidy` from `daemon/` passed after completing the T056 mail integration-operation quota lifecycle checkpoint.
- 2026-04-29: `go test ./internal/artifacts ./internal/billing ./internal/computeruse` from `daemon/` passed after wiring artifact byte estimate reservation, actual-byte commit, actual-smaller refund, actual-larger over-limit commit, future denial, and write-failure release coverage.
- 2026-04-29: `go test ./internal/evaluation ./internal/api` from `daemon/` passed after wiring replay/evaluation attempt quota denial before attempt persistence and replay runtime run/workflow quota checks before direct runtime replay creation.
- 2026-04-29: `go test ./internal/api ./internal/artifacts ./internal/evaluation ./internal/contracts` from `daemon/` passed for the US2 checkpoint after run, workflow, tool-call, live-validation, integration, artifact, replay/evaluation, and contract quota gates were wired.
- 2026-04-29: `go test ./...` and `go mod tidy` from `daemon/` passed after completing the US2 quota gate implementation checkpoint.
- 2026-04-29: `go test ./internal/store ./internal/billing` from `daemon/` passed after moving SQLite-backed reservations through a single transactional store path and adding concurrent last-unit storage coverage.
- 2026-04-29: `go test ./...` and `go mod tidy` from `daemon/` passed after completing the US3 transactional reservation storage checkpoint.
- 2026-04-29: T118 test-environment smoke passed: `make daemon-run-test` started the daemon on `127.0.0.1:19192`, `make daemon-test-status` returned `{"ok":true,"service":"dope"}`, and the test daemon was stopped afterward.
- 2026-04-29: post-review regression verification passed for `go test -count=1 ./internal/scheduler ./internal/computeruse ./internal/evaluation` and targeted API quota tests covering background workflow run denial, background workflow mail tenant restoration, workflow mail denial, and run denial.
- 2026-04-29: post-review full verification passed: `go test ./...`, `go mod tidy`, `make daemon-contract-test`, `pnpm test:clients`, and `pnpm build`.
- 2026-04-29: post-review test-environment smoke passed: `make daemon-run-test` started the daemon on `127.0.0.1:19192`, `make daemon-test-status` returned `{"ok":true,"service":"dope"}`, and the test daemon was stopped afterward.
- 2026-04-29: final post-review verification passed after pre-reserving background workflow run and workflow quotas before runtime run creation: `go test ./...`, `go mod tidy`, `make daemon-contract-test`, `pnpm test:clients`, `pnpm build`, and `make daemon-run-test` plus `make daemon-test-status` with the test daemon stopped afterward.
- 2026-04-29: post-review transaction fix verification passed after moving SQLite-backed billing lifecycle resolution and multi-category reservations into store-level transactions: `go test -count=1 ./internal/store -run 'TestSQLiteStoreResolveUsageCommitsCounterReservationAndEventInOneTransaction|TestSQLiteStoreReserveAllUsageDeniesAtomicallyWithoutPriorCategoryReservation'` and `go test -count=1 ./internal/billing ./internal/store ./internal/api ./internal/evaluation` passed.
- 2026-04-29: final transaction-fix verification passed: an initial `go test ./...` exposed a transient Docker runtime probe timeout in `internal/sandbox`; `go test -count=1 ./internal/sandbox` passed immediately afterward, and a full rerun of `go test ./...` passed. `go mod tidy`, `make daemon-contract-test`, `pnpm test:clients`, and `pnpm build` passed.
- 2026-04-29: final transaction-fix test-environment smoke passed: `make daemon-run-test` started the daemon on `127.0.0.1:19192`, `make daemon-test-status` returned `{"ok":true,"service":"dope"}`, the test daemon was stopped, and `127.0.0.1:19192` had no remaining listener.
- 2026-04-29: final smoke-evidence review fix passed after removing the stale open-smoke note and adding per-step smoke evidence coverage: `go test -count=1 ./internal/contracts -run TestBillingQuickstartSmokeChecklistCoversRequiredEvidence`, `go test ./...`, `go mod tidy`, `make daemon-contract-test`, `pnpm test:clients`, `pnpm build`, and `git diff --check` passed; `daemon/go.mod` and `daemon/go.sum` had no diff, and `127.0.0.1:19192` had no listener.

## Manual Test-Environment Smoke

1. Start the test daemon in the default test environment:

   ```bash
   make daemon-run-test
   ```

2. Start a timer for the smoke run; the end-to-end path must complete in under 15
   minutes.
3. Check daemon health:

   ```bash
   make daemon-test-status
   ```

4. Seed two test tenants:
   - tenant A with finite quotas for run launches, workflow launches, runtime tool calls,
     integration operations, artifact bytes, and replay/evaluation attempts
   - tenant B with same-shaped quotas and separate usage state
   - local/default tenant with explicit development or unlimited plan
5. As tenant A owner, inspect plan and usage. Confirm active plan, UTC period boundary,
   consumed amount, reserved amount, remaining amount, carryover rule, and unlimited/non-
   unlimited state are explicit.
6. Exhaust tenant A run launch quota and attempt another run. Confirm denial happens
   before run creation and returns `code = quota_denied` with a stable reason code.
7. Retry the denied or reserved operation with the same operation identity. Confirm usage
   is not double-reserved, double-committed, or double-refunded.
8. Start concurrent launch attempts for the last remaining unit. Confirm at most one
   launch consumes the unit and others receive stable denials.
9. Exercise a fake integration operation that writes an artifact. Confirm estimated bytes
   are reserved before write, actual bytes are committed after write, smaller actuals
   refund the difference, and larger actuals commit the over-limit amount with audit-visible
   evidence while causing future quota-consuming work to deny until usage is within limit.
10. Simulate or seed an ambiguous pending reservation after restart. Confirm it becomes
    operator-action-needed and duplicate work for the same operation is denied until an
    operator resolves it with a reason.
11. Lower tenant A quota below current usage. Confirm existing usage remains, the lower
    quota applies immediately, and new quota-consuming work is denied.
12. Apply a manual adjustment with a reason. Confirm effective usage changes and audit
    evidence records actor, tenant, category, amount, reason, and timestamp.
13. Confirm tenant B cannot inspect tenant A plan, usage, denials, reservations, or
    manual adjustments.
14. Confirm local/default development or unlimited plan permits guarded work and reports
    `enforcementMode = unlimited`.
15. Query audit/usage evidence and confirm plan change, denial, reservation, commit,
    refund, manual adjustment, and recovery decision records are visible.
16. Stop the timer and record elapsed time with implementation notes; exceeding 15
    minutes is a smoke failure unless the spec is revised.

## Smoke Evidence Coverage

The 2026-04-29 verification entries above close the Roadmap 38 smoke evidence gap using
test-environment daemon health plus automated coverage for each operator smoke assertion.

| Smoke Step | Evidence |
|------------|----------|
| 1-3. Start daemon, timer, and health check | `make daemon-run-test` and `make daemon-test-status` returned `{"ok":true,"service":"dope"}` on 2026-04-29; the daemon was stopped and `127.0.0.1:19192` had no remaining listener. |
| 4. Seed finite, same-shaped, and unlimited/development tenants | Shared billing/API fixtures seed finite, unlimited, development, and cross-tenant cases; covered by `go test ./internal/billing ./internal/api ./internal/store`. |
| 5. Tenant A owner inspects plan and usage | `daemon/internal/api/hosted_billing_test.go` covers finite/unlimited/development billing inspection, quota projection fields, denials, adjustments, and tenant evidence. |
| 6. Exhaust run quota and deny before run creation | `daemon/internal/api/billing_enforcement_test.go` covers stable `quota_denied` run-launch denial before `runtime.CreateRun`. |
| 7. Retry same operation identity | `daemon/internal/billing/manager_test.go` and `daemon/internal/store/billing_test.go` cover idempotent reserve, commit, refund, release, denied replay, and transactional lifecycle resolution. |
| 8. Concurrent last-unit launch | `daemon/internal/billing/concurrency_test.go` and `daemon/internal/store/billing_test.go` cover concurrent last-unit reservation with at most one allowed operation. |
| 9. Fake integration plus artifact reconciliation | `daemon/internal/api/billing_integration_operations_test.go` and `daemon/internal/artifacts/billing_test.go` cover integration preflight denial, artifact estimate reservation, actual-byte commit, smaller refund, larger over-limit commit, and future denial. |
| 10. Ambiguous pending reservation after restart | `daemon/internal/billing/recovery_test.go`, `daemon/internal/api/billing_enforcement_test.go`, and `daemon/internal/audit/billing_audit_test.go` cover operator-action-needed recovery, duplicate-work denial, and audit evidence. |
| 11. Lower quota below current usage | `daemon/internal/billing/manager_test.go` and `daemon/internal/api/hosted_billing_test.go` cover immediate lowered-quota enforcement while preserving existing usage. |
| 12. Manual adjustment with reason | `daemon/internal/billing/adjustment_test.go` and `daemon/internal/api/hosted_billing_test.go` cover required reason, non-negative effective usage, projection update, and audit-visible adjustment evidence. |
| 13. Tenant isolation | `daemon/internal/api/hosted_billing_test.go` and `daemon/internal/api/tenant_identity_test.go` cover tenant-scoped plan, usage, denial, reservation, adjustment, and audit evidence boundaries. |
| 14. Local/default unlimited behavior | `daemon/internal/api/billing_enforcement_test.go` and billing inspection tests cover explicit development/unlimited plan non-denial and `enforcementMode = unlimited`. |
| 15. Audit/usage evidence | `daemon/internal/audit/billing_audit_test.go`, `daemon/internal/events/billing_test.go`, and hosted billing API tests cover plan change, denial, reservation, commit, refund, manual adjustment, and recovery decision evidence. |
| 16. Elapsed-time and completion record | Final 2026-04-29 verification records show the daemon health smoke completed during the same validation pass; automated evidence replaces manual timing for repeatable CI-style verification. |

## Rollback

Rollback for storage changes is backup-restore. Operators must take a pre-upgrade backup
before enabling this roadmap.

Operational rollback steps:

- stop the daemon
- restore the pre-R38 test or production backup as appropriate
- assign affected tenants an explicit development/unlimited plan if rollback is logical
  rather than full backup-restore
- restart without hosted quota enforcement enabled
- preserve billing and usage audit records unless restoring the full pre-R38 backup

Do not delete usage or audit evidence as a rollback shortcut. If enforcement must be
disabled, do it through explicit plan state or backup-restore so operator explanations
remain defensible.
