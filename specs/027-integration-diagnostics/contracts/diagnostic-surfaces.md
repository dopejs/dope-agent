# Contract: Diagnostic Surfaces

## Goal

Expose integration diagnostic truth through daemon-owned API resources, SDK types, web
operator projections, and user-facing remediation payloads without changing existing
integration execution routes.

## Permissions

| Permission | Purpose | Default Role Guidance |
|------------|---------|-----------------------|
| `integrations.diagnostics.read` | Inspect latest and historical diagnostic state for a tenant integration. | Tenant operator/admin. |
| `integrations.diagnostics.run` | Start or refresh a diagnostic run. | Tenant operator/admin with integration management authority. |
| `integrations.diagnostics.smoke` | Run safe real-account smoke probes. | Authorized operator with explicit tenant context. |
| `integrations.diagnostics.smoke_risky` | Request non-idempotent or externally visible probes. | Requires both tenant administrator approval and authorized operator approval. |

Permission tests must prove viewers cannot run diagnostics, unauthorized users cannot
learn whether inaccessible tenants or integrations exist, and risky smoke probes are
blocked without both required approvals.

## API Routes

### `GET /v1/integrations/{integrationId}/diagnostics`

- Purpose: inspect the latest diagnostic results for one integration account.
- Query parameters:
  - `capability`
  - `includeStale`
  - `limit`
  - `cursor`
- Response:
  - integration identity summary
  - diagnostic result resources
  - freshness summary
  - unsupported or limited classification when full diagnostics do not exist

### `POST /v1/integrations/{integrationId}/diagnostics/runs`

- Purpose: start or refresh diagnostics for one integration account.
- Request body:
  - `capabilities`
  - `forceRefresh`
  - `clientKey` for idempotency
  - optional `reason`
- Behavior:
  - resolves explicit tenant context
  - enforces `integrations.diagnostics.run`
  - uses cached state for inspection only when not forcing refresh
  - marks cached state stale after 15 minutes
  - redacts before persistence or fails closed
- Response:
  - diagnostic run resource
  - result resources when available
  - blocked reason when the run cannot proceed

### `GET /v1/integration-diagnostics/runs`

- Purpose: list tenant-scoped diagnostic runs.
- Query parameters:
  - `integrationId`
  - `providerKind`
  - `domainKind`
  - `status`
  - `reasonCode`
  - `limit`
  - `cursor`
- Response:
  - ordered run resources
  - deterministic cursor pagination

### `GET /v1/integration-diagnostics/runs/{runId}`

- Purpose: inspect one diagnostic run.
- Response:
  - diagnostic run resource
  - checked capabilities
  - result transitions
  - redaction status
  - retention expiry
  - audit evidence references

### `GET /v1/integration-diagnostics/reason-codes`

- Purpose: expose the stable reason-code catalog to SDK and web clients.
- Response:
  - reason code
  - category
  - default remediation owner
  - default retry safety
  - user-facing message key
  - operator-facing message key

### User-Facing Failure Projection

Existing domain action responses that expose integration failures must be able to include
a diagnostic failure projection:

- `reasonCode`
- `remediationOwner`
- `remediationHint`
- `retrySafety`
- `diagnosticResultId` where visible
- `currentDiagnosticTruth`: `true`

This projection must never include raw provider error text or credential-bearing detail.

## Resource Fields

Diagnostic result resources include:

- `diagnosticResultId`
- `tenantId`
- `integrationId`
- `integrationAccountId`
- `domainKind`
- `providerKind`
- `capability`
- `status`
- `reasonCode`
- `remediationOwner`
- `remediationHint`
- `retrySafety`
- `checkedAt`
- `staleAfter`
- `freshnessState`
- `redactionStatus`
- `evidenceSummary`
- `retentionExpiresAt`

Diagnostic run resources include:

- `diagnosticRunId`
- `tenantId`
- `integrationId`
- `status`
- `trigger`
- `requestedBy`
- `startedAt`
- `completedAt`
- `checkedCapabilities`
- `resultIds`
- `failureReasonCode`
- `redactionStatus`
- `retentionExpiresAt`

## SDK And Web Contract

The TypeScript SDK must expose typed resources and methods for:

- listing integration diagnostics,
- starting diagnostic runs,
- listing and inspecting diagnostic runs,
- listing reason codes,
- receiving user-facing diagnostic failure projections.

The web operator shell must expose enough UI to:

- inspect latest diagnostic state and freshness,
- distinguish healthy, blocked, degraded, limited, unsupported, and stale states,
- run diagnostics when authorized,
- see remediation owner and next step,
- see redaction-failure safe classifications,
- follow smoke and release-readiness evidence links where available.

## Compatibility Rules

- Existing integration routes and execution behavior remain backward compatible.
- Diagnostic projections are additive fields or additive routes.
- Existing `/v1/operator/diagnostics` can continue to exist as a broader operational
  surface, but Roadmap 42 integration diagnostics require typed integration-specific
  resources.
- Cached results may be shown for operator inspection, but stale state must be explicit
  after 15 minutes.
- User-facing integration failures must use current diagnostic truth before remediation
  is presented.
