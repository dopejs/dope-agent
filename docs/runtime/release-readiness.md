# Release Readiness Gate

Release readiness review must be completable in 30 minutes or less using the
produced evidence. Missing required evidence or failed thresholds produce a
no-ship decision.

Use [Release Truth Checklist](./release-truth-checklist.md) to classify roadmap closure,
residual release evidence gaps, and public-readiness claims before accepting ship-ready
status.

Required evidence:

- clean install result
- representative upgrade result
- backup artifact evidence
- restore verification result
- migration preflight and postflight report
- rollback guidance, including restore from backup when in-place rollback is
  unsafe
- 24-hour soak report
- resource-growth checks
- credential redaction checks
- real-account smoke result or explicit skip reason for every supported domain
- Roadmap 40 and Roadmap 41 rerun gate
- Roadmap 42 integration diagnostic latest-state and smoke evidence
- Roadmap 43 hosted release evidence index when hosted/test-host operation is
  in scope for the reviewed release

Safe real-account credentials are optional. Missing safe credentials do not
block release readiness when fake-backend coverage passes and every affected
integration domain records an explicit skip reason.

Roadmap 42 evidence must show, for each supported integration domain, whether
diagnostics passed, failed, were blocked, were skipped, were limited, or were
deliberately unsupported. Diagnostic runs and smoke reports use 90-day default
retention and must remain tenant-scoped and redacted. Risky smoke probes require
both tenant administrator and authorized operator approval.

## Hosted Operational Evidence

Hosted release readiness uses
`scripts/production/hosted-profile.sh evidence-index`. The command runs
`daemon/cmd/hosted-evidence-validate` after generating the index. The index must link the
deployment manifest, configuration profile, health checks, logs, soak report,
backup evidence, restore evidence, upgrade preflight, upgrade postflight,
rollback decision, integration diagnostics, resource observations, redaction
check, and retention metadata.

Every linked artifact must match the reviewed commit or version, hosted
profile, and run identity. Evidence expires after 90 days for normal inspection
unless an authorized longer-retention policy is recorded in the index. Missing,
stale, mismatched, failed, expired, or secret-exposing hosted evidence is a
no-ship condition. Reviewers must be able to reach a defensible decision from
the index in 30 minutes or less.

Any final release that includes Roadmap 40 live side-effect validation or
Roadmap 41 evaluation-product expansion must rerun the Roadmap 39 soak harness
after those changes land. A pre-Roadmap-40/41 soak result is not sufficient
final release evidence.

Roadmap 41 dashboard evidence must include the tenant id, projection id,
projection time window, generated timestamp, campaign status counts, drift and
failure totals, unsupported replay totals, live-validation linkage count, and
operator-action-needed count. A dashboard projection is acceptable only when it
links back to stored campaigns, campaign attempt groups, replay comparisons,
and Roadmap 40 ledger entries for inspection.

If dashboard reads are unavailable, pagination is nondeterministic, or linked
campaign/inspection evidence cannot be opened for the reviewed tenant, the
Roadmap 41 release-readiness gate is incomplete even if older replay harness
soak evidence passed.

## Active Roadmap 41 Evidence Closure

Roadmap 41 release evidence closure is complete for commit `5ad95ba`. The combined
Roadmap 39/40/41 24-hour rerun passed on stable host `zentalk-1` on 2026-05-01
Asia/Shanghai and met all
[`docs/harness/roadmap41-soak-acceptance-runbook.md`](../harness/roadmap41-soak-acceptance-runbook.md)
criteria.

Authoritative evidence links:

- Roadmap 41 implementation and verification summary:
  [`specs/026-evaluation-product-expansion/quickstart.md`](../../specs/026-evaluation-product-expansion/quickstart.md)
- Roadmap 41 task closure, including T153:
  [`specs/026-evaluation-product-expansion/tasks.md`](../../specs/026-evaluation-product-expansion/tasks.md)
- Upstream Roadmap 41 spec status:
  [`docs/specs/026-evaluation-product-expansion.md`](../specs/026-evaluation-product-expansion.md)
- Daemon roadmap closure:
  [`docs/runtime/daemon-roadmaps.md`](./daemon-roadmaps.md)

Do not mark a future release complete from targeted-validation evidence or from a
pre-Roadmap-40/41 soak. Any later release that changes Roadmap 40 live validation or
Roadmap 41 evaluation-product behavior must rerun the required soak and link fresh
evidence for the reviewed commit.

## Non-Knowledge Public Beta Launch Gate (Roadmap 72)

The public beta launch gate is codified as `opsreadiness.ValidateLaunchGate` over a
`LaunchGateEvidence` index (POST `/v1/release/launch-gate`). It is a no-ship gate: missing
required workload evidence, fewer than three channel entries, missing calendar/mail provider
entries, or unmet soak / support-bundle / redaction evidence each produce a specific no-ship
reason. Real-account smoke may be skipped only with a structured accepted reason.

Required exercised workloads: activation, setup, channels, sessions, profile binding, routines,
webhooks, quota denial, diagnostics, evaluation, live validation, support bundle, backup,
restore, upgrade, rollback.

Entry-gate rule: **context, knowledge, and memory work may begin only after non-knowledge parity
release evidence passes (LaunchDecision.result == "ship") or residual exceptions are explicitly
accepted.** With Roadmaps 44-72 (specs 044-057) landed, the non-knowledge personal-agent product
+ operations baseline is in place; the launch gate validates the release evidence index that
authorizes starting context/knowledge/memory design.
