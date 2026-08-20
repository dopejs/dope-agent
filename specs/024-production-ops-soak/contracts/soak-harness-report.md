# Contract: Soak Harness And Report

This contract defines the minimum workload, fault, restart, resource, and report evidence
for Roadmap 39.

## Required Workload Coverage

Every valid soak scenario must cover:

| Area | Evidence Requirement |
|------|----------------------|
| Runtime | run, step, or tool-call work progresses and reaches terminal or recoverable state |
| Scheduler | scheduled work dispatches during the run |
| Integrations | fake integration/backend operations execute under success and fault modes |
| Delivery | delivery attempt/outcome state is recorded |
| Approvals | approval wait/resolution or operator-action-needed behavior is exercised |
| Quotas | allowed, denied, retry, and restart-sensitive accounting behavior is observed |
| Tenant switching | work and evidence remain scoped to the active tenant |
| Evaluation | replay/evaluation attempt or fixture behavior is exercised |

## Duration And Restart Requirements

- First baseline duration: at least 24 hours in `KURA_ENV=test`.
- Minimum restarts: 3 daemon restarts during the run.
- Restart evidence must record restart timestamp, in-flight work, recovered state, and
  recovery time.

Unfinished work after restart must be classified as:

- `recovered`
- `interrupted`
- `retried`
- `operator_action_needed`

## Fault Drill Requirements

Fake-backend fault drills must include:

- transient 5xx
- rate limit
- auth expiry
- provider unavailable
- slow response
- malformed response

Each observed fault must be classified as:

- `recovered`
- `retry_exhausted`
- `operator_action_needed`

Unclassified fault outcomes are hard failures.

## Resource Observations

The report must record observations for:

- log growth
- database or stored data size
- active work or queue backlog
- memory
- open handles or file descriptors where available
- goroutine count where available

Missing observability for a required threshold must be reported as a planning or
implementation gap, not silently ignored.

## Hard-Fail Thresholds

A soak report must fail when any of the following occur:

- any cross-tenant leakage
- any unclassified failure
- restart recovery takes more than 5 minutes
- retry exhaustion occurs without operator-action-needed state
- queue backlog persists for more than 30 minutes
- any tracked resource category grows monotonically over the full run

## Required Report Fields

Machine-readable reports, markdown summaries, or contract fixtures must include:

- run id or report id
- branch or product version under test
- environment and data directory
- start and end timestamps
- total duration
- tenant set summary
- workload coverage summary
- restart event summary
- fault drill summary
- recovery time summary
- retry exhaustion summary
- queue backlog summary
- resource growth summary
- cross-tenant leakage result
- real-account smoke summary or explicit skip evidence
- final pass/fail result

## Contract Tests

Required tests or checks:

- missing required report fields fail validation
- reports shorter than the required duration fail unless a temporary threshold and follow-up
  rerun requirement are explicitly recorded
- fewer than three restarts fail validation
- missing required fault categories fail validation
- any hard-fail threshold marks the report failed
- successful reports cannot contain unclassified failures or cross-tenant leakage

Final implementation fixtures:

- `specs/024-production-ops-soak/fixtures/soak-report.passing.json`
- `specs/024-production-ops-soak/fixtures/soak-report.failures.json`
