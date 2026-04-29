# Contract: Release Readiness Gate

This contract defines the release evidence and ship/no-ship rules for Roadmap 39 and the
future Roadmaps 40 and 41 rerun gate.

## Required Evidence

Release readiness requires:

| Evidence | Blocking Rule |
|----------|---------------|
| Install runbook result | Missing or failed clean install blocks readiness |
| Upgrade runbook result | Missing or failed representative upgrade blocks readiness |
| Backup artifact evidence | Missing backup contents/integrity evidence blocks readiness |
| Restore verification result | Missing or failed restore validation blocks readiness |
| Migration verification report | Missing preflight or postflight evidence blocks readiness |
| Rollback guidance | Missing restore-from-backup decision path blocks readiness |
| Soak report | Missing, incomplete, or failed soak report blocks readiness |
| Real-account smoke status | Missing safe credentials do not block if fake-backend coverage passes and skip reason is recorded |
| Resource-growth checks | Missing required observations or failed thresholds block readiness |
| Credential redaction checks | Any raw credential exposure blocks readiness |
| Future rerun gate | Missing Roadmaps 40/41 rerun requirement blocks readiness |

## Real-Account Smoke Policy

For each supported integration domain:

- if safe credentials are available, run opt-in real-account smoke and record the result
- if safe credentials are unavailable, expired, revoked, or unsafe, record an explicit
  skip reason
- fake-backend coverage remains mandatory regardless of real-account availability
- real-account smoke output must not expose credential material

Missing safe credentials do not block readiness when fake-backend coverage passes and the
skip reason is recorded.

## Ship/No-Ship Rules

The release decision is `no_ship` when:

- any required evidence is missing
- install or upgrade verification fails
- backup/restore validation fails
- migration preflight or postflight checks fail
- rollback guidance cannot identify a safe recovery path
- soak hard-fail thresholds are hit
- credential material is exposed
- fake-backend fault coverage is incomplete
- Roadmaps 40/41 rerun requirement is absent

The release decision may be `ship_with_recorded_skips` only when:

- all fake-backend and operational evidence passes
- only real-account smoke is skipped
- every skipped real-account domain has an explicit reason

## Roadmaps 40 And 41 Rerun Gate

Any final release that includes Roadmap 40 live side-effect validation or Roadmap 41
evaluation-product expansion must rerun the Roadmap 39 soak harness after those changes
land.

The rerun must use the same hard-fail thresholds unless a later spec explicitly tightens
them. A pre-Roadmap-40/41 soak result is not sufficient final release evidence.

## Contract Tests

Required tests or checks:

- readiness fails when required evidence is absent
- readiness fails when soak report hard-fails
- readiness fails when credential redaction fails
- readiness passes with recorded real-account skips only when fake-backend coverage passes
- readiness contract explicitly names the Roadmaps 40/41 rerun gate

Final implementation fixtures:

- `specs/024-production-ops-soak/fixtures/release-readiness.passing.json`
- `specs/024-production-ops-soak/fixtures/release-readiness.failures.json`
