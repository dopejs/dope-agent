# Contract: Managed Provider Sandbox Surfaces

## Scope

This contract defines the external and operator-visible surfaces that may change for the
managed-provider sandbox convergence slice. The contract is intentionally additive and must
remain backward-compatible.

## Affected Existing Surfaces

### Provider Auth Routes

- `GET /v1/providers/{providerId}/auth`
- `POST /v1/providers/{providerId}/auth/start`
- `POST /v1/providers/{providerId}/auth/complete`
- `POST /v1/providers/{providerId}/auth/refresh`
- `POST /v1/providers/{providerId}/auth/revoke`

Planned contract rules:

- `ProviderAuthStateResponse.auth.status` keeps the existing enum values.
- In this slice, additive provider-auth contract work is expected to surface through the
  shared auth-state response schema used by these routes, rather than through new
  route-specific payload shapes.
- If additional sandbox convergence detail is needed, it must be added through additive
  metadata rather than by breaking the existing auth status enum.
- Candidate additive `auth.metadata` keys for this slice:
  - `managedProviderAction`
  - `sandboxProfileId`
  - `sandboxDecision`
  - `failureClass`
  - `enforcementStrength`
  - `sensitiveStateClasses`

### Sandbox Inspection Routes

- `GET /v1/sandboxes/executions`
- `GET /v1/sandboxes/executions/{executionId}`
- `POST /v1/sandboxes/explain`

Planned contract rules:

- Existing routes remain in place; this slice adds no new top-level sandbox routes.
- Subprocess-backed managed-provider workflows may enrich `sandbox.Execution.metadata` and
  `sandbox.Result.backendMetadata` with managed-provider provenance and enforcement detail.
- Additive metadata must be schema-backed if it is returned through the API.
- In-scope additive API work for this slice is expected to land on the execution resource
  and result schemas returned by the existing inspection routes, without introducing a new
  list- or request-only schema branch.

### Event Surfaces

- `sandbox.execution_requested`
- `sandbox.execution_started`
- `sandbox.execution_completed`
- `sandbox.execution_failed`
- `sandbox.execution_cancelled`
- `sandbox.execution_denied`
- `provider.auth_started`
- `provider.auth_completed`
- `provider.auth_refreshed`
- `provider.auth_revoked`

Planned contract rules:

- Event names remain stable.
- If managed-provider provenance or failure detail is added in this slice, it must remain
  limited to the explicit provider-auth and sandbox execution lifecycle event schemas
  listed above, and it must be additive and schema-backed.
- Operator-visible payloads must not expose credential-bearing local state values.

## Failure Classification Contract

The implementation must preserve a clear distinction between:

- `policy_denied`
- `approval_required`
- `approval_rejected`
- `missing_local_state`
- `provider_auth_failed`
- `process_failed`
- `timeout`
- `cancelled`

Compatibility rule:

- Existing `status` enums stay stable.
- Additional classification may be exposed through metadata, result detail, or additive
  fields, but the implementation must not collapse sandbox policy failures into generic
  provider auth or provider process errors.

## Enforcement Strength Contract

Operator-visible surfaces must clearly reflect real enforcement strength.

- The current backend remains `subprocess`.
- Network control may remain policy-declared or preflight-validated rather than hard
  isolated.
- No payload, doc, or UI string introduced by this slice may imply Docker-, container-, or
  VM-grade guarantees.

## Non-Goals

- No new top-level provider or sandbox routes
- No generic secret-ref API
- No additional backend families
- No requirement that every provider-owned local file read or write become an independent
  sandbox execution resource
