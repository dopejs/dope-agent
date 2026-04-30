# Release Readiness Gate

Release readiness review must be completable in 30 minutes or less using the
produced evidence. Missing required evidence or failed thresholds produce a
no-ship decision.

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

Safe real-account credentials are optional. Missing safe credentials do not
block release readiness when fake-backend coverage passes and every affected
integration domain records an explicit skip reason.

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
