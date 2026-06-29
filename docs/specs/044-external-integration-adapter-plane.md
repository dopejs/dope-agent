# External Integration Adapter Plane

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 59, the supervised
out-of-process adapter plane that real personal-data integrations (calendar, mail, and
later additional providers) plug into instead of being embedded in the daemon as
in-process backends. It is sequenced immediately before the real calendar and mail
provider closures so those roadmaps can adopt it as their provider execution boundary.

Primary source documents:
- `docs/architecture/module-map.md` (sections 1.8 Capability Supervisor, 3 Capability
  Processes, 4.1 Schemas / capability RPC contracts)
- `docs/specs/012-personal-integrations-platform.md`
- `docs/specs/022-hosted-secrets-integrations-and-connector-isolation.md`
- `docs/specs/027-integration-health-and-permission-diagnostics.md`
- `docs/specs/033-channel-connector-conformance-contract.md` (reuse the conformance
  pattern; do not duplicate it)
- `docs/specs/045-real-calendar-provider-closure.md` (first adopter)
- `docs/specs/048-real-mail-provider-closure.md` (first adopter)

## Background

Calendar and mail are currently implemented as in-daemon integrations with a repo-owned
fake backend behind a Go `Backend` interface (`daemon/internal/calendar/backend.go`,
`daemon/internal/mail`). The operation model — side-effect evidence, idempotency,
ambiguous-commit classification, live validation, artifacts, and delivery truth — already
lives in the daemon and is the single execution ledger per Roadmap 29/30.

Before the first real providers are closed (Roadmap 60 calendar, Roadmap 63 mail), the
project is adding a supervised out-of-process adapter plane so that real provider API
mapping runs in isolated, independently restartable processes rather than expanding
in-process daemon surface for every provider. The module map already reserves this boundary
for "heavy, fragile, dependency-rich, or safer-when-isolated" features and already names
"capability RPC contracts" as schema-defined.

## Goal

Establish a daemon-supervised, schema-defined RPC adapter plane that real integration
providers implement, while keeping all operation, ledger, evidence, and persistence truth
inside the daemon. Make the calendar (Roadmap 60) and mail (Roadmap 63) closures the first
adopters.

## Fixed Decisions

- The operation/ledger plane stays in the daemon and is NOT moved out of process. Adapters
  do provider request/response mapping only. Side-effect evidence, idempotency keys,
  ambiguous-commit classification, live-validation classification, artifacts, and
  persistence remain daemon-owned. Adapters MUST NOT create a second execution ledger.
- The seam is the existing per-domain `Backend` interface. The plane ships an in-daemon RPC
  client shim that implements `Backend` by calling an external adapter process. Existing
  managers and operation truth are unchanged.
- Adapter processes are supervised by the daemon Capability Supervisor (spawn, heartbeat,
  readiness, restart policy, registration), reusing existing supervisor patterns rather
  than inventing a parallel lifecycle.
- The RPC contract is schema-defined under `schemas/capability/` and contract-tested. No
  informal JSON boundary.
- Credentials are injected per call as daemon-scoped, short-lived material. Adapters MUST
  NOT persist credentials or message/event content. Roadmap 37 secret and redaction
  semantics are reused, not redefined.
- The in-daemon fake backend remains the deterministic verification baseline and the
  default in test env. The adapter plane is the path for real providers only.
- This roadmap does not build a marketplace, does not migrate IM connectors, and does not
  ship a specific provider (Google/Gmail land in Roadmap 60/63 as adapters).

## Dependencies On Completed Phases

- Roadmap 27: Personal Integrations Platform
- Roadmap 29: Calendar Integration
- Roadmap 30: Mail Integration
- Roadmap 37: Hosted Secrets, Integrations, And Connector Isolation
- Roadmap 42: Integration Health And Permission Diagnostics
- Roadmap 48: Channel Connector Conformance Contract (pattern reuse)

## In Scope

- a schema-defined capability RPC contract for integration adapters under
  `schemas/capability/`, covering the calendar and mail `Backend` operations
- an in-daemon RPC client shim implementing the existing `calendar.Backend` and mail
  backend interfaces by dispatching to an external adapter process
