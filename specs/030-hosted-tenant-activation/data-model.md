# Data Model: Hosted Signup And Tenant Activation

## Hosted User

Represents an authenticated hosted principal attempting activation.

**Fields**

- `principalId`: stable principal identity from the tenant identity foundation.
- `status`: principal lifecycle status; activation is allowed only when active.
- `defaultTenantId`: principal default tenant after personal tenant resolution.
- `tokenId`: authenticated token used for the activation request.

**Validation Rules**

- Principal status must not be `disabled` or denied by token, grant, or membership state.
- Hosted activation must not create a second personal tenant for the same principal.

## Personal Tenant

Represents the tenant activated for the hosted user's personal use.

**Fields**

- `tenantId`: stable tenant identifier.
- `tenantKind`: must be `personal`.
- `displayName`: user-visible tenant label.
- `status`: must be active for activation to proceed.
- `defaultOwnerPrincipalId`: hosted user principal that owns the personal tenant.
- `createdAt`, `updatedAt`: lifecycle timestamps.

**Relationships**

- One hosted user has one default personal tenant.
- A personal tenant has active membership for the hosted user.
- Organization tenants may also be available but are not required for personal activation.

**Validation Rules**

- Activation must resolve an existing personal tenant before attempting to create one.
- Concurrent activation attempts must converge on one active personal tenant.

## Activation State

Represents first-run activation progress for one hosted user and personal tenant.

**Fields**

- `activationId`: stable activation record id.
- `principalId`: hosted user principal.
- `tenantId`: active personal tenant.
- `environmentScope`: `test` or `prod`.
- `status`: one of `not_started`, `in_progress`, `blocked`, `active`,
  `first_action_completed`.
- `currentStepId`: current first-run step, such as `resolve_personal_tenant`,
  `quota_baseline`, or `test_chat`.
- `completedStepIds`: completed steps.
- `blockingReasonCodes`: stable reason codes currently blocking completion.
- `readinessItems`: activation readiness checks.
- `quotaBaseline`: plan and quota projection when available.
- `firstAction`: required v1 action summary for `test_chat`.
- `firstActionCompletedAt`: set only after test chat succeeds.
- `createdAt`, `updatedAt`: lifecycle timestamps.

**Relationships**

- Belongs to one hosted user and one personal tenant.
- References quota baseline for the same tenant.
- References test chat metadata after completion or failure.
- Produces activation audit records for state transitions.

**State Transitions**

```text
not_started -> in_progress -> blocked
not_started -> in_progress -> active
blocked -> in_progress -> active
active -> first_action_completed
first_action_completed -> first_action_completed
```

**Validation Rules**

- `first_action_completed` requires an active personal tenant, quota baseline readiness,
  and successful test chat metadata.
- Missing quota baseline forces `blocked` and includes a retryable quota readiness reason.
- Disabled or denied principal state forces `blocked` or denied response and must not
  create or update an activation completion state.
- Test chat transcript and message content must not be stored in this record.

## Readiness Check

Represents one activation prerequisite.

**Fields**

- `itemId`: stable readiness id.
- `itemKind`: `tenant_access`, `environment`, `quota_baseline`, `test_chat`, or
  extension value.
- `status`: `ready`, `blocked`, `degraded`, `missing_configuration`, or `optional`.
- `reasonCode`: stable machine-readable reason when not ready.
- `displayName`: user-visible label.
- `requiredForActivation`: boolean.
- `retryable`: boolean.
- `remediationOwner`: `product_user`, `operator`, `tenant_admin`, `system`, or
  `none_required`.
- `updatedAt`: timestamp.

**Validation Rules**

- Quota baseline readiness is required for activation completion.
- Live connectors and provider setup readiness are optional follow-ups for Roadmap 45.
- Required readiness blockers must appear in activation diagnostics.

## Quota Baseline

Represents the default plan and quota projection for the personal tenant.

**Fields**

- `tenantId`: personal tenant.
- `planKey`: active plan key.
- `enforcementMode`: billing enforcement mode.
- `quotas`: list of effective quota projections.
- `projectedAt`: timestamp.
- `status`: `available` or `unavailable`.
- `reasonCode`: set when unavailable.

**Validation Rules**

- Activation completion is blocked while quota baseline status is unavailable.
- Unknown quota must not be shown as capacity.
- Projection belongs to the active personal tenant only.

## Safe First Action

Represents the required v1 test chat action.

**Fields**

- `actionId`: `test_chat`.
- `actionKind`: `test_chat`.
- `displayName`: user-visible label.
- `recommended`: true for v1 activation.
- `available`: true only when required readiness checks pass.
- `blockingItemIds`: readiness blockers when unavailable.
- `invokeRoute`: activation test chat route.
- `resultRoute`: route for activation state after completion.

**Validation Rules**

- Test chat must not require live connectors, production secrets, payment checkout, or
  organization setup.
- Test chat must run under the active personal tenant.
- Completion metadata must not contain test chat transcript or message content.

## Test Chat Metadata

Represents the metadata retained after the test chat first action.

**Fields**

- `activationId`: activation record.
- `tenantId`: personal tenant.
- `dispatchId`: chat/dispatch identifier if available.
- `status`: `completed`, `failed`, or `cancelled`.
- `provider`: provider identifier when available.
- `model`: model identifier when available.
- `usage`: usage summary when available and safe to expose.
- `finishReason`: terminal reason when available.
- `reasonCode`: stable failure reason when failed.
- `completedAt`: completion timestamp.

**Validation Rules**

- Must not contain user query text, reply text, streamed deltas, transcript, prompts, or
  raw provider payloads.
- Must be sufficient for audit and diagnostics to prove completion or failure stage.

## Activation Failure Reason

Represents a stable activation failure class.

**Fields**

- `reasonCode`: stable machine-readable code.
- `stage`: `tenant_resolution`, `eligibility`, `quota_baseline`, `authorization`,
  `test_chat`, `audit`, `persistence`, or `unexpected`.
- `retryable`: boolean.
- `remediationOwner`: `product_user`, `operator`, `tenant_admin`, `system`, or
  `none_required`.
- `message`: user-safe message.

**Validation Rules**

- Clients must not parse raw error text.
- Denials must not reveal inaccessible tenant details.
- Reason codes must remain stable across SDK and web shell tests.

## Activation Audit Record

Represents tenant-scoped audit evidence for activation changes.

**Fields**

- `auditEventId`: audit identifier.
- `eventKind`: activation event kind.
- `tenantId`: personal tenant when resolved.
- `principalId`: hosted user principal.
- `tokenId`: authenticated token when available.
- `activationId`: activation record id.
- `outcome`: `succeeded`, `failed`, or `denied`.
- `reasonCode`: stable reason.
- `document`: metadata-only transition details.
- `createdAt`: timestamp.

**Validation Rules**

- Security-relevant activation state changes must be audit-visible.
- Audit document must not retain raw secrets, credential-bearing values, test chat
  transcripts, or test chat message content.
