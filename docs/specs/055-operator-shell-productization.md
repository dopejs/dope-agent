# Operator Shell Productization

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 70, the operator
shell productization slice for public beta readiness.

Primary source documents:
- `docs/specs/017-operator-shell-and-onboarding.md`
- `docs/specs/021-tenant-aware-operator-shell-and-sdk.md`
- `docs/specs/030-hosted-signup-and-tenant-activation.md`
- `docs/specs/038-channel-management-and-repair-ux.md`
- `docs/specs/051-routine-builder.md`

## Background

The web shell exposes strong engineering projections. A public product needs a cohesive
user control surface for setup, channels, sessions, profiles, routines, providers,
diagnostics, quota, and support evidence.

## Goal

Turn the operator shell from an engineering dashboard into a product control console for
hosted users and support operators.

## Fixed Decisions

- The shell remains a client of daemon APIs and must not own runtime truth.
- Product UX must expose authoritative details without requiring SQLite or logs.
- TUI parity is not required in this roadmap unless explicitly clarified.
- This roadmap does not add memory UX.

## Dependencies On Completed Phases

- Roadmaps 45-68 as product surfaces become available

## In Scope

- information architecture for setup, channels, sessions, profiles, routines, providers,
  quota, diagnostics, evaluation, and support
- first-run and repair task flows
- authoritative detail drawers or pages
- event-driven refresh for active surfaces
- empty, error, denied, and unsupported states
- web test coverage for critical flows

## Out Of Scope

- native mobile app
- full TUI parity
- marketing landing page
- memory or knowledge UI

## Operator Or User Problems To Solve

- Users need one place to configure and trust the hosted agent.
- Support needs product-native inspection rather than asking users for raw logs.

## User Stories

- As a user, I can complete setup and manage channels, providers, profiles, routines, and
  quota from one shell.
- As support, I can navigate from a failure to authoritative evidence.

## Functional Requirements

- The shell MUST organize all public non-knowledge surfaces into coherent navigation.
- The shell MUST preserve tenant selection across views.
- Critical actions MUST show permission, approval, quota, and side-effect expectations.
- Every failure view MUST expose stable reason and next step where available.
- The shell MUST not bypass daemon APIs.

## Compatibility And Operational Notes

This roadmap should consolidate UI surfaces without broad backend refactors. Existing API
contracts remain authoritative.

## Verification Expectations

- Web tests for setup, channel repair, session reset, profile binding, routine creation,
  provider diagnostics, quota denial, and support evidence navigation.
- SDK build and tests.
- Manual hosted `KURA_ENV=test` walkthrough for public beta happy path and one repair path.

## Definition Of Done

- A hosted user can operate the non-knowledge personal-agent product from the web shell
  without developer guidance.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/055-operator-shell-productization.md 完成 phase 70 的工作`
