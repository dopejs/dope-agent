# Hosted Credential And OAuth Setup Wizard

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 46, the hosted
credential and provider authorization setup flow.

Primary source documents:
- `docs/specs/022-hosted-secrets-integrations-and-connector-isolation.md`
- `docs/specs/027-integration-health-and-permission-diagnostics.md`
- `docs/runtime/hosted-credential-isolation.md`
- `docs/providers/integration-diagnostics.md`

## Background

Hosted users cannot be expected to create integration resources, secret references, OAuth
bindings, and diagnostic probes through raw APIs. Public readiness requires a guided,
recoverable setup path for provider and channel credentials.

## Goal

Make hosted credential and OAuth setup self-service, tenant-scoped, diagnostic-backed, and
safe to retry.

## Fixed Decisions

- The setup wizard orchestrates existing secrets, integrations, provider auth, connectors,
  diagnostics, and readiness surfaces.
- Raw credential material must never be displayed after submission.
- Failed setup remains recoverable without deleting unrelated integration state.
- This roadmap does not add new provider domains.

## Dependencies On Completed Phases

- Roadmap 37: Hosted Secrets, Integrations, And Connector Isolation
- Roadmap 42: Integration Health And Permission Diagnostics
- Roadmap 45: Hosted Signup And Tenant Activation

## In Scope

- setup-session resource for credential or OAuth onboarding
- secret submission and redaction confirmation
- OAuth start, callback, and completion state where applicable
- diagnostic probe after setup
- retry, cancel, replace, and disable flows
- web shell wizard and SDK methods

## Out Of Scope

- external managed secret managers
- enterprise SSO
- new integration domains
- memory or context personalization

## Operator Or User Problems To Solve

- Users need to connect accounts without learning daemon resource models.
- Operators need setup failures to identify missing scope, tenant approval, token failure,
  or provider outage.

## User Stories

- As a user, I can connect a provider account and see whether it is ready.
- As a user, I can retry or replace failed credentials without leaking secrets.
- As an operator, I can inspect setup attempts and redacted diagnostics.

## Functional Requirements

- The system MUST persist setup attempts with tenant, actor, target provider, state,
  redacted evidence, and diagnostic linkage.
- The wizard MUST support submitted-secret and OAuth-style providers.
- Setup MUST end in ready, degraded, unavailable, cancelled, or action-required states.
- All setup states MUST be auditable and tenant-scoped.
- The UI MUST show remediation next steps from diagnostic reason codes.

## Compatibility And Operational Notes

Existing secret and integration APIs remain the source of truth. Setup sessions are an
operator-visible orchestration layer over those APIs.

## Verification Expectations

- API and store tests for setup lifecycle and restart recovery.
- Redaction tests for submitted credentials, OAuth tokens, and callback payloads.
- Diagnostic integration tests for healthy, missing-scope, and tenant-approval-needed
  outcomes.
- Web tests for setup, retry, cancel, and replacement flows.

## Definition Of Done

- A hosted user can connect or repair a provider account from the product UI without raw
  API calls.
- Failed credential setup produces safe, actionable diagnostic truth.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/031-hosted-credential-and-oauth-setup-wizard.md 完成 phase 46 的工作`
