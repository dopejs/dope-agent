# Hosted Signup And Tenant Activation

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 45, the hosted
user activation slice for public-product readiness before knowledge-plane work begins.

Primary source documents:
- `docs/specs/019-tenant-identity-and-access-foundation.md`
- `docs/specs/021-tenant-aware-operator-shell-and-sdk.md`
- `docs/specs/028-hosted-operational-profile-and-recovery.md`
- `docs/runtime/operator-trust-model.md`

## Background

DopeAgent has tenant identity, token grants, hosted operation, and an operator shell. A
public hosted product still needs a first-run path where an unfamiliar user can activate a
personal tenant, understand the active environment, and complete one useful action without
manual API calls.

## Goal

Provide a hosted activation flow that creates or selects the user's personal tenant,
confirms access, initializes safe defaults, and leads the user to a first useful action.

## Fixed Decisions

- Activation is product behavior, not a developer runbook.
- Every hosted user starts with a personal tenant.
- Activation must not require live connectors or production secrets.
- Organization onboarding remains additive and must not block personal activation.
- This roadmap does not introduce memory, context, or personalized recall.

## Dependencies On Completed Phases

- Roadmap 34: Tenant Identity And Access Foundation
- Roadmap 36: Tenant-Aware Operator Shell And SDK
- Roadmap 43: Hosted Operational Profile And Recovery

## In Scope

- hosted signup or invitation acceptance surface
- personal tenant activation state
- first-run environment and readiness checks
- default quota and plan projection for new users
- first useful action selection, such as test chat, reminder, or provider setup
- SDK and web shell support for activation state

## Out Of Scope

- enterprise SSO
- payment checkout
- organization administration suite
- memory-based personalization

## Operator Or User Problems To Solve

- New users need to know whether their account is active and what to do first.
- Operators need activation failures to be diagnosable without database inspection.

## User Stories

- As a new hosted user, I can sign in or accept an invite and land in my personal tenant.
- As a new hosted user, I can complete one safe first action without configuring secrets.
- As an operator, I can see why activation failed.

## Functional Requirements

- The system MUST expose activation state for the resolved user and tenant.
- The system MUST create or resolve a default personal tenant for eligible new users.
- The web shell MUST show active tenant, environment, quota baseline, and next activation
  actions.
- Activation failures MUST use stable reason codes.
- Activation MUST be tenant-scoped and audited.

## Compatibility And Operational Notes

Existing token and tenant APIs remain compatible. Activation adds a guided projection and
workflow on top of existing identity primitives.

## Verification Expectations

- API, SDK, and web tests for first-run activation states.
- Tenant isolation tests for activation projections.
- Restart test proving activation state survives daemon restart.
- Manual `DOPE_ENV=test` walkthrough from no active setup to first useful action.

## Definition Of Done

- A new hosted user can activate a personal tenant and complete a safe first action from
  the product surface.
- Activation failures are inspectable and auditable.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/030-hosted-signup-and-tenant-activation.md 完成 phase 45 的工作`
