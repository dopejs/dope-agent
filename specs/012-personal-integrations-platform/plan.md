# Implementation Plan: Personal Integrations Platform

**Branch**: `012-personal-integrations-platform` | **Date**: 2026-04-22 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/012-personal-integrations-platform/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Add a first-class daemon-owned personal integrations plane that exposes inspectable
integration resources with account binding, readiness, auth, health, canonical-default,
and redacted provenance truth. The design closes roadmap 27 by introducing a dedicated
`integrations` control-plane package rather than overloading the existing IM
`connectors` supervisor, by extending runtime, workflow, and approval truth with
integration-binding summaries, and by providing one repo-owned fake integration backend,
a run-scoped probe route, and workflow-hosted binding propagation checks in
`KURA_ENV=test` so operators can verify shared readiness, approval, and provenance
behavior before calendar, mail, and reminder domains land.

## Technical Context

**Language/Version**: Go 1.24.0; Markdown docs; JSON Schema contracts
**Primary Dependencies**: `daemon/internal/api`, new `daemon/internal/integrations`, `daemon/internal/app`, `daemon/internal/runtime`, `daemon/internal/orchestration`, `daemon/internal/policy`, `daemon/internal/events`, `daemon/internal/store`, `daemon/internal/contracts`, existing auth wiring, and existing redacted secret-scope / sandbox provenance helpers reused where applicable
**Storage**: SQLite daemon state with additive `integrations` persistence plus additive integration-binding snapshots on existing runtime, workflow, and policy-backed documents; no new blob storage required in phase 27
**Testing**: `go test ./internal/integrations ./internal/api ./internal/store ./internal/app ./internal/runtime ./internal/orchestration ./internal/policy ./internal/contracts`, `make daemon-contract-test`, targeted approval and fake-integration probe regressions including workflow-hosted execution checks, plus one manual `KURA_ENV=test` verification path using the repo-owned fake integration backend
**Target Platform**: macOS/Linux local daemon in `KURA_ENV=test` by default, using the existing localhost HTTP API, SQLite store, and operator-authenticated `/v1/*` control plane
**Project Type**: Go daemon and harness control-plane service with schema-backed HTTP and event contracts
**Performance Goals**: list or inspect an integration from persisted local state in `<=500 ms`; project readiness/default-state changes to operator-visible resource reads in `<=1 s`; create integration-bound runtime tool-call provenance in `<=1 s` after probe execution completes on local test hardware
**Constraints**: phase 27 stays domain-agnostic and MUST NOT claim calendar/mail/reminder object behavior; multiple records may exist for the same domain/account/environment but exactly one canonical default may be active at a time; `unavailable` blocks integration-backed work while `degraded` stays inspectable and requires operation-specific gating; readiness gating must not redefine delivery-status semantics; secret-backed material remains redacted and environment-scoped; marketplace discovery and multi-user tenancy remain out of scope; existing connector, MCP, capability, workflow, and delivery behavior stays additive and backward compatible
**Scale/Scope**: one operator-managed daemon, low tens of integration records across a handful of personal domains, low-volume readiness transitions per day, and one repo-owned fake integration backend sufficient to verify shared runtime linkage before downstream domain roadmaps land

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Roadmap closure: PASS. This plan closes roadmap 27 only: shared integration resources,
  readiness and provenance semantics, canonical-default selection, runtime linkage, and
  one fake integration verification path. Calendar, mail, reminders, delivery, and
  marketplace work remain out of scope.
- Production-grade change control: PASS. The design adds a dedicated
  `daemon/internal/integrations` plane with additive API, store, schema, event, and
  runtime-linkage changes rather than refactoring existing connector or workflow systems
  broadly. Rollback is a single change-set revert of integration-specific surfaces while
  leaving other daemon planes intact.
- Contracts and auditability: PASS. The plan names additive resource routes, schema
  files, event families, runtime/provenance extensions, and docs updates so readiness,
  canonical-default changes, secret-scope truth, and fake integration probe execution all
  remain operator-visible.
- Verification and observability: PASS. The design requires targeted package, contract,
  restart, and approval/probe regressions plus one manual `KURA_ENV=test` fake-backend
  walkthrough. Operator-visible resources and events replace raw config or backend logs as
  the source of truth for integration readiness.
- Environment and secrets: PASS. Local planning and later verification stay in
  `KURA_ENV=test`, the fake integration backend avoids live personal-system connectors,
  and secret-backed material remains operator-owned, redacted, and environment-scoped.

Post-design re-check:

- PASS. The design remains roadmap-closed and domain-agnostic: it establishes a reusable
  integration substrate and verification path without drifting into calendar/mail feature
  work.
- PASS. Integration-backed execution stays on the existing runtime, workflow, approval,
  and event planes with additive binding summaries instead of introducing a second
  invocation ledger.
- PASS. Connector supervision is not overloaded with personal integration semantics; the
  new integrations plane can reuse patterns from existing daemon-owned resources without
  collapsing unrelated concepts.

## Project Structure

### Documentation (this feature)

