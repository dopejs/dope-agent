# Hosted Secrets, Integrations, And Connector Isolation

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 37, the hosted
isolation layer for secrets, integrations, connectors, and MCP installs.

Primary source documents:
- `docs/product/hosted-productization-roadmap-split.md`
- `docs/specs/019-tenant-identity-and-access-foundation.md`
- `docs/specs/020-tenant-scoped-data-migration.md`
- `docs/specs/012-personal-integrations-platform.md`

## Background

A hosted personal agent handles credentials and external account bindings. Tenant-scoped
runtime records are not enough if secrets, OAuth state, connector configs, MCP installs, or
sandbox policies can bleed across tenants.

## Goal

Make hosted secrets, integration accounts, connector configuration, MCP installs, provider
auth state, and sandbox policy records tenant-owned and permission-gated.

## Fixed Decisions

- Secrets are tenant-scoped operator-owned material.
- Secret values must not be exposed through API responses, logs, events, replay fixtures, or
  evaluation artifacts.
- Integration account ownership is tenant-local even when the same human user joins multiple
  tenants.
- MCP server installs and connector configs are tenant resources, not global daemon truth.
- Sandbox profiles that reference tenant secrets must resolve through tenant scope.

## Dependencies On Completed Phases

- Roadmap 34: Tenant Identity And Access Foundation
- Roadmap 35: Tenant-Scoped Data Migration
- Roadmap 27: Personal Integrations Platform
- Roadmap 21: Complete MCP Runtime And Catalog

## In Scope

- tenant-scoped secret records and redaction policy enforcement
- tenant-owned integration account bindings and provider auth state
- tenant-owned connector configuration and MCP install state
- permission checks for secret, integration, connector, and MCP administration
- audit events for secret reference use, integration connect/disconnect, and connector or
  MCP config changes
- tests for cross-tenant secret and integration isolation

## Boundary With Tenant-Scoped Data Migration

Roadmap 35 must already classify and tenant-scope persisted rows for integrations,
connectors, MCP resources, sandbox policies, and provider auth state where those records
exist. This roadmap builds on that ownership and is responsible for the hosted credential
semantics:

- secret values, secret references, and redaction policy
- provider auth state lifecycle, expiry, disconnect, and rotation behavior
- runtime credential resolution through the active tenant only
- connector, MCP, and sandbox policy mutation permissions
- audit events for secret reference use and credential-bearing configuration changes
- proof that logs, events, replay fixtures, evaluation artifacts, and API responses never
  contain raw secret material

The implementation plan MUST include a handoff table for every shared resource with:

- resource or table name
- Roadmap 35 tenant-ownership status
- Roadmap 37 credential or admin behavior to implement
- permission required for read, mutate, connect, disconnect, rotate, or invoke
- redaction expectations for API, events, logs, replay fixtures, and evaluation artifacts
- cross-tenant misuse test case

## Out Of Scope

- external enterprise secret-manager integrations unless explicitly selected later
- cross-tenant shared service accounts
- marketplace distribution
- billing enforcement

## Operator Or User Problems To Solve

- Organization data must not be accessible through a personal tenant credential path.
- Hosted operators need confidence that logs and replay artifacts do not leak secrets.
- Tenant admins need inspectable connector and MCP installation state.

## User Stories

- As a tenant admin, I can add an integration account that only my tenant can use.
- As an operator, I can inspect which tenant owns an MCP server install.
- As a viewer, I cannot read or mutate tenant secrets.

## Functional Requirements

- Secret references MUST resolve only inside the active tenant.
- Integration account and provider auth state MUST be tenant-owned.
- Connector and MCP administration MUST require tenant permissions.
- API responses, events, fixtures, and replay artifacts MUST redact secret values.
- Cross-tenant attempts to use secret references MUST fail with stable errors.

## Compatibility And Operational Notes

- Existing local secret configuration should migrate or bridge into the default personal
  tenant without printing values.
- Operators need a documented path for rotating tenant secrets.

## Verification Expectations

- Secret redaction contract tests.
- Cross-tenant integration and MCP install isolation tests.
- Handoff table verification proving every shared integration, connector, MCP, provider
  auth, sandbox policy, and secret-bearing resource has an owner in either Roadmap 35 or
  Roadmap 37.
- Permission-denial tests for viewer and operator roles.
- Manual `DOPE_ENV=test` smoke for tenant-scoped fake integration configuration.

## Definition Of Done

- Hosted credentials and external account bindings cannot be read, used, or mutated outside
  their owning tenant through normal product paths.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/022-hosted-secrets-integrations-and-connector-isolation.md 完成 phase 37 的工作`
