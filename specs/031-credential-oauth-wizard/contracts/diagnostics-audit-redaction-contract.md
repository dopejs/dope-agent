# Diagnostics, Audit, And Redaction Contract

## Diagnostic Linkage

Every ready, degraded, action-required, or unavailable setup session must link to a
diagnostic result or an explicit unsupported/action-required classification.

Diagnostic linkage fields:
- `setupSessionId`
- `targetId`
- `diagnosticResultId`
- `diagnosticRunId`
- `status`
- `reasonCode`
- `retrySafety`
- `remediationOwner`
- `allowedCapabilities` when degraded
- `checkedAt`
- `staleAfter`
- `redactionStatus`

## Reason Code Mapping

| Setup reason | Diagnostic source |
|--------------|-------------------|
| `credential_missing` | tenant secret missing or disabled |
| `scope_missing` | integration diagnostic `scope_missing` |
| `tenant_approval_pending` | integration diagnostic `tenant_approval_pending` |
| `token_missing` | provider/integration token missing |
| `token_expired` | provider/integration token expired |
| `tenant_mismatch` | OAuth callback or diagnostic tenant mismatch |
| `provider_unavailable` | diagnostic provider unavailable |
| `network_failed` | diagnostic network failed |
| `rate_limited` | diagnostic rate limited |
| `unsupported_target` | target catalog unsupported classification |
| `redaction_failed_closed` | redaction failure |

## Audit Events

Setup audit event families:
- `credential_setup.started`
- `credential_setup.secret_submitted`
- `credential_setup.oauth_started`
- `credential_setup.oauth_completed`
- `credential_setup.diagnostic_completed`
- `credential_setup.action_required`
- `credential_setup.unavailable`
- `credential_setup.ready`
- `credential_setup.degraded`
- `credential_setup.cancelled`
- `credential_setup.disabled`
- `credential_setup.retried`
- `credential_setup.replaced`
- `credential_setup.redaction_failed_closed`

Audit document fields:
- `setupSessionId`
- `tenantId`
- `principalId`
- `targetId`
- `targetKind`
- `setupStyle`
- `operation`
- `fromState`
- `toState`
- `reasonCode`
- `retryable`
- `remediationOwner`
- `safeUseMode`
- `diagnosticResultId`
- `resourceRefs`
- `redactionStatus`

Audit records must be tenant-scoped and metadata-only.

## Forbidden Evidence

These values must never appear in setup state, diagnostics, audit records, events, logs,
fixtures, reports, SDK objects, or rendered UI:
- raw submitted secret values
- OAuth authorization codes
- access tokens
- refresh tokens
- provider tokens
- provider client secrets
- callback payloads
- authorization headers
- credential-bearing request or response bodies
- local CLI auth material
- derived credential material

## Redaction Fail-Closed

If evidence cannot be proven safe:
1. Set redaction status to `failed_closed`.
2. Transition setup to `action_required` or `unavailable`.
3. Block ready state.
4. Emit `credential_setup.redaction_failed_closed`.
5. Show operator remediation without the unsafe evidence.

## Operator Diagnostics

Operator diagnostics should include setup findings when setup is action-required,
unavailable, degraded, disabled, or redaction failed closed.

Finding fields:
- source kind: `credential_setup`
- source id: setup session id
- plane: `readiness`
- status: setup state
- reason: setup reason code
- recommended action: derived from remediation owner
- detail route: setup diagnostics route
- related tenant and target refs

## Retention

Setup sessions and attempts remain available while they are current or needed for
support review. If retention is added, expiry must apply only to redacted evidence and
must not delete tenant secrets, provider auth state, integration records, or audit
history required by existing policies.

## Tests

- Audit tests assert event families, tenant scope, setup state, and metadata-only
  documents.
- Redaction tests seed fake secret/OAuth strings and assert absence across setup,
  diagnostics, audit, event, fixture, log, SDK, and UI surfaces.
- Operator diagnostic tests assert setup findings appear with route, reason, remediation
  owner, and no credential material.
- Degraded diagnostic tests assert allowed limited-safe capabilities are present before
  dependent use is allowed.
