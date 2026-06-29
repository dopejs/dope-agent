# Real Calendar Provider Closure

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 60, the first real
calendar provider closure after the fake calendar backend.

Primary source documents:
- `docs/specs/014-calendar-integration.md`
- `docs/specs/031-hosted-credential-and-oauth-setup-wizard.md`
- `docs/specs/027-integration-health-and-permission-diagnostics.md`
- `docs/specs/044-external-integration-adapter-plane.md`
- `docs/providers/real-account-smoke.md`

## Background

The calendar domain is implemented with a repo-owned fake backend and production-shaped
operation truth. Public parity requires at least one real provider path with OAuth,
scopes, diagnostics, smoke evidence, and safe side-effect behavior.

## Goal

Close one real calendar provider end to end for hosted use while preserving the existing
calendar operation model.

## Fixed Decisions

- Provider choice must be recorded during speckit clarification.
- The real provider MUST be implemented as an adapter on the external integration adapter
  plane (Roadmap 59), not as an in-process `Backend`. The operation ledger stays
  daemon-owned per that plane's fixed decisions.
- Real provider work must reuse calendar operation, integration readiness, diagnostics,
  live validation, and delivery truth.
- Fake backend coverage remains mandatory.
- This roadmap does not expand attendee, RSVP, recurrence, or all-day mutation semantics.

## Dependencies On Completed Phases

- Roadmap 29: Calendar Integration
- Roadmap 46: Hosted Credential And OAuth Setup Wizard
- Roadmap 42: Integration Health And Permission Diagnostics
- Roadmap 59: External Integration Adapter Plane (prerequisite; build before this roadmap)

## In Scope

- one real calendar provider backend
- OAuth or credential setup integration
- account projection and event inspection
- availability query
- timed single-event create, update, and cancel
- provider scope and token diagnostics
- real-account smoke matrix with safe credentials or structured skip

## Out Of Scope

- attendee invitation and RSVP semantics
- recurring-event mutation
- all-day event mutation
- travel planning or meeting summarization
- memory-driven meeting context

## Operator Or User Problems To Solve

- Users need calendar actions to work against their real account.
- Operators need provider failures to be diagnosed and safely replayed or skipped.

## User Stories

- As a user, I can connect a real calendar and inspect events.
- As a user, I can create, update, and cancel a timed event.
- As a release reviewer, I can inspect real-account smoke or explicit skip evidence.

## Functional Requirements

- The provider backend MUST map provider responses to existing calendar account, event,
  operation, and artifact resources.
- OAuth and scope failures MUST map to stable diagnostics.
- Side-effecting writes MUST preserve idempotency and ambiguous-commit evidence.
- Real-account smoke MUST avoid leaking tokens and credential material.

## Compatibility And Operational Notes

Existing fake backend tests remain required. Provider-specific behavior must not create a
second calendar execution ledger.

## Verification Expectations

- Provider unit tests with recorded or synthetic provider responses.
- API and workflow tests against fake and provider adapter boundaries.
- Live validation classification for provider write outcomes.
- Manual real-account smoke where safe credentials are available.

## Definition Of Done

- One real calendar provider is hosted-ready for the phase-29 calendar capability set.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/045-real-calendar-provider-closure.md 完成 phase 60 的工作`