- Capability Supervisor integration: spawn, readiness gate, heartbeat, restart policy,
  registration and availability for integration adapters
- per-call scoped credential injection with no adapter-side persistence
- an adapter conformance contract and harness (mirrors connector conformance) so any
  future provider adapter is verified against the contract
- mapping of adapter failure, timeout, crash, and unavailability into existing integration
  diagnostics and live-validation classifications
- a reference adapter process skeleton (no real provider) used by conformance and tests

## Out Of Scope

- any specific real provider implementation (Google Calendar, Gmail, etc.) — those remain
  Roadmap 60/63, now built as adapters
- moving operation/ledger/evidence/persistence out of the daemon
- a community or third-party marketplace and distribution
- migrating existing IM connectors to this plane
- attendee/RSVP, recurrence, all-day, or attachment semantics (their own roadmaps)
- memory-backed integration context

## Operator Or User Problems To Solve

- Operators need a flaky or hanging provider integration to fail in isolation without
  destabilizing the daemon spine.
- Operators need provider adapter health, restarts, and failures to be observable through
  existing diagnostics rather than a new ad hoc surface.
- The project needs new real providers to be added behind one verified contract instead of
  growing in-process daemon surface per provider.

## User Stories

- As an operator, I can see an integration adapter process's health, readiness, and restart
  history alongside connector health.
- As an operator, when an adapter crashes mid-operation, I can see the operation classified
  with the same side-effect/ambiguity evidence as any other failure, with no ledger
  duplication and no daemon restart.
- As a developer, I can implement a new real provider by satisfying the adapter RPC
  conformance contract without touching the daemon operation plane.

## Functional Requirements

- The system MUST expose a schema-defined RPC contract covering the calendar and mail
  `Backend` operations, with contract tests under `make daemon-contract-test`.
- The daemon MUST provide a `Backend` implementation that dispatches to an external adapter
  process and preserves existing operation, evidence, idempotency, and artifact truth in
  the daemon.
- The Capability Supervisor MUST spawn, health-check, gate readiness for, and restart
  integration adapter processes, and MUST surface their state as daemon operational truth.
- Credentials MUST be injected per call as scoped, short-lived material; adapters MUST NOT
  persist credentials or content, and redaction policy MUST be preserved.
- Adapter failures (crash, hang/timeout, unavailable, auth/scope error) MUST map to
  existing integration diagnostics and live-validation classifications, and MUST NOT crash
  or block the daemon.
- An adapter conformance harness MUST verify any adapter against the RPC contract,
  including failure-mode behavior.
- The in-daemon fake backend path MUST remain available and unchanged.

## Compatibility And Operational Notes

This is additive: it introduces an alternate `Backend` implementation and a supervised
process boundary without changing the in-daemon operation model, the fake backend, or
existing API/event payloads beyond additive adapter-health fields. Test env continues to
default to the fake backend. Roadmap 60/63 switch their real provider target onto this
plane; their operation-truth requirements are unchanged because the ledger stays in-daemon.

## Verification Expectations

- Contract tests for the capability RPC schema.
- Supervisor lifecycle tests: spawn, readiness gate, heartbeat loss, restart policy.
- Conformance harness tests against the reference adapter skeleton, including crash, hang,
  timeout, and auth-failure modes.
- Credential-injection tests proving no adapter-side credential or content persistence and
  preserved redaction.
- Failure-isolation tests proving an adapter crash leaves the daemon healthy and the
  in-flight operation classified with single-ledger evidence.
- A test proving the operation ledger remains single and daemon-owned across the RPC
  boundary (no second ledger).
- Fake backend regression tests remain green.

## Definition Of Done

- A supervised, schema-defined external integration adapter plane exists with an in-daemon
  `Backend` RPC shim, Capability Supervisor lifecycle, scoped per-call credential
  injection, failure-to-diagnostics mapping, and a conformance harness — ready for Roadmap
  60/63 to implement the first real calendar and mail providers as adapters instead of
  in-process backends, with the operation ledger unchanged and daemon-owned.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/044-external-integration-adapter-plane.md 完成 phase 59 的工作`
