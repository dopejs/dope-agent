# Contract Surfaces: Redaction And Credential Audit

## Redaction Invariant

The following material MUST NOT appear in API responses, UI-visible data, events, logs,
replay fixtures, evaluation artifacts, diagnostics, contract fixtures, or test failure
output:

- raw secret values
- OAuth authorization codes
- access tokens
- refresh tokens
- provider tokens
- local CLI auth material
- derived credential material

Allowed redacted fields:

- `tenantId`
- `resourceKind`
- `resourceId` when it is not a secret
- `secretRef`
- `secretVersionId`
- `resolution`
- `status`
- `disabledReason`
- `remediationReason`
- `redactionRule`

## Audit Event Families

Audit-visible records must be emitted for:

- secret create/update metadata/rotate/disable
- successful secret reference use
- integration connect and disconnect
- provider authorization lifecycle changes
- connector configuration changes
- MCP install, lifecycle, and exposure changes
- sandbox policy changes involving secrets
- denied cross-tenant credential attempts
- failed-closed audit writes where supported by existing tenant audit behavior

## Successful Runtime Secret Use

Successful runtime secret use emits exactly one audit-visible record per:

- credential-bearing run
- connector invocation
- MCP invocation
- sandbox preparation

Repeated internal secret resolutions inside the same work item must not emit additional
successful-use audit records. Admin changes and denied attempts still emit separate audit
records.

## Required Audit Fields

Every credential audit record must include:

- acting `tenantId`
- `principalId` where available
- `resourceKind`
- `action`
- `outcome`
- `reasonCode`
- timestamp

Records may include redacted `resourceId`, `secretRef`, `secretVersionId`, and counts.
Records must not include another tenant's secret details or any raw credential material.

## Event/Schema Surfaces

Expected schema work:

- add credential audit event schema(s) under `schemas/events/`, or extend
  `tenant-audit-event-resource.schema.json` if the existing tenant audit surface is the
  canonical resource
- update `schemas/api/tenant-audit-event-resource.schema.json` and
  `schemas/api/tenant-audit-event-list.response.schema.json` only with redacted fields
- keep compatibility with `audit-cross-tenant-access-denied.event.schema.json` from
  Roadmap 35; R37 credential denials may reuse that event family only if the payload stays
  redacted and semantically clear

## Contract Tests

Required contract tests:

- redaction fixtures containing fake secret values must fail if those values appear in
  any API response, event payload, replay fixture, evaluation artifact, log capture, or
  diagnostic output
- successful-use audit granularity emits one event per work item
- denied cross-tenant attempts emit stable denied audit records without target tenant
  secret details
- audit write failures fail closed or surface existing tenant-audit-failed behavior
  without exposing secret material
