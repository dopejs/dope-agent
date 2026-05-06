# Integration Health And Permission Diagnostics

Status: implementation and local verification complete in
`specs/027-integration-diagnostics/`; stable-host or real-account release evidence remains
pending unless current evidence is linked from release readiness.

Authority: This document is the authoritative upstream spec for Roadmap 42, the
integration reliability diagnostics layer for real external systems.

Primary source documents:
- `docs/product/hosted-productization-roadmap-split.md`
- `docs/specs/012-personal-integrations-platform.md`
- `docs/specs/022-hosted-secrets-integrations-and-connector-isolation.md`
- `docs/specs/024-production-install-upgrade-backup-and-soak.md`
- `docs/specs/025-live-validation-and-side-effect-replay.md`
- `docs/runtime/release-readiness.md`
- `docs/runtime/release-truth-checklist.md`
- `specs/027-integration-diagnostics/quickstart.md`

## Background

DopeAgent can model personal integrations, tenant-owned credentials, live validation, and
evaluation workflows. The remaining reliability gap is that real external systems fail in
provider-specific ways: missing application scopes, user OAuth not granted, tenant app
approval pending, expired tokens, rate limits, provider outages, network failures, and
ambiguous downstream commits.

These failures must become product-visible diagnostic truth instead of ad hoc operator
debugging. The first proof domain is Feishu/Lark because it exercises bot permissions,
user OAuth, tenant approval, scopes, calendar/task/document APIs, and CLI-backed operator
workflows.

## Goal

Make integration health, authorization, provider error classification, and operator
remediation inspectable through stable product surfaces and repeatable smoke evidence.

## Fixed Decisions

- Provider-specific errors must map to stable system-level reason codes.
- Health checks and permission diagnostics are product behavior, not only test scripts.
- Diagnostics must distinguish bot/app authorization, user OAuth authorization, tenant
  approval, provider scopes, token freshness, network reachability, provider availability,
  quota/rate limits, and tenant mismatch.
- Feishu/Lark is the first proof domain, but the diagnostic model must be reusable by
  calendar, mail, reminders/tasks, delivery, connectors, MCP, and provider auth states.
- Diagnostics must not expose raw secrets, OAuth tokens, app secrets, or credential
  material.
- Real-account smoke may be skipped only with explicit structured skip reasons.

## Dependencies On Completed Phases

- Roadmap 27: Personal Integrations Platform
- Roadmap 29: Calendar Integration
- Roadmap 30: Mail Integration
- Roadmap 31: Tasks And Reminders
- Roadmap 37: Hosted Secrets, Integrations, And Connector Isolation
- Roadmap 38: Billing, Quotas, And Usage Accounting
- Roadmap 39: Production Install, Upgrade, Backup, And Soak
- Roadmap 40: Live Validation And Side-Effect Replay
- Roadmap 41: Evaluation Product Expansion

## In Scope

- integration health and permission diagnostic resource shape
- stable diagnostic reason codes and remediation hints
- provider-specific error classification adapters, starting with Feishu/Lark
- bot/app and user OAuth diagnostic separation
- tenant approval and scope-missing detection where the provider exposes enough evidence
- token expired, token missing, token revoked, and refresh failure classification
- network failure, provider unavailable, transient failure, and rate-limit classification
- ambiguous commit and unsafe retry classification for side-effecting integration actions
- SDK and operator-shell projections for diagnostic state
- structured real-account smoke reports with pass, fail, blocked, and skipped outcomes
- audit events for diagnostic runs and remediation-relevant state transitions

## Out Of Scope

- adding new integration domains
- automatic external writes solely for diagnostics
- bypassing provider approval or tenant administrator controls
- external marketplace packaging
- memory or context engineering
- autonomous remediation without operator approval

## Operator Or User Problems To Solve

- Operators need to know whether an integration failed because the app lacks a scope, the
  user has not authorized OAuth, the tenant has not approved the app, the token expired,
  or the provider is unavailable.
- Product users need clear next steps instead of raw provider error text.
- Engineers need real-account smoke evidence that can be repeated without leaking secrets
  or creating uncontrolled external side effects.
