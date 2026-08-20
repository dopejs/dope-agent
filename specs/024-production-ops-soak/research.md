# Research: Production Operations Soak

## Decision: Treat Roadmap 39 as an operational evidence baseline, not a new domain feature

**Rationale**: The upstream roadmap requires install, upgrade, backup, restore, soak,
fault, and release-gate evidence before live validation and evaluation-product expansion.
Those requirements are best closed by runbooks, repeatable harnesses, fixtures, reports,
and contract tests around existing daemon surfaces rather than by adding a new product
domain.

**Alternatives considered**:

- Add a new production-ops API domain: rejected because Roadmap 39 does not require an
  operator API unless implementation discovers missing diagnostics that cannot otherwise
  be observed.
- Treat the roadmap as docs-only: rejected because the upstream spec requires tested
  backup/restore, migration verification, soak reports, resource checks, and fault drills.

## Decision: Validate a tenant-scoped single-node production baseline

**Rationale**: Clarification selected tenant-scoped single-node production operation.
This matches the current daemon architecture and upstream dependency on Roadmaps 35, 37,
and 38 without pulling in clustering or distributed failover work.

**Alternatives considered**:

- Local-first single-user only: rejected because Roadmap 39 explicitly validates the
  tenant-scoped, quota-aware product baseline.
- Multi-node managed hosting: rejected because clustering, distributed failover, and
  managed service rollout are outside this phase and would expand scope beyond the fixed
  roadmap.

## Decision: Backup artifacts exclude raw credential material

**Rationale**: Roadmap 37 credential isolation rules state raw values must never appear in
API responses, events, logs, replay fixtures, evaluation artifacts, contract fixtures, or
smoke-test output. Extending that rule to backup and restore avoids creating a more
dangerous credential export path. Restore can still recover tenant ownership, secret refs,
status, and remediation context, then require reconnect or revalidation before
credential-bearing use.

**Alternatives considered**:

- Encrypted raw credential backup: rejected because it materially increases leak and key
  management risk for this phase.
- Exclude all secret/integration state: rejected because operators need enough metadata to
  identify remediation after restore and prove tenant isolation.

## Decision: Use at least three tenants for representative backup/restore verification

**Rationale**: Three tenants with distinct credential, quota, and work states catch more
failure modes than a two-tenant collision test: tenant-specific credential remediation,
quota accounting divergence, in-flight work/restart state, and cross-tenant leakage across
different shapes.

**Alternatives considered**:

- Two tenants only: rejected as too narrow for credential/quota/work-state diversity.
- Production-sized sample: rejected because Roadmap 39 is an operational correctness gate,
  not a scale/load benchmark.

## Decision: Make the first baseline soak a 24-hour `KURA_ENV=test` run

**Rationale**: The upstream roadmap requires at least 24 hours unless a shorter temporary
threshold is explicitly recorded. A full-day test-environment soak is the minimum useful
evidence for long-running scheduler/runtime behavior, resource growth, restart recovery,
and external-service instability.

**Alternatives considered**:

- Short smoke-only soak: rejected because it cannot prove long-running behavior or
  resource growth.
- Multi-day soak as initial gate: rejected as useful later but too heavy for the first
  roadmap baseline.

## Decision: Hard-fail soak on explicit release-blocking thresholds

**Rationale**: Clarification selected hard failures for any cross-tenant leakage,
unclassified failure, restart recovery over 5 minutes, retry exhaustion without
operator-action-needed state, queue backlog persisting over 30 minutes, or monotonic
resource growth over the full run. These thresholds are concrete enough to drive tests,
report validation, and ship/no-ship decisions.

**Alternatives considered**:

- Narrative-only report: rejected by the upstream roadmap.
- Defer thresholds to implementation: rejected because it would leave acceptance tests
  ambiguous.
- Stricter one-minute recovery and 10-minute backlog thresholds: rejected as potentially
  brittle for the first single-node 24-hour baseline.

## Decision: Fake-backend coverage is mandatory; real-account smoke is opt-in with skip evidence

**Rationale**: Fake backends provide repeatable CI-style coverage for fault drills. Real
accounts are valuable but depend on safe operator-provided credentials. Missing safe
credentials should not block Roadmap 39 if fake-backend coverage passes and the skip is
explicitly recorded by integration domain and reason.

**Alternatives considered**:

- Missing real credentials block readiness: rejected because it makes roadmap closure
  dependent on external accounts rather than product correctness.
- Real-account smoke is informational only: rejected because where safe credentials exist,
  the smoke result should be release evidence.

## Decision: Generate machine-readable soak and readiness evidence where practical

**Rationale**: Release gates and long-running soak outcomes need repeatable validation.
Even if the first implementation stores reports as JSON fixtures or markdown tables, the
required fields and thresholds must be contract-tested so a narrative report cannot pass
accidentally.

**Alternatives considered**:

- Markdown-only runbook outputs: rejected because threshold validation and future Roadmap
  40/41 rerun gates would be hard to enforce.
- Public API-first report storage: deferred until implementation proves operator or client
  surfaces require it.
