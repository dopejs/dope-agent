# Contract: Activation API

## Contract Risks

- Activation state is a new long-lived client contract; fields must be additive after
  release.
- Activation must not mutate existing tenant, billing, onboarding, or chat response
  semantics in incompatible ways.
- Test chat execution may return user-visible chat content to the current request, but
  persisted activation state, audit, diagnostics, fixtures, and logs must retain metadata
  only.
- Repeated and concurrent activation calls must be idempotent.

## Protected Routes

All routes are protected operator APIs and resolve tenant/principal context through the
existing authentication and tenant access boundary.

### `GET /v1/activation`

Returns the current activation projection for the authenticated hosted user and resolved
personal tenant.

**Response 200**

```json
{
  "activation": {
    "activationId": "act_123",
    "principalId": "prn_user",
    "tenantId": "ten_personal",
    "environmentScope": "test",
    "status": "active",
    "currentStepId": "test_chat",
    "completedStepIds": ["tenant_resolved", "quota_baseline_ready"],
    "blockingReasonCodes": [],
    "readinessItems": [],
    "quotaBaseline": {
      "tenantId": "ten_personal",
      "planKey": "free",
      "enforcementMode": "enforced",
      "status": "available",
      "quotas": []
    },
    "firstAction": {
      "actionId": "test_chat",
      "actionKind": "test_chat",
      "recommended": true,
      "available": true,
      "blockingItemIds": [],
      "invokeRoute": "/v1/activation/test-chat",
      "resultRoute": "/v1/activation"
    },
    "lastEvaluatedAt": "2026-05-06T00:00:00Z"
  }
}
```

### `POST /v1/activation`

Creates or resolves the personal tenant and activation state for the authenticated hosted
user. The operation is idempotent for the same principal and personal tenant.

**Request**

```json
{
  "source": "signup"
}
```

`source` is optional and may be `signup`, `invite_acceptance`, `returning_user`, or
`operator_retry`.

**Response 200**

Same shape as `GET /v1/activation`.

### `POST /v1/activation/test-chat`

Runs the required v1 test chat first action under the active personal tenant after
required readiness checks pass.

**Request**

```json
{
  "message": "Run a safe hosted activation test."
}
```

The request message is used only for the current action and must not be persisted in
activation state, audit, diagnostics, fixtures, or logs.

**Response 200**

```json
{
  "activation": {
    "activationId": "act_123",
    "principalId": "prn_user",
    "tenantId": "ten_personal",
    "environmentScope": "test",
    "status": "first_action_completed",
    "currentStepId": "completed",
    "completedStepIds": ["tenant_resolved", "quota_baseline_ready", "test_chat_completed"],
    "blockingReasonCodes": [],
    "readinessItems": [],
    "quotaBaseline": {
      "tenantId": "ten_personal",
      "planKey": "free",
      "enforcementMode": "enforced",
      "status": "available",
      "quotas": []
    },
    "firstAction": {
      "actionId": "test_chat",
      "actionKind": "test_chat",
      "recommended": true,
      "available": true,
      "blockingItemIds": [],
      "invokeRoute": "/v1/activation/test-chat",
      "resultRoute": "/v1/activation"
    },
    "lastEvaluatedAt": "2026-05-06T00:00:00Z"
  },
  "testChat": {
    "dispatchId": "dispatch_123",
    "status": "completed",
    "provider": "test",
    "model": "test-chat",
    "finishReason": "stop",
    "usage": {},
    "completedAt": "2026-05-06T00:00:00Z"
  }
}
```

`testChat` contains metadata only. It must not include `query`, `reply`, transcript,
streamed deltas, prompts, raw provider payloads, or credential-bearing values.

### `GET /v1/activation/diagnostics`

Returns operator-facing activation diagnostics for the current resolved tenant context.
This may also be projected into existing operator diagnostics, but the activation contract
must provide a stable metadata-only diagnostic shape.

**Response 200**

```json
{
  "items": [
    {
      "activationId": "act_123",
      "tenantId": "ten_personal",
      "principalId": "prn_user",
      "status": "blocked",
      "stage": "quota_baseline",
      "reasonCode": "activation_blocked:quota_baseline_unavailable",
      "retryable": true,
      "remediationOwner": "operator",
      "lastTransitionAt": "2026-05-06T00:00:00Z",
      "readinessItemIds": ["quota-baseline"],
      "quotaBaselineStatus": "unavailable"
    }
  ]
}
```

## Status And Reason Codes

### Activation Status

- `not_started`
- `in_progress`
- `blocked`
- `active`
- `first_action_completed`

### Stable Reason Codes

- `activation_denied:principal_disabled`
- `activation_denied:principal_denied`
- `activation_denied:tenant_access_revoked`
- `activation_blocked:quota_baseline_unavailable`
- `activation_blocked:environment_unavailable`
- `activation_blocked:test_chat_unavailable`
- `activation_failed:tenant_resolution`
- `activation_failed:test_chat`
- `activation_failed:audit_write`
- `activation_failed:persistence`
- `activation_failed:unexpected`

## Compatibility Assessment

- New routes and schemas are additive.
- Existing `/v1/auth/me`, `/v1/tenants`, `/v1/billing/*`, `/v1/operator/onboarding`, and
  `/v1/chat/query` contracts remain compatible.
- New schema files must be added under `schemas/api/` and covered by
  `make daemon-contract-test`.
- SDK types must make new response fields explicit and preserve existing tenant option
  behavior.

## Required Tests

- New hosted user creates/resolves a personal tenant and activation state.
- Returning user resolves existing personal tenant without duplicates.
- Concurrent activation attempts converge to one personal tenant and one activation state.
- Disabled or denied principal receives stable denial and does not complete activation.
- Missing quota baseline returns blocked activation state and retryable quota reason.
- Test chat completes activation only after quota baseline readiness passes.
- Test chat metadata excludes query, reply, transcript, deltas, prompts, and raw provider
  payloads.
- Activation state survives daemon restart.
- Cross-tenant requests cannot read or mutate another tenant's activation state.
