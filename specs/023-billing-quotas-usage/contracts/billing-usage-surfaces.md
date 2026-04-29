# Contract: Billing And Usage Surfaces

This contract defines the stable daemon, SDK, event, and audit concepts required by
Roadmap 38. Concrete JSON Schema files under `schemas/api` and `schemas/events` must be
updated with equivalent shapes during implementation.

## API Resources

### Tenant Plan And Usage Inspection

```text
GET /v1/billing/plan
GET /v1/billing/usage
GET /v1/billing/quotas
GET /v1/billing/denials
```

Responses are tenant-scoped through the protected API tenant context. Tenant owners and
authorized operators can inspect the active tenant only. Cross-tenant access is denied
through the existing tenant guard.

Required response fields:

- `tenantId`
- `planKey`
- `enforcementMode`
- `quotas[]`
  - `category`
  - `unit`
  - `periodStart`
  - `periodEnd`
  - `periodAnchor = UTC`
  - `limit`
  - `consumedAmount`
  - `reservedAmount`
  - `adjustedAmount`
  - `carryoverApplied`
  - `remainingAmount`
  - `denialReasonCode`
- `manualAdjustments[]` where requested
- `denials[]` where requested

### Plan, Quota, And Adjustment Administration

```text
POST /v1/admin/billing/tenants/{tenantId}/plan
POST /v1/admin/billing/tenants/{tenantId}/quota-overrides
POST /v1/admin/billing/tenants/{tenantId}/manual-adjustments
POST /v1/admin/billing/tenants/{tenantId}/reservations/{reservationId}/resolve
```

Administration requires the canonical `billing.manage` permission and writes
audit-visible records. Owner and admin roles receive `billing.manage`; operator and viewer
roles do not. Manual adjustments and operator-action-needed reservation resolution require
a reason.

## Stable Denial Error

All quota denials return a machine-readable error:

```json
{
  "code": "quota_denied",
  "reasonCode": "quota_denied:runtime_tool_calls_exhausted",
  "tenantId": "ten_example",
  "category": "runtime_tool_calls",
  "operationKey": "tenant:ten_example:tool_call:run_1:step_1:req_2",
  "periodStart": "2026-04-28T00:00:00Z",
  "periodEnd": "2026-04-29T00:00:00Z",
  "requestedAmount": 1,
  "remainingAmount": 0,
  "message": "Quota exhausted for runtime tool calls."
}
```

Clients must branch on `code` and `reasonCode`, not on `message`.

## Events And Audit Records

Required event/audit kinds:

- `billing.plan_changed`
- `billing.quota_override_changed`
- `billing.usage_reserved`
- `billing.usage_committed`
- `billing.usage_refunded`
- `billing.usage_released`
- `billing.quota_denied`
- `billing.manual_adjustment_created`
- `billing.reservation_recovery_decided`
- `billing.retention_policy_changed` when explicit retention policy support is added

Required fields:

- `tenantId`
- `principalId` where available
- `category`
- `operationKey` where applicable
- `quotaPeriodStart`
- `quotaPeriodEnd`
- `amount`
- `reasonCode`
- `reason`
- `outcome`
- `createdAt`

Billing and usage audit records retain indefinitely unless an explicit operator retention
policy is applied.

## SDK Expectations

The TypeScript SDK must expose typed resources for:

- active plan inspection
- effective quota projection
- usage summary
- denial listing
- stable quota-denial error
- admin plan assignment
- quota override
- manual adjustment
- operator-action-needed reservation resolution

SDK tests must prove stable denial codes are surfaced without string parsing.

Implementation schemas and SDK resources must stay aligned with:

- `billing-plan.response.schema.json`
- `billing-usage.response.schema.json`
- `billing-quota-resource.schema.json`
- `billing-denial-resource.schema.json`
- `billing-reservation-resource.schema.json`
- `billing-manual-adjustment-resource.schema.json`
- `billing-plan-assignment.request.schema.json`
- `billing-quota-override.request.schema.json`
- `billing-manual-adjustment.request.schema.json`
- `billing-reservation-resolution.request.schema.json`

## Contract Tests

Contract tests must validate:

- schema conformance for plan, usage, quota, denial, reservation, commit, refund,
  adjustment, and audit/event payloads
- tenant scoping on inspection and administration surfaces
- fail-closed hosted denial shape when quota state is unavailable
- unlimited/development plan projection shape
- stable denial handling in SDK/client tests
