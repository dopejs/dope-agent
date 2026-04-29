# Production Soak Harness

Roadmap 39 requires a 24-hour `DOPE_ENV=test` soak in the isolated test
environment unless a temporary shorter threshold is explicitly recorded with a
mandatory follow-up full-duration rerun.

The soak must cover runtime work, scheduler dispatch, integrations, delivery,
approvals, quota enforcement, tenant switching, and evaluation behavior.

## Required Faults

Fake-backend fault drills must cover transient 5xx, rate limit, auth expiry,
provider unavailable, slow response, and malformed response. Every result is
classified as recovered, retry-exhausted, or operator-action-needed.

## Restarts

Restart the daemon at least three times. Unfinished work after each restart is
classified as recovered, interrupted, retried, or operator-action-needed.
Restart recovery over 5 minutes is a hard failure.

## Resource Observations

Record logs, stored data size, active work or queue backlog, memory, open
handles or file descriptors where available, goroutine count where available,
and monotonic growth. Queue backlog persisting over 30 minutes and monotonic
resource growth over the full run are hard failures.

## Execution

```bash
scripts/production/run-soak.sh
```

The final report must include duration, workload coverage, restart events,
fault drills, recovery times, retry exhaustion, queue backlog, resource growth,
cross-tenant leakage checks, real-account smoke summary or skips, and final
pass/fail status.

Set `DOPE_SOAK_REPORT` to choose the JSON output path. `DOPE_SOAK_DURATION=24h`
performs a real 86400-second run and records elapsed time. Shorter values such
as `targeted-validation`, `30s`, or `5m` are treated as temporary validation and
the generated report marks `followUpFullRerun` as required.
