# Quickstart: Production Operations Soak

## Preconditions

- Work from branch `024-production-ops-soak`.
- Use the default test daemon environment: `~/.kura-test` and `127.0.0.1:19192`.
- Use fake tenants, fake integrations, fake credentials, and fake external-service faults
  by default.
- Do not touch `~/.kura`, production user data, live connectors, or real-account
  credentials unless running an explicitly opted-in real-account smoke.
- Review the planning contracts before implementation:
  - `specs/024-production-ops-soak/contracts/backup-restore-evidence.md`
  - `specs/024-production-ops-soak/contracts/soak-harness-report.md`
  - `specs/024-production-ops-soak/contracts/release-readiness-gate.md`

## Fixture Conventions

Automated Roadmap 39 tests should use deterministic fake tenants:

- `ten_ops_alpha`: active credential references, finite quota usage, scheduled/runtime
  work, and delivery/evaluation evidence.
- `ten_ops_beta`: same-shaped resources with different quota and work state to prove
  isolation.
- `ten_ops_gamma`: disabled or reconnect-needed credential state plus retry or
  operator-action-needed recovery evidence where practical.

No fixture may contain production tenant ids, production account ids, raw credential
material, access tokens, refresh tokens, OAuth codes, or local CLI auth material.

## Implementation Order

1. Add production install and upgrade runbooks under `docs/runtime`, with explicit
   tenant-scoped single-node scope and out-of-scope multi-node managed hosting.
2. Add backup and restore runbook sections, including raw credential exclusion,
   secret-reference remediation, integrity checks, invalid-backup behavior, and
   backup-restore rollback.
3. Add or extend representative multi-tenant migration fixtures under
   `daemon/internal/store/migrationfixture` with at least three tenants and distinct
   credential, quota, and work states.
4. Add restore validation checks that prove tenant-owned records restore to the correct
   tenant, cross-tenant leakage is zero, raw credentials are absent, and credential-bearing
   integrations require reconnect or revalidation.
5. Add migration preflight and postflight verification evidence, reusing existing
   migration progress and rollback docs where possible.
6. Add the soak harness or script surface for a 24-hour `KURA_ENV=test` run covering
   runtime, scheduler, integrations, delivery, approvals, quota enforcement, tenant
   switching, and evaluation behavior.
7. Add fake-backend fault drills for transient 5xx, rate limit, auth expiry, provider
   unavailable, slow response, and malformed response.
8. Add at least three daemon restarts during the soak and classify unfinished work as
   recovered, interrupted, retried, or operator-action-needed.
9. Add resource-growth observations for logs, stored data size, active work/queue backlog,
   memory, open handles/file descriptors where available, and goroutine count where
   available.
10. Add soak report validation for all required fields and hard-fail thresholds.
11. Add real-account smoke checklist support: run where safe credentials are available,
    otherwise record explicit skip reasons while fake-backend coverage remains mandatory.
