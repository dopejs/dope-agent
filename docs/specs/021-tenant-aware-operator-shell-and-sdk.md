# Tenant-Aware Operator Shell And SDK

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 36, the client and
operator surfaces required for tenant-aware hosted use.

Primary source documents:
- `docs/product/hosted-productization-roadmap-split.md`
- `docs/specs/019-tenant-identity-and-access-foundation.md`
- `docs/specs/020-tenant-scoped-data-migration.md`
- `docs/specs/017-operator-shell-and-onboarding.md`

## Background

Tenant isolation is incomplete if users cannot understand or control which tenant they are
operating in. SDK consumers also need explicit tenant override support without manually
constructing headers everywhere.

## Goal

Make tenant selection, tenant membership, and tenant-scoped operator projections visible and
usable in the web shell and TypeScript SDK.

## Fixed Decisions

- The SDK supports a default tenant and per-request tenant override.
- The web shell includes a tenant switcher based on allowed tenants.
- Operator projections are tenant-scoped by default.
- Tenant management UI is minimal but production-shaped: membership inspection and role
  changes for authorized users.

## Dependencies On Completed Phases

- Roadmap 34: Tenant Identity And Access Foundation
- Roadmap 35: Tenant-Scoped Data Migration
- Roadmap 32: Operator Shell And Onboarding

## In Scope

- SDK tenant configuration and request override support
- tenant-aware generated or shared client types
- web tenant switcher
- tenant-scoped activity, diagnostics, approvals, evaluation, and onboarding projections
- tenant membership inspection and management surfaces
- denial and empty-state UX for unauthorized tenant access

## Out Of Scope

- billing UI beyond links or read-only quota placeholders
- full organization administration suite
- payment-provider checkout
- native mobile tenant switching

## Operator Or User Problems To Solve

- Users need to know which tenant they are acting in before launching runs or resolving
  approvals.
- SDK callers need a stable tenant contract that does not rely on ad hoc headers.
- Organization owners need enough UI to inspect and correct membership mistakes.

## User Stories

- As a user in multiple tenants, I can switch between my personal tenant and an organization
  tenant.
- As an SDK caller, I can set a default tenant and override it for one request.
- As an organization owner, I can inspect members and update roles.

## Functional Requirements

- The SDK MUST support default tenant configuration and per-call tenant override.
- The web shell MUST display the active tenant and allowed tenant list.
- Tenant-scoped views MUST refetch after tenant switch.
- The UI MUST present stable authorization errors without falling back to global data.
- Membership management MUST be permission-gated.

## Compatibility And Operational Notes

- Existing SDK usage without a tenant override should continue to use the server-resolved
  default tenant.
- Tenant switch must not leave stale data from the previous tenant in visible detail panes.

## Verification Expectations

- SDK tests for default tenant and override header behavior.
- Web tests for tenant switch, scoped projections, and denied tenant states.
- Contract tests where tenant list or membership response shapes are added.

## Definition Of Done

- A multi-tenant user can operate the product without losing track of the active tenant, and
  SDK clients can express tenant intent consistently.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/021-tenant-aware-operator-shell-and-sdk.md 完成 phase 36 的工作`