- Release reviewers need to see which domains passed real-account smoke and which were
  skipped for explicit reasons.

## User Stories

- As an operator, I can inspect a Feishu/Lark integration and see whether bot scopes, user
  OAuth, tenant approval, and token freshness are healthy.
- As a product user, I can see a stable remediation message when a calendar action fails
  because tenant administrator approval is required.
- As an engineer, I can run a real-account smoke matrix and receive a structured report
  with pass, fail, blocked, skipped, and remediation fields.
- As a release reviewer, I can confirm that provider failures are classified into
  retryable, operator-action-needed, permission-admin-required, auth-remediation-required,
  unsafe-to-retry, provider-unavailable, or network-failed categories.

## Functional Requirements

- The system MUST expose integration diagnostic results with stable reason codes.
- The system MUST distinguish app or bot credential health from user OAuth health.
- The system MUST distinguish missing provider scope from tenant app approval pending when
  provider evidence permits that distinction.
- The system MUST classify token expiry, revocation, missing refresh credentials, and
  refresh failure separately.
- The system MUST classify transient provider failures, rate limits, provider outages, and
  local network failures separately from permission failures.
- The system MUST classify side-effecting provider failures as retryable, unsafe to retry,
  or operator-action-needed based on idempotency and commit evidence.
- The system MUST redact raw secrets, app secrets, OAuth tokens, refresh tokens, provider
  authorization headers, and credential-bearing request payloads from diagnostics,
  reports, logs, events, fixtures, and evaluation artifacts.
- The system MUST produce structured smoke reports for real-account checks, including
  domain, tenant, integration account, probe action, result, reason code, remediation hint,
  timestamp, and artifact links.
- Real-account smoke MUST support explicit skip reasons, including missing safe
  credentials, unsafe side-effect scope, tenant approval unavailable, provider outage, and
  operator-deferred.
- Diagnostic state MUST be tenant-scoped and permission-gated.
- Diagnostic runs MUST be auditable without leaking inaccessible tenant existence or
  credential material.

## Compatibility And Operational Notes

- Existing integration APIs remain compatible; diagnostics are additive.
- Provider raw errors may be preserved only in redacted diagnostic details where they do
  not reveal secret material.
- Fake-backend coverage remains mandatory even when real-account smoke passes.
- Real-account smoke must default to read-only or reversible probes. Non-idempotent
  provider mutations require explicit operator-selected scope and approval.
- Diagnostic classifications should be reusable by live validation, delivery, campaigns,
  and release-readiness reports.
- Release-truth classification treats remaining stable-host or real-account smoke as a
  release evidence gap rather than an implementation gap.

## Verification Expectations

- Unit tests for provider error classification and stable reason-code mapping.
- Feishu/Lark diagnostic tests covering app scope missing, user OAuth missing, tenant
  approval pending, token expired, rate limit, provider unavailable, network failure, and
  successful readiness where fixtures can model provider responses.
- Tenant isolation tests proving diagnostic results and remediation hints do not leak
  cross-tenant integration existence.
- Redaction tests proving diagnostics, logs, events, reports, and evaluation artifacts do
  not include raw credential material.
- API, schema, SDK, and web tests for diagnostic resource shapes and permission denials.
- Real-account smoke report fixture covering passed, failed, blocked, and skipped domains.
- Manual `DOPE_ENV=test` smoke for at least one Feishu/Lark diagnostic path when safe
  credentials and tenant approval are available.
- Release-truth checklist review classifies missing safe credentials, tenant approval, or
  operator-deferred smoke as explicit residual release work.

## Definition Of Done

- Feishu/Lark integration failures can be diagnosed without relying on prior chat context
  or manual provider-console archaeology.
- Every supported integration domain has a structured diagnostic or a deliberate not-yet-
  supported classification.
- Real-account smoke produces structured evidence with explicit pass/fail/blocked/skip
  states.
- Operator-facing diagnostics explain the reason, remediation owner, retry safety, and
  next step without exposing secrets.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/027-integration-health-and-permission-diagnostics.md 完成 phase 42 的工作`
