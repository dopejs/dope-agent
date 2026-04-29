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

Responses are scoped to the resolved tenant context. Billing schemas keep `tenantId`
additive for legacy-client compatibility.

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
records.
