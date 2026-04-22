# Data Model: Personal Integrations Platform

## Entities

### Integration Resource

- Purpose: Operator-visible record for one account-backed personal-system connection.
- Fields:
  - `integrationId`
  - `domainKind`: `calendar`, `mail`, `tasks`, `files`, or other future personal domain
  - `displayName`
  - `environmentScope`
  - `readinessStatus`: `not_configured`, `auth_pending`, `healthy`, `degraded`, or
    `unavailable`
  - `authState`: `not_started`, `pending`, `authorized`, `expired`, `revoked`, or
    `not_applicable`
  - `healthState`: `unknown`, `healthy`, `degraded`, or `unavailable`
  - `readinessReason`
  - `requiredOperatorAction`
  - `canonicalDefault`
  - `accountBinding`
  - `backendBinding`
  - `secretProvenanceSummary`
  - `createdAt`
  - `updatedAt`
  - `lastReadyAt`
  - `lastTransitionAt`
- Validation rules:
  - every integration belongs to exactly one environment scope
  - multiple integration records may share the same domain/account/environment group
  - exactly one integration in a domain/account/environment group may have
    `canonicalDefault=true` at a time
  - `unavailable` is terminal for execution until readiness changes
  - `degraded` remains visible and does not by itself imply blanket execution denial

### Account Binding

- Purpose: Durable association between an integration resource and the external account it
  represents.
- Fields:
  - `accountKey`
  - `accountLabel`
  - `accountType`
  - `tenantLabel`
  - `externalAccountId`
  - `knownAfterAuth`: boolean
- Validation rules:
  - `accountKey` must be stable enough to group duplicate integration records once known
  - pre-auth integrations may omit final external identity but must not claim canonical
    default for an unknown account group if a known binding already exists in the same
    domain and environment

### Backend Binding

- Purpose: Inspectable summary of how the integration is implemented.
- Fields:
  - `backendKind`: `mcp`, `managed_provider`, `native`, or `fake_local`
  - `backendRefId`
  - `backendDisplayName`
  - `sourceKind`
  - `supportsInteractiveAuth`
  - `supportsProbeRead`
  - `supportsProbeMutation`
- Validation rules:
  - backend binding is operator-visible but does not leak raw credentials or secret values
  - different backend kinds must converge on the same integration readiness model

### Integration Binding Summary

- Purpose: Snapshot attached to runtime, workflow, and approval truth showing which
  integration was used when work executed.
- Fields:
  - `integrationId`
  - `domainKind`
  - `displayName`
  - `accountKey`
  - `canonicalDefault`
  - `readinessAtInvocation`
  - `backendKind`
  - `secretResolution`
  - `environmentScope`
  - `capturedAt`
- Validation rules:
  - binding summaries are immutable snapshots once attached to a tool call, workflow step,
    or approval
  - summaries use redacted secret-resolution outcomes only
  - one tool call may reference multiple integration bindings in future phases, though the
    fake backend verification path needs only one

### Fake Integration Probe

- Purpose: Deterministic verification-only operation against a fake integration backend so
  roadmap 27 can prove runtime, approval, and provenance reuse without domain logic.
- Fields:
  - `probeKind`: `inspect` or `mutate`
  - `integrationId`
  - `runId`
  - `stepId`
  - `toolCallId`
  - `approvalId`
  - `status`
  - `resultSummary`
  - `failureClass`
- Validation rules:
  - probe execution uses normal runtime step and tool-call truth
  - `inspect` is read-only and should not require approval unless policy elevates it
  - `mutate` is side-effecting in verification terms and should exercise the shared
    approval path

## State Transitions

### Integration Readiness Lifecycle

- `not_configured` -> `auth_pending` when an integration has been registered and is
  waiting on account authorization or secret material
- `not_configured` -> `healthy` when a local or fake backend becomes immediately ready
- `auth_pending` -> `healthy` when authorization completes and the backend is usable
- `healthy` -> `degraded` when partial health or auth issues exist but some operations may
  remain safe
- `healthy` or `degraded` -> `unavailable` when the backend, required secret scope, or
  account access is no longer usable
- `unavailable` or `degraded` -> `healthy` when readiness is restored
- any state -> `not_configured` when the operator removes required binding material

### Canonical Default Selection

- a new integration in an existing domain/account/environment group may be promoted to
  canonical default
- when one integration is promoted, all sibling records in that group lose canonical
  default status atomically
- if the canonical default becomes unavailable or is removed, a sibling does not become
  default implicitly; operator-visible selection remains explicit

## Relationships

- one account binding may be referenced by many integration resources over time
- one backend binding belongs to exactly one integration resource
- one integration resource may appear in many runtime tool calls, workflow steps, or
  approvals through immutable binding summaries
- one fake integration probe belongs to one run and one integration resource and links to
  one normal runtime tool call

## Derived Views

- integration list views can group resources by `domainKind/accountKey/environmentScope`
  and highlight the canonical default
- integration detail views expand auth context, health context, readiness reason, backend
  binding, and redacted secret provenance
- runtime tool-call, workflow-step, and approval views can project the integration
  binding snapshot without re-reading live integration state
