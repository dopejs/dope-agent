# Hosted Billing Quotas

Roadmap 38 introduces a tenant-scoped billing control plane for hosted quota inspection,
administration, and durable usage accounting.

## Runtime Model

Billing state is stored in SQLite under `billing_*` tables:

- tenant plans define `enforcementMode` as `enforced`, `unlimited`, or `not_measurable`
- quota definitions provide the canonical catalog and stable denial reason codes
- quota periods reset on UTC boundaries
- counters track committed, reserved, adjusted, and carryover amounts
- reservations are idempotent by tenant, category, and operation key
- denials, manual adjustments, recovery decisions, and usage events are durable evidence

Local-first tenants without an active finite plan project an explicit development plan
with `enforcementMode = unlimited`. Hosted enforcement must fail closed when billing state
is unavailable.

## Inspection

Tenant owners and administrators with `billing.view` can call:

- `GET /v1/billing/plan`
- `GET /v1/billing/usage`
- `GET /v1/billing/quotas`
- `GET /v1/billing/denials`
- `GET /v1/billing/quota-dashboard`
- `GET /v1/billing/denials/{denialId}`

Responses are scoped to the resolved tenant context. Billing schemas keep `tenantId`
additive for legacy-client compatibility.

`quota-dashboard` is a user-readable projection over the Roadmap 38 catalog. It includes
every enforced category, readable sections, current active period usage, immediately
previous completed period usage when available, finite/unlimited/not-measurable state,
near-limit warnings, base versus effective limits, visible overrides, and explicit abuse
restriction summaries. Near-limit is true at 80% consumed or when remaining allowance is
less than one category-defined typical operation. Count and attempt categories use `1`;
artifact byte quotas use the catalog's artifact write reservation estimate.

## Administration

Owners and administrators with `billing.manage` can call:

- `POST /v1/admin/billing/tenants/{tenantId}/plan`
- `POST /v1/admin/billing/tenants/{tenantId}/quota-overrides`
- `POST /v1/admin/billing/tenants/{tenantId}/manual-adjustments`
- `POST /v1/admin/billing/tenants/{tenantId}/reservations/{reservationId}/resolve`

Every mutation requires an operator reason. Lowered quota overrides apply immediately to
effective projection and do not rewrite existing usage evidence. Manual adjustments are
rejected when they would make effective usage negative.

## Denials And Recovery

Stable quota denials use `code = quota_denied` plus a category-specific `reasonCode`.
Ambiguous pending reservations are marked `operator_action_needed` during restart
recovery; duplicate work for the same operation key is denied until an operator resolves
the reservation.

Denial detail classifies records as `quota_exhaustion`, `abuse_restriction`,
`quota_state_unavailable`, `unauthorized`, or `operator_action_needed`. Detail responses
show safe operation references, guarded entry points, measured amount, remaining amount,
and recovery actions such as wait, reduce scope, request override, contact support, or
operator resolution. Abuse restriction visibility is intentionally limited to explicit
restriction status, affected category, visible reason, duration when available, support
contact state, and source audit reference; detection signals, thresholds, raw trigger
events, and evasion-relevant patterns are not exposed.

Support operators with explicit `billing.evidence_export` can call:

- `POST /v1/billing/denials/{denialId}/evidence-export`

The export is structured redacted JSON for an associated denial. It includes schema
version, export id, denial detail, usage snapshot, effective limit state, audit
references, and redaction records. It excludes secrets, connector payloads, unrelated run
content, and cross-tenant data. `billing.view` alone is insufficient for export.

For local operator verification, start the test daemon and run
`scripts/phase47-public-quota-walkthrough.sh`. The script seeds only
`~/.kura-test/daemon.sqlite`, then validates quota dashboard projection, ordinary and
abuse denial detail, evidence export redactions, cross-tenant hiding, and unauthorized
no-partial-data behavior.

## Audit And Retention

Billing audit evidence records actor, tenant, category, operation key, amount, reason,
and outcome. Default billing audit retention is indefinite unless an explicit retention
policy is configured.

## Matrix Maintenance

New hosted entry points that start work, invoke integrations, write artifacts, or create
evaluation attempts must either reuse an existing quota category from the enforcement
matrix or add a new catalog row, stable denial reason code, operation key shape, schema
fixture, SDK surface, and targeted allowed/denied/retry/restart/concurrency tests before
shipping.

## Rollback

Storage rollback is backup-restore. Logical enforcement rollback should assign affected
tenants an explicit `unlimited` or development plan rather than deleting usage or audit
records. Product rollback for Roadmap 47 can hide the new SDK/web surfaces and disable the
new projection/export routes while leaving Roadmap 38 enforcement intact.
