# Quickstart: Evaluation Product Expansion

## Scope

This quickstart validates Roadmap 41 planning assumptions in the local test environment.
It does not require production state or live connector access.

## Environment

Use the default DopeAgent test environment:

```bash
make daemon-run-test
make daemon-test-status
```

Expected daemon target:

- data directory: `~/.dope-test`
- address: `127.0.0.1:19192`

## Implementation Checkpoints

1. Add additive storage migrations for discovery, product fixtures, campaigns,
   dashboards, tool-call inspections, suppression, and retention metadata.
2. Add tenant-safe store accessors and query-plan coverage before exposing API routes.
3. Add API resources and JSON schemas for all new product evaluation contracts.
4. Add SDK methods and web projections after schemas are contract-tested.
5. Enable bounded discovery workers only after redaction, suppression, and retention
   tests pass.
6. Enable fixture editing and campaign starts only after permission and audit tests pass.
7. Enable dashboards and tool-call inspection after campaign aggregation and Roadmap 40
   live-validation linkage tests pass.

## Targeted Verification

Run targeted daemon tests while implementing each slice:

```bash
cd daemon
go test ./internal/evaluation ./internal/store ./internal/store/tenancy ./internal/api ./internal/audit ./internal/events ./internal/identity ./internal/contracts
```

Run full daemon verification before considering implementation complete:

```bash
cd daemon
go test ./...
go mod tidy
```

Run contract validation whenever schemas, events, API resources, SDK types, or fixtures
change:

```bash
make daemon-contract-test
```

Run client verification whenever SDK or web projections change:

```bash
pnpm test:clients
pnpm build
```

## Product Smoke Flow

After implementation, verify this flow in `~/.dope-test`:

1. Seed or run tenant-owned historical runs, workflows, tool calls, replay attempts,
   comparisons, and Roadmap 40 live-validation evidence.
2. Start a bounded discovery run for one tenant.
3. Confirm discovery records scan bounds, cursor, inspected count, emitted count, and
   partial status when a bound is reached.
4. Time the discovery review and confirm the operator can identify 10 ranked candidates,
   or all eligible candidates if fewer than 10 exist, within 2 minutes of starting the
   review.
5. Confirm candidate evidence is redacted before it appears in list/detail responses.
6. Suppress one run or candidate and confirm future discovery and campaign selection
   exclude it.
7. Create a product-managed fixture from a discovered candidate.
8. Edit the fixture and confirm a new immutable revision, provenance, and audit event.
9. Time fixture creation plus one edit and confirm an authorized user completes the
   workflow in under 5 minutes while retaining source provenance and edit history.
10. Confirm repo-managed fixtures remain visible but cannot be silently edited by product
   fixture routes.
11. Start a campaign with a product fixture and a discovered candidate.
12. Confirm campaign items keep immutable source snapshots.
13. Confirm campaign attempt groups link replay attempts, comparisons, and Roadmap 40
    live-validation ledger evidence.
14. Confirm dashboard aggregates show drift, failure, unsupported replay,
    live-validation linkage, and operator-action-needed counts for the selected tenant
    only.
15. Open tool-call inspection and confirm original, non-live replay, live-validation,
    unsupported, and missing-evidence states are distinguishable and redacted.
16. Apply retention/deletion to discovered candidates, candidate evidence,
    product-managed fixtures, campaign result details, dashboard projections, and
    tool-call inspection payloads; confirm future selection excludes deleted or expired
    product records while runtime truth, repo-managed fixtures, snapshots, and audit
    provenance remain intact.

## Rollback Verification

Verify the operator can disable new Roadmap 41 product actions while preserving evidence:

1. Disable discovery scheduling and new discovery starts.
2. Disable product fixture edits.
3. Disable campaign starts and result publication.
4. Confirm existing product records remain readable to authorized users.
5. Confirm existing Roadmap 33 replay and Roadmap 40 live-validation routes still work.

## Release Readiness Rerun

