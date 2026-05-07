# Quickstart: Public Quota UX

This quickstart describes the verification path for Roadmap 47 after implementation.
Run all commands from the repository root unless noted.

## 1. Prepare The Test Environment

Use the test environment only.

```sh
make daemon-run-test
make daemon-test-status
```

Expected result: daemon health reports the test environment under `~/.dope-test` and
`127.0.0.1:19192`.

## 2. Run Daemon Verification

```sh
cd daemon
go test ./...
go mod tidy
```

Required coverage:

- quota dashboard projection includes all enforced catalog categories
- current and previous-period usage projections
- near-limit warnings at 80% consumed and below one category-defined typical operation
  remaining for count, attempt, and byte quota categories
- denial detail for quota exhaustion, quota-state unavailable, and operator-action-needed
- abuse restriction messages hide detection signals and thresholds
- structured redacted JSON evidence export requiring `billing.evidence_export`
- permission gates and cross-tenant denial behavior
- fail-closed denials occur before expensive or side-effecting work

## 3. Run Contract Verification

```sh
make daemon-contract-test
```

Required coverage:

- new and updated JSON schemas validate daemon fixtures
- SDK resource shapes match schema fields
- stable quota reason codes and classifications do not depend on message text
- evidence export includes explicit redaction records

## 4. Run Client Verification

```sh
pnpm test:clients
pnpm build
```

Required coverage:

- TypeScript SDK methods for dashboard, denial detail, and evidence export
- tenant override propagation for all new SDK methods
- web shell quota dashboard and denial detail rendering
- stale quota and denial data cleared or hidden during tenant switch
- support evidence export button hidden or denied without permission

## 5. Manual Test Walkthrough

Use explicit test tenants and seeded test quota states.

The deterministic operator walkthrough can be run against the local test daemon:

```sh
scripts/phase47-public-quota-walkthrough.sh
```

It creates a local pairing token, seeds only `~/.dope-test/daemon.sqlite`, verifies the
dashboard, ordinary denial detail, abuse-restriction denial detail, structured evidence
exports, cross-tenant hiding, and unauthorized no-partial-data behavior, then prints a
sanitized JSON summary.

1. Open a tenant with finite enforced quotas.
2. Confirm all enforced quota categories appear, grouped into readable sections.
3. Confirm current active period and immediately previous completed period are visible.
4. Seed or trigger a quota at 80% consumed and confirm near-limit status.
5. Seed or trigger count, attempt, and byte categories with less than one
   category-defined typical operation remaining and confirm near-limit status.
6. Trigger an exhausted quota denial and confirm the denial detail identifies the source
   operation, stable reason code, measured amount, reset timing, and recovery actions.
7. Trigger or seed an abuse restriction and confirm the UI shows status, affected category,
   duration when available, and recovery action while hiding detection signals and
   thresholds.
8. Switch tenants while quota details are visible and confirm previous-tenant quota and
   denial records disappear before new tenant data renders.
9. Export structured redacted JSON evidence as an authorized support operator with
   `billing.evidence_export` and confirm it contains redaction records and no secrets,
   connector payloads, unrelated run content, or cross-tenant data.
10. Repeat dashboard, denial detail, and evidence export attempts as an unauthorized user
    and confirm stable denial states with no partial data.

## 6. Rollback Check

Rollback is logical disablement of the new projection routes/views/SDK calls while keeping
Roadmap 38 enforcement, billing tables, audit evidence, and stable quota denial behavior
intact. Do not delete usage or audit records as part of rollback.

## 7. Implementation Verification Results

Recorded on 2026-05-07:

- PASS: `go test ./internal/billing ./internal/identity ./internal/contracts ./internal/store` plus `go test ./internal/api -run 'TestHostedBilling'` for the changed daemon paths after setting `GOCACHE` to a writable cache.
- PASS: `GOCACHE=/private/tmp/dope-agent-gocache go test ./...` from `daemon/`.
- PASS: `make daemon-contract-test`.
- PASS: `pnpm test:clients`.
- PASS: `pnpm build`.
- PASS: `go mod tidy` from `daemon/`; no module dependency change was required.
- PASS: post-review gap tests for abuse restriction hydration, effective limit evidence
  state, recursive evidence redaction, immediately previous closed-period selection,
  `request_override` recovery action, and additive tenant-id compatibility.
- PASS: `scripts/phase47-public-quota-walkthrough.sh` provides a repeatable local
  operator walkthrough for the full seeded quota, abuse restriction, evidence export,
  permission, and cross-tenant scenario.

Manual walkthrough status:

- PASS: `make daemon-run-test` and `make daemon-test-status` against `~/.dope-test` on `127.0.0.1:19192`.
- PASS: manual seeded walkthrough confirmed grouped quota dashboard sections, current and previous-period usage, near-limit status, visible override reason, explicit abuse restriction visibility with stable `abuse_restriction:temporary` reason code, denial detail recovery action, and structured evidence export audit references.

Residual risks:

- Billing response schemas keep top-level `tenantId` additive rather than required for
  legacy-client compatibility; daemon responses, fixtures, and SDK types still include it.
- Evidence export audit-reference enrichment is scoped to the denial record, explicit
  restriction source audit reference, and tenant-scoped usage evidence references for the
  denied operation.
- Production tenants, live connectors, payment checkout, invoices, and taxes remain out
  of scope for this phase; rollout validation should run the same walkthrough shape
  against operator-owned staging fixtures, not production state.
