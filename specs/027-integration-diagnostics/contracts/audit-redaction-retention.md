# Contract: Audit, Redaction, And Retention

## Redaction Invariant

The following material MUST NOT appear in diagnostics, API responses, SDK fixtures, web
fixtures, smoke reports, audit events, logs, evaluation artifacts, release-readiness
artifacts, or test failure output:

- raw secret values
- OAuth authorization codes
- OAuth access tokens
- OAuth refresh tokens
- app secrets
- provider tokens
- authorization headers
- local CLI auth material
- credential-bearing request payloads
- credential-bearing response payloads

Allowed diagnostic fields include:

- `tenantId`
- `integrationId`
- `integrationAccountId`
- `domainKind`
- `providerKind`
- `capability`
- `reasonCode`
- `remediationOwner`
- `retrySafety`
- `redactionStatus`
- `retentionExpiresAt`
- redacted provider code or request id when non-secret

## Fail-Closed Rule

If evidence cannot be confidently redacted:

- suppress diagnostic detail,
- emit redaction-failure audit evidence,
- show only a generic safe classification,
- do not persist the raw detail,
- do not export the raw detail into fixtures or release evidence.

## Audit Event Families

Roadmap 42 audit evidence must cover:

- `integration_diagnostic.run_started`
- `integration_diagnostic.run_completed`
- `integration_diagnostic.run_failed`
- `integration_diagnostic.state_changed`
- `integration_diagnostic.permission_denied`
- `integration_diagnostic.redaction_failed_closed`
- `integration_diagnostic.smoke_started`
- `integration_diagnostic.smoke_completed`
- `integration_diagnostic.smoke_blocked`
- `integration_diagnostic.smoke_skipped`
- `integration_diagnostic.retention_applied`

Event names may be adapted to existing event naming conventions, but each semantic
action above must have a stable event or audit record and contract fixture.

## Required Audit Fields

Every diagnostic audit record includes:

- `eventId` or audit id,
- `tenantId`,
- actor or principal id where available,
- `action`,
- `targetKind`,
- `targetId`,
- `outcome`,
- `reasonCode`,
- timestamp,
- redaction status,
- evidence references when safe.

Permission denials must not disclose inaccessible tenant existence, integration account
existence, credential state, or provider account labels.

## Retention

Diagnostic runs and smoke report evidence use a 90-day default retention period unless an
authorized longer retention policy applies.

Retention behavior:

- active evidence is available through normal inspection,
- expired evidence is removed from normal inspection,
- minimal audit records remain available to authorized operators,
- retention application emits audit evidence,
- release-readiness artifacts must point to retained evidence or record that the evidence
  expired under policy.

## Contract Tests

Required tests:

- redaction fixtures fail if credential material appears anywhere in diagnostic,
  smoke, audit, log, fixture, SDK, web, or readiness payloads,
- redaction-uncertain evidence fails closed,
- retention tests expire diagnostic runs and smoke reports from normal inspection after
  90 days,
- permission-denial tests do not leak inaccessible tenant or integration existence,
- audit fixtures cover run lifecycle, state transition, smoke publication, skipped and
  blocked outcomes, retention application, and redaction failures.