Before final hosted-productization release readiness, rerun the Roadmap 39 soak harness
with Roadmap 40 live validation and Roadmap 41 evaluation product workflows included.
This rerun is required for Roadmap 41 completion; if the rerun is blocked, Roadmap 41
remains incomplete and the blocker must be recorded with owner, date, and unblock path.
The readiness evidence must include:

- soak workload result
- fault drills
- cross-tenant leakage checks
- discovery bounds and resource-growth report
- campaign result summaries
- live-validation ledger linkage
- product fixture provenance and audit evidence

## Phase 8 Verification Evidence

Recorded on branch `026-evaluation-product-expansion` on 2026-04-30
Asia/Shanghai against the default test environment.

- Prerequisites/checklists: `.specify/scripts/bash/check-prerequisites.sh --json --require-tasks --include-tasks` passed; `checklists/requirements.md` had 16/16 complete items.
- Targeted Go verification: `GOCACHE=/tmp/dope-go-build go test ./internal/evaluation ./internal/store ./internal/store/tenancy ./internal/api ./internal/audit ./internal/events ./internal/identity ./internal/contracts` passed.
- Full daemon verification: `GOCACHE=/tmp/dope-go-build go test ./...` passed after fixing schema inventory coverage for the Roadmap 41 tables.
- Module hygiene: `go mod tidy` from `daemon/` completed with no `go.mod`/`go.sum` diff.
- Contract verification: `make daemon-contract-test` passed.
- Client tests: `pnpm test:clients` passed after aligning the SDK `suppressProductFixture` input type with its default empty input.
- Client build: `pnpm build` passed.
- Test daemon smoke: `make daemon-run-test` started `127.0.0.1:19192`; `make daemon-test-status` returned `{"ok":true,"service":"dope"}`; the foreground daemon session was stopped and `nc -vz 127.0.0.1 19192` returned connection refused afterward.
- Roadmap 41 product smoke: `/usr/bin/time -p env GOCACHE=/tmp/dope-go-build go test ./internal/api -run 'TestEvaluationProduct(DiscoveryAPIRoutes|FixturePermissionDenialsAndLifecycleRoutes|CampaignAPIRoutes|DashboardAPIRequiresReadPermissionAndListsTenantProjections|InspectionAPIRoutes)' -count=1` passed in 1.98 seconds real time. This covers discovery, suppression/fixture lifecycle, campaign start/transitions, dashboard projection reads, and tool-call inspection reads. The automated smoke is below the SC-001 two-minute candidate review and SC-005 five-minute fixture create/edit thresholds.
- Roadmap 39 targeted rerun evidence: `/usr/bin/time -p env DOPE_SOAK_DURATION=targeted-validation DOPE_SOAK_REPORT=specs/026-evaluation-product-expansion/fixtures/roadmap39-rerun-targeted.json scripts/production/run-soak.sh` passed in 0.04 seconds real time with `daemonHealth: "pass"`, `crossTenantLeakage: false`, `monotonicResourceGrowth: false`, and `finalResult: "pass"`.

Full Roadmap 39 soak rerun status: blocked for final release readiness. The required
24-hour `DOPE_ENV=test` rerun with Roadmap 40 live validation and Roadmap 41 evaluation
product workflows included was not completed in this implementation window. Owner:
release owner. Blocker date: 2026-04-30 Asia/Shanghai. Unblock path: seed the combined
Roadmap 40/41 workload on a stable always-on test host, run
`scripts/production/run-soak.sh` with the default `DOPE_SOAK_DURATION=24h`, attach the
generated report, and keep Roadmap 41 in `implementation complete; final soak evidence
pending` status until that report passes.

The targeted-validation report is intentionally not release-gate evidence. A developer
laptop or other movable local machine is not an acceptable environment for the 24-hour
Roadmap 39 rerun because sleep, power events, network changes, VPN changes, OS updates,
and Wi-Fi instability make failures difficult to attribute. The full rerun must execute
on a fixed-power, no-sleep, stable-network test machine or long-running CI/VM runner with
the commit, data directory, daemon config, workload seed, logs, and generated report
captured as artifacts.
