# Real Mail Provider Closure

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 63, the first real
mail provider closure after the fake mail backend.

Primary source documents:
- `docs/specs/015-mail-integration.md`
- `docs/specs/031-hosted-credential-and-oauth-setup-wizard.md`
- `docs/specs/027-integration-health-and-permission-diagnostics.md`
- `docs/specs/044-external-integration-adapter-plane.md`

## Background

The mail domain has production-shaped draft, send, reply, forward, operation, artifact,
and delivery truth backed by fake provider verification. Public parity requires one real
mail provider with OAuth, scope diagnostics, safe sends, and real-account smoke evidence.

## Goal

Close one real mail provider end to end for hosted use while preserving the existing mail
operation model.

## Fixed Decisions

- Provider choice must be recorded during speckit clarification.
- The real provider MUST be implemented as an adapter on the external integration adapter
  plane (Roadmap 59), not as an in-process backend. The mail operation ledger stays
  daemon-owned per that plane's fixed decisions.
- Real mail sends remain side effects requiring explicit permission and evidence.
- Fake backend coverage remains mandatory.
- Full attachment transfer remains a separate roadmap.

## Dependencies On Completed Phases

- Roadmap 30: Mail Integration
- Roadmap 46: Hosted Credential And OAuth Setup Wizard
- Roadmap 42: Integration Health And Permission Diagnostics
- Roadmap 59: External Integration Adapter Plane (prerequisite; build before this roadmap)

## In Scope

- one real mail provider backend
- account, thread, message, and draft projection
- draft create/update
- direct send, send-existing-draft, reply, and forward
- scope, token, provider, and rate-limit diagnostics
- real-account smoke with safe test mailbox or structured skip

## Out Of Scope

- full attachment upload/download
- CRM automation
- campaign tooling
- memory-based recipient inference

## Operator Or User Problems To Solve

- Users need mail actions to work against a real mailbox.
- Operators need provider failures and ambiguous sends to be diagnosable.

## User Stories

- As a user, I can connect a real mailbox and inspect messages.
- As a user, I can draft and send mail with explicit recipients.
- As a release reviewer, I can inspect mail real-account smoke evidence.

## Functional Requirements

- The provider backend MUST map provider responses to existing mail resources.
- Send operations MUST preserve side-effect evidence, idempotency keys, and ambiguity
  classification.
- Provider auth and scope failures MUST map to diagnostics.
- Real-account smoke MUST avoid leaking message content beyond redacted evidence policy.

## Compatibility And Operational Notes

The fake backend remains the deterministic verification baseline. Provider-specific
behavior must not create a second mail execution plane.

## Verification Expectations

- Provider adapter tests for account, thread, message, draft, send, reply, and forward.
- Live validation classification for provider send outcomes.
- API/workflow tests proving provider path uses existing mail operation truth.
- Manual real-account smoke where safe mailbox credentials are available.

## Definition Of Done

- One real mail provider is hosted-ready for the phase-30 mail capability set.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/048-real-mail-provider-closure.md 完成 phase 63 的工作`
