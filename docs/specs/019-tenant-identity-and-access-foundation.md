# Tenant Identity And Access Foundation

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 34, the tenant
identity and access foundation required before hosted multi-tenant product work can land.

Primary source documents:
- `docs/product/hosted-productization-roadmap-split.md`
- `docs/runtime/daemon-roadmaps.md`
- `docs/runtime/operator-trust-model.md`

## Background

The current daemon is local-first and effectively single-operator. A hosted product needs a
stable tenant boundary before resources can be migrated, exposed in clients, or connected to
shared integrations.

## Goal

Introduce first-class tenants, principals, memberships, token tenant grants, tenant
resolution, and permission checks that every later hosted roadmap can depend on.

## Fixed Decisions

- Support both personal tenants and organization tenants.
- Every principal has one default tenant and an explicit allowed-tenant set.
- `X-Dope-Tenant-ID` may override the default tenant only when the principal is allowed to
  access that tenant.
- Requests resolve both `principalId` and `tenantId`.
- Tenant roles are `owner`, `admin`, `operator`, and `viewer`.
- Sensitive capabilities are permission-gated separately from broad role names.
- Initial permissions include `tenant.manage`, `secrets.manage`, `integrations.manage`,
  `connectors.manage`, `mcp.manage`, `runs.execute`, `approvals.resolve`,
  `live_validation.execute`, `evaluation.manage`, and `billing.view`.

## Dependencies On Completed Phases

- Roadmap 4: Operator Trust And Security
- Roadmap 32: Operator Shell And Onboarding
- Roadmap 33: Evaluation And Replay Harness

## In Scope

- tenant and principal resource model
- personal-tenant bootstrap for existing single-user installations
- organization tenant model
- membership records and role assignments
- token tenant grants and default tenant
- request tenant resolution middleware
- permission evaluation service
- audit events for tenant switching, denied access, and membership changes

## Out Of Scope

- migrating every existing domain table to tenant scope
- billing and quota enforcement
- per-tenant storage backends
- tenant switcher UI beyond minimal API exposure

## Operator Or User Problems To Solve

- Operators need a reliable answer to "which tenant owns this request?"
- Hosted administrators need to grant and revoke access without sharing global daemon
  authority.
- Later roadmaps need one authorization contract instead of per-domain tenant hacks.

## User Stories

- As a hosted user, I can use my personal tenant without choosing an organization first.
- As an organization owner, I can grant another user operator or viewer access.
- As an API client, I can select an allowed tenant explicitly and receive a stable error if
  the tenant is not allowed.

## Functional Requirements

- The system MUST persist tenant, principal, membership, and token tenant-grant records.
- The system MUST create or resolve a default personal tenant for existing local-first use.
- The system MUST reject requests for tenants outside the principal's allowed tenant set.
- The system MUST expose stable tenant and membership inspection APIs.
- The system MUST evaluate capability permissions through a shared service.
- The system MUST emit auditable events for membership changes and denied tenant access.

## Compatibility And Operational Notes

- Existing single-user local deployments must continue to work through an implicit default
  personal tenant.
- API changes should be additive where possible.
- Denial responses must not leak whether an inaccessible tenant exists.

## Verification Expectations

- API tests for tenant resolution, default tenant behavior, header override, and denial.
- Unit tests for role and permission evaluation.
- Contract fixtures for tenant and membership resources.
- Restart coverage proving tenant grants and memberships survive daemon restart.

## Definition Of Done

- All inbound requests have a resolved tenant context or a stable authorization error.
- Later roadmap specs can depend on a shared tenant and permission service.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/019-tenant-identity-and-access-foundation.md 完成 phase 34 的工作`