```text
specs/012-personal-integrations-platform/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── integration-platform-surfaces.md
└── tasks.md
```

### Source Code (repository root)

```text
daemon/
├── internal/
│   ├── api/
│   ├── app/
│   ├── connectors/
│   ├── contracts/
│   ├── events/
│   ├── integrations/
│   ├── policy/
│   ├── runtime/
│   └── store/
└── go.mod

schemas/
├── api/
└── events/

docs/
├── harness/
└── runtime/

AGENTS.md
```

**Structure Decision**: Keep personal integration lifecycle, readiness projection,
canonical-default selection, backend abstraction, and fake-backend probe support in a new
`daemon/internal/integrations` package. `daemon/internal/api` exposes additive
integration-resource routes and the run-scoped probe verification route.
`daemon/internal/runtime`, `daemon/internal/orchestration`, and
`daemon/internal/policy` remain the execution, workflow, and approval owners, gaining
additive integration-binding summaries rather than a second execution plane.
`daemon/internal/store` persists integration resources and extended runtime/policy
documents. `schemas/` and `docs/` carry the new resource, event, and operator guidance
contracts. `AGENTS.md` should point at this plan for downstream task generation.

## Complexity Tracking

No constitution violations remain. The design avoids reusing IM connector resources for
personal integration identity, avoids a second execution ledger for integration-backed
work, avoids marketplace and tenancy scope drift, and avoids requiring live connectors to
close roadmap 27.

## Implementation Notes

- Add daemon-owned integration resources rather than deriving readiness from scattered
  provider or config state at read time.
- Preserve one canonical default per `domainKind/accountKey/environmentScope` while still
  allowing multiple integration records for intentional multi-backend cases.
- Reuse existing approval, policy, orchestration, and redacted secret-scope helpers by
  projecting additive `integrationBindings` summaries on approvals, tool calls, and
  workflow steps.
- Keep fake integration probe execution intentionally small and repo-owned: one read-only
  probe and one approval-gated mutation probe are sufficient to verify readiness gating,
  runtime truth, workflow-step truth, and provenance reuse without introducing
  domain-specific calendar or mail behavior.

## Automated Verification

- `cd daemon && go test ./internal/integrations ./internal/api ./internal/store ./internal/app ./internal/runtime ./internal/orchestration ./internal/policy ./internal/contracts`
- `make daemon-contract-test`
- `cd daemon && go test ./internal/api -run 'TestIntegrationRoutesProjectReadinessAndCanonicalDefault|TestIntegrationProbeRoutesLinkRuntimeApprovalAndProvenance|TestIntegrationRestartRestoresResourcesAndDoesNotInventHealthyState' -count=1`

These commands cover:

- integration resource registration, list, inspect, readiness transitions, and canonical
  default replacement
- degraded versus unavailable gating truth
- fake integration probe execution, approval handling, and runtime/workflow linkage
- contract regressions for additive API and event schema surfaces
- restart recovery for persisted integration resources and binding snapshots

## Verification Performed

Recorded on 2026-04-22:

- `cd daemon && go test ./internal/integrations ./internal/runtime ./internal/policy ./internal/store ./internal/api ./internal/app ./internal/contracts ./internal/orchestration -count=1` passed
- `make daemon-contract-test` passed
- a live current-branch daemon walkthrough passed on `127.0.0.1:19193` with
  `KURA_ENV=test`, isolated `KURA_DATA_DIR=/tmp/kura-integrations-manual`, local
  pairing auth, canonical-default promotion, inspect and mutate fake probes,
  approval resolution, and explicit `unavailable` blocking

## Manual Verification

- `make daemon-run-test`
- `make daemon-test-status`
- pair or reuse a local bearer token
- register one fake integration, move it through not-configured, auth-pending, healthy,
  degraded, and unavailable truth
- create a second integration for the same account and promote it as canonical default
- run one read-only probe and one approval-gated mutation probe against the canonical
  default integration
- inspect `/v1/integrations`, `/v1/integrations/{id}`, linked approval resources, linked
  run and tool-call truth, and event history to confirm integration identity, default
  selection, readiness, and redacted provenance are consistent; rely on the automated
  workflow-hosted regression for workflow-step binding propagation

## Residual Risks

- The fake integration backend verifies shared substrate behavior but does not validate
  real third-party auth flows or provider quirks; those risks remain for roadmaps 29-31.
- Existing `connectors` and `integrations` resources may look superficially similar to
  operators until docs and naming are tightened; implementation should emphasize the
  difference between channel ingress and personal-system account binding.
- The first runtime linkage shape must stay generic enough for downstream domains; if the
  binding summary is too narrow, later domain phases may still need additive schema work.

## Rollback Notes

- Rollback is a single change-set revert of integration-resource routes, persistence,
  runtime/provenance linkage, fake probe surfaces, and associated schema or doc updates.
- Existing connector, capability, MCP, and workflow planes remain valid if roadmap 27 is
  reverted because the design is additive.
- If rollback occurs after integration resources were created, preserve existing rows and
  event history as read-only audit truth even if the routes are disabled.