12. Add release-readiness checklist and Roadmaps 40/41 rerun gate.
13. Update `docs/specs/024-production-install-upgrade-backup-and-soak.md`,
    `docs/runtime/daemon-roadmaps.md`, and related runtime/harness/provider docs to point
    at the final runbooks and evidence.

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
go test ./internal/store ./internal/store/tenancy ./internal/app ./internal/integrations ./internal/calendar ./internal/mail ./internal/delivery ./internal/scheduler ./internal/orchestration ./internal/runtime ./internal/evaluation ./internal/billing ./internal/secrets ./internal/audit ./internal/events ./internal/contracts
go test ./...
go mod tidy
```

Use the default test daemon:

```bash
make daemon-run-test
make daemon-test-status
```

Expected targeted coverage:

- clean install runbook walkthrough evidence
- representative upgrade preflight and postflight evidence
- backup and restore regression on at least three tenants
- raw credential exclusion from backup, restore, logs, reports, fixtures, and diagnostics
- restore leaves credential-bearing integrations blocked until reconnect/revalidation
- invalid or incompatible backup fails clearly
- rollback guidance requires backup-restore when in-place rollback is unsafe
- 24-hour soak report with required fields
- at least three daemon restarts with recovery classifications
- all required fake-backend fault categories
- restart recovery <=5 minutes
- queue backlog clears within 30 minutes
- no retry exhaustion without operator-action-needed state
- no monotonic resource growth over the full soak
- zero cross-tenant leakage and zero unclassified failures
- real-account smoke run or explicit skip evidence per supported domain
- Roadmaps 40/41 rerun gate present in release-readiness evidence

## Manual 24-Hour Soak Flow

1. Confirm no test daemon is already running on `127.0.0.1:19192`.
2. Start the daemon in test mode:

   ```bash
   make daemon-run-test
   ```

3. Confirm health:

   ```bash
   make daemon-test-status
   ```

4. Seed or select the representative three-tenant test data set.
5. Start the soak harness and record start time, branch, environment, data directory, and
   tenant set.
6. Exercise the required workload areas: runtime, scheduler, integrations, delivery,
   approvals, quota, tenant switching, and evaluation.
7. Inject each required fake-backend fault category.
8. Restart the daemon at least three times and record recovery classifications and timing.
9. Capture resource observations throughout the run.
10. At 24 hours, stop workload generation and generate the soak report.
11. Validate the report against hard-fail thresholds.
12. Record real-account smoke results or skip reasons.
13. Complete the release-readiness checklist.
14. Stop the test daemon and confirm `127.0.0.1:19192` has no remaining listener.

## Rollback

Rollback for persisted state changes is backup-restore. Operators must take a verified
backup before upgrade or migration verification. Do not use raw credential export as a
rollback shortcut.

Operational rollback steps:

- stop the daemon
- identify whether in-place rollback is safe
- if persisted state cannot be safely reversed, restore the verified pre-upgrade backup
- start the compatible daemon version for the restored state
- confirm tenant data integrity, credential remediation state, quota/accounting state, and
  health checks
- record the rollback evidence in the release-readiness checklist

Do not delete audit, quota, or recovery evidence as a shortcut unless restoring the full
pre-upgrade backup.

## Implementation Notes

- Shared evidence validators live in `daemon/internal/opsreadiness`.
- The representative Roadmap 39 tenant fixture lives in
  `daemon/internal/store/migrationfixture/r39_production_ops.go` and now creates an
  actual SQLite fixture with tenant, secret-reference, quota, and work-state tables for
  backup/restore regression.
- Fake-backend fault drill controls live in `daemon/internal/integrations/fake_backend.go`.
- `scripts/production/run-soak.sh` writes a structured report artifact with workload,
  restart, fault, resource, leakage, elapsed-time, temporary-duration, and pass/fail
  fields. `KURA_SOAK_DURATION=24h` uses a real 86400-second run; shorter runs are marked
  as temporary validation with `followUpFullRerun=true`.
- Production helper scripts live in `scripts/production` and were verified executable:
  `upgrade-preflight.sh`, `upgrade-postflight.sh`, `backup-test-state.sh`,
  `restore-test-state.sh`, `run-soak.sh`, and `restart-test-daemon.sh`.
- Operator runbooks are linked from `docs/runtime/production-operations.md`.

## Verification Results

- Checklist status: `requirements.md` passed with 16 completed items and 0 incomplete.
- Ignore file verification: existing `.gitignore` already covered Node.js, Go, and
  universal build/runtime patterns; no ignore-file change was required.
- Targeted Go tests passed:
  `go test ./internal/opsreadiness ./internal/store/migrationfixture ./internal/integrations ./internal/contracts`.
- Full daemon tests passed: `go test ./...` from `daemon/`.
- Module tidy completed: `go mod tidy` from `daemon/` produced no required dependency
  changes.
- Contract test passed: `make daemon-contract-test`.
- Client verification was not run because this implementation did not add SDK, web, TUI,
  public API, event schema, or client-visible contract surfaces.
- Test daemon smoke passed: `make daemon-run-test` started the daemon on
  `127.0.0.1:19192`, `make daemon-test-status` returned
  `{"ok":true,"service":"kura"}`, and the daemon was stopped after verification.

## Soak Evidence Status

The full 24-hour `KURA_ENV=test` soak was not run to completion in this implementation
session. The runner now enforces a real 86400-second run for `KURA_SOAK_DURATION=24h`;
temporary shorter validation was used only to verify the generated report artifact,
report fixtures, validators, restart/fault/resource hard-fail logic, backup/restore
SQLite regression, and test-daemon smoke path.

Mandatory follow-up before final release readiness: run the full 24-hour
`KURA_ENV=test` soak with `scripts/production/run-soak.sh`, record the generated report,
and replace this temporary note with the full-duration pass/fail evidence. Roadmaps 40
and 41 must rerun this gate after their changes land.
