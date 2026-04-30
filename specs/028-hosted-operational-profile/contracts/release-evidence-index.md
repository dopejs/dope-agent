# Contract: Release Evidence Index

## Goal

Define the single hosted operational evidence index used for 30-minute ship/no-ship
review.

## Required Identity Fields

| Field | Required |
|-------|----------|
| `releaseIndexId` | yes |
| `runId` | yes |
| `profileId` | yes |
| `commitOrVersion` | yes |
| `generatedAt` | yes |
| `reviewTarget` | yes |
| `retentionExpiresAt` | yes |
| `decision` | yes |
| `reviewElapsedSeconds` | yes |

All linked evidence must match `runId`, `profileId`, and `commitOrVersion`.

## Required Evidence Links

| Evidence | No-Ship Rule |
|----------|--------------|
| Deployment manifest | Missing or mismatched blocks readiness |
| Configuration profile | Missing or mismatched blocks readiness |
| Health checks | Missing, failed, or mismatched blocks readiness |
| Logs | Missing, inaccessible, or suspected secret exposure blocks readiness |
| Soak report | Missing, failed, stale, or mismatched blocks readiness |
| Backup evidence | Missing, failed, incompatible, or mismatched blocks readiness |
| Restore evidence | Missing, failed, active-target, or mismatched blocks readiness |
| Upgrade preflight | Missing, failed, or mismatched blocks readiness |
| Upgrade postflight | Missing, failed, or mismatched blocks readiness |
| Rollback decision | Missing or ambiguous rollback path blocks readiness |
| Integration diagnostics | Missing, stale, failed, or mismatched blocks readiness |
| Resource observations | Missing, failed, or unsupported without marker blocks readiness |
| Redaction check | Missing or failed blocks readiness |
| Retention metadata | Missing or expired evidence blocks normal readiness |

## Decisions

Allowed decisions:

- `ship`
- `no_ship`
- `ship_with_recorded_skips`

`ship_with_recorded_skips` is allowed only when fake-backend and operational evidence
passes and the only skips are explicitly recorded live/real-account smoke skips inherited
from existing readiness policy.

## No-Ship Rules

The release decision is `no_ship` when:

- required evidence is missing
- evidence is stale or expired for normal inspection
- evidence is mismatched to reviewed commit, profile, or run identity
- health, recovery, backup, restore, upgrade, rollback, diagnostic, observability, or
  redaction checks fail
- crash or reboot recovery exceeds 5 minutes
- restore evidence does not prove at least three representative tenants
- release reviewer cannot reach a defensible decision in 30 minutes or less from the
  index

## 90-Day Retention Rules

- Default retention is 90 days.
- Authorized longer retention must be explicit in the index.
- Expired evidence is unavailable for normal readiness review.

## Contract Tests

- Fixture file: `daemon/internal/opsreadiness/testdata/hosted/release_index.json`
- Implemented command: `scripts/production/hosted-profile.sh evidence-index`
- Implemented validator: `go run ./cmd/hosted-evidence-validate <release-evidence-index.json>`
- passing fixture links all required evidence with matching identity
- missing evidence fixture produces `no_ship`
- mismatched commit/profile/run fixture produces `no_ship`
- expired evidence fixture produces `no_ship` unless authorized retention applies
- redaction failure fixture produces `no_ship`
- review fixture is organized so required evidence can be inspected within 30 minutes
- generated indexes are validated by the Go validator; `--allow-no-ship` permits
  structurally valid no-ship indexes while default mode requires ship readiness
