# Hosted Productization Roadmap Split

Status: proposed

Authority: This document is the roadmap-splitting authority for the post-Roadmap 33 work
needed to make DopeAgent credible as a hosted, multi-tenant, production personal-agent
product before context management, memory, and self-improvement become the main focus.

Primary source documents:
- `docs/runtime/daemon-roadmaps.md`
- `docs/harness/harness-architecture.md`
- `docs/specs/018-evaluation-and-replay-harness.md`
- `docs/runtime/operator-trust-model.md`
- `docs/runtime/p0-release-review.md`

## Background

Roadmap 33 closed the first evaluation and replay harness slice, but it deliberately left
several product-grade gaps out of scope: live side-effect validation, automatic historical
run candidate discovery, in-product fixture editing, full tool-call replay, and real
user-side soak. Separately, the current local-first trust model is not sufficient for a
hosted product with multiple users, organizations, shared resources, quota enforcement, and
auditable administrative boundaries.

## Goal

Define the next roadmap split that turns the local-first personal agent into a hosted,
tenant-aware product surface with production validation and expanded evaluation/replay
capability.

## Fixed Decisions

- This work is production planning, not demo planning.
- Multi-tenancy is a roadmap family, not a reserved field or a single implementation task.
- Every user has a personal tenant and may join organization tenants.
- Runtime resources, integration state, secrets, evaluation records, billing usage, and
  operator projections must be tenant-owned.
- Tenant resolution uses token-allowed tenants, a default tenant, and an explicit request
  override.
- The first storage model is shared database with strict tenant scoping, while storage
  resolution must leave room for later per-tenant backends.
- Billing and quota are a first-class roadmap, not a one-line future note.
- Live side-effect validation and full tool-call replay are separate from basic
  non-live replay and require fresh authorization, audit, and kill-switch semantics.

## Dependencies On Completed Phases

- Roadmap 24: Tool-Call Orchestration
- Roadmap 27: Personal Integrations Platform
- Roadmap 28: Delivery And Notifications
- Roadmap 29: Calendar Integration
- Roadmap 30: Mail Integration
- Roadmap 31: Tasks And Reminders
- Roadmap 32: Operator Shell And Onboarding
- Roadmap 33: Evaluation And Replay Harness

## Roadmap Split

The hosted productization program is split into:

1. `019-tenant-identity-and-access-foundation.md` — Roadmap 34, **complete**.
2. `020-tenant-scoped-data-migration.md` — Roadmap 35, **complete**.
3. `021-tenant-aware-operator-shell-and-sdk.md`
4. `022-hosted-secrets-integrations-and-connector-isolation.md` — Roadmap 37 owns credential
   semantics for `provider_auth_states`, `mcp_servers`, `mcp_server_states`, `mcp_tools`,
   `connectors`, and the credential portion of `secret_scope_bindings` /
   `mcp_tool_exposure_rules`. Roadmap 35 explicitly does NOT change DDL or store access for
   those tables — the boundary is enforced by the
   `TestR37BoundarySignaturesGolden` exported-signature snapshot.
5. `023-billing-quotas-and-usage-accounting.md` — Roadmap 38 owns tenant plans, quota
   definitions, usage counters, reservations, denials, manual adjustments, billing
   inspection/admin APIs, SDK types, schema contracts, inventory registration, and
   hosted fail-closed/local-unlimited semantics.
6. `024-production-install-upgrade-backup-and-soak.md`
7. `025-live-validation-and-side-effect-replay.md`
8. `026-evaluation-product-expansion.md`

## Why Separate Specs Are Required

- tenant identity and access is a security boundary and should land before data migration
- tenant-scoped data migration is rollback-sensitive and needs its own verification plan
- operator shell and SDK changes affect API ergonomics and user-visible tenant selection
- secrets, integrations, connectors, and MCP installs have distinct leakage and blast-radius
  risks
- billing and quotas require durable accounting semantics, not only schema placeholders
- install, upgrade, backup, and foundational soak are operational readiness work, not
  feature UI
- live side-effect validation and full replay can cause external mutations and need a
  stronger safety model than non-live replay
- fixture editing and automatic candidate discovery are evaluation product work and should
  not be collapsed into the live executor
- final user-deliverable readiness is not achieved until the Roadmap 39 operational
  baseline has been rerun after Roadmaps 40 and 41, including live validation and
  evaluation product workflows in the soak workload

## Out Of Scope

- context engineering
- long-term memory
- self-improvement or dreaming
- external marketplace packaging
- payment-provider collection flows in the first billing roadmap unless explicitly added
  by a later spec clarification

## Verification Expectations

Each child roadmap must end with:

- tenant-aware API, event, persistence, and operator-visible behavior where applicable
- restart-safe persistence and migration coverage
- contract and SDK verification when API shape changes
- failure-mode tests for permission denial, cross-tenant access, quota enforcement, or
  side-effect abort paths as appropriate
- at least one manual or automated acceptance path that exercises the production-relevant
  behavior in `DOPE_ENV=test`

Final release readiness after this split additionally requires a release-verification pass
that depends on all eight child specs and reuses the Roadmap 39 soak harness against the
Roadmap 40 and 41 surfaces.

## Definition Of Done

This split is done when the child specs exist, runtime and harness docs link the roadmap
order, and future implementation can start from each child spec without reopening the
program structure.

## Recommended Spec Input

Do not implement this umbrella document directly. Use one child spec as the authoritative
input for a future roadmap slice.
