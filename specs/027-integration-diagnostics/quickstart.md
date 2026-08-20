# Quickstart: Integration Health And Permission Diagnostics

## Scope

Use this quickstart to validate the Roadmap 42 implementation after tasks are generated
and code changes are made. All local validation defaults to the test environment
(`~/.kura-test`, `127.0.0.1:19192`).

## 1. Review Planning Contracts

Before implementation, confirm these files match the planned API, schema, SDK, web, and
operator evidence:

```sh
sed -n '1,220p' specs/027-integration-diagnostics/plan.md
sed -n '1,220p' specs/027-integration-diagnostics/research.md
sed -n '1,260p' specs/027-integration-diagnostics/data-model.md
find specs/027-integration-diagnostics/contracts -type f -maxdepth 1 -print
```

## 2. Targeted Daemon Tests

Run targeted packages first while implementing:

```sh
cd daemon
go test ./internal/integrations ./internal/api ./internal/store ./internal/store/tenancy ./internal/audit ./internal/events ./internal/opsreadiness ./internal/livevalidation
```

Required focused coverage:

- Feishu/Lark reason-code classification fixtures.
- Limited or unsupported diagnostic projection for non-Feishu/Lark domains.
- Diagnostic state freshness and stale-after-15-minutes behavior.
- User-facing failures use current diagnostic truth.
- Redaction fail-closed behavior.
- Tenant isolation and permission denial non-disclosure.
- 90-day diagnostic and smoke retention.
- Real-account smoke pass, fail, blocked, and skipped outcomes.
- Dual approval for non-idempotent or externally visible smoke probes.
- Release-readiness evidence includes Roadmap 42 diagnostic and smoke outcomes.

## 3. Contract And Schema Validation

Any API, schema, SDK, event, or fixture change requires contract validation:

```sh
make daemon-contract-test
pnpm test:clients
pnpm build
```

Contract fixtures must include:

- diagnostic result and diagnostic run resources,
- reason-code catalog,
- user-facing remediation payloads,
- smoke matrix reports and probe outcomes,
- audit events for run lifecycle, permission denial, redaction failure, retention, and
  smoke publication,
- release-readiness evidence with pass, fail, blocked, skipped, limited, and unsupported
  states.

## 4. Full Daemon Tests

Run the full Go suite from `daemon/`:

```sh
cd daemon
go test ./...
go mod tidy
```

Do not hide failures. If full tests cannot complete, record the failing packages and
impact in the implementation summary.

## 5. Local Test Daemon Smoke

Start the isolated test daemon:

```sh
make daemon-run-test
make daemon-test-status
```

Smoke expectations:

- diagnostic routes return tenant-scoped data only,
- stale cached state is marked stale after 15 minutes,
- unsupported-domain classifications are visible rather than absent,
- redaction-failure fixtures show only generic safe classification,
- release-readiness summary includes Roadmap 42 diagnostic evidence.

## 6. Optional Safe Real-Account Smoke

Run real-account smoke only when safe credentials and tenant approval are explicitly
available. Non-idempotent or externally visible probes require both tenant administrator
and authorized operator approval.

Expected outcomes:

- safe Feishu/Lark smoke completes within 10 minutes, or
- missing credentials, tenant approval, provider outage, unsafe scope, unsupported
  domain, or operator deferral records structured blocked/skipped output.

No real-account smoke output may contain raw secrets, OAuth tokens, refresh tokens, app
secrets, authorization headers, or credential-bearing provider payloads.

## 7. Implementation Notes

Roadmap 42 implementation adds tenant-scoped diagnostic runs/results, provider
classification fixtures, diagnostic failure projections, real-account smoke evidence,
audit/event builders, and retention records. Local verification completed with targeted
daemon tests, schema contracts, client tests/build, full daemon tests, `go mod tidy`, and
isolated test daemon health. The test daemon was stopped after health verification.

## Release Truth Status

Roadmap 42 is implementation and local verification complete. Remaining stable-host or
real-account release evidence is tracked as release residual work, not as missing
implementation. Use `docs/runtime/release-truth-checklist.md` to classify missing safe
credentials, unavailable tenant approval, operator-deferred smoke, stale evidence, and
public-readiness claims.
