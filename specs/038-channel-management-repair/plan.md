# Implementation Plan: Channel Management And Repair UX

**Branch**: `038-channel-management-repair` | **Date**: 2026-05-10 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/038-channel-management-repair/spec.md`

## Summary

Close Roadmap 53 by adding a tenant-scoped channel management and repair product surface
for existing production channel connectors. The work coordinates the current connector
supervisor, connector-specific setup projections, shared diagnostics, delivery outcomes,
route policy evidence, audit events, schemas, TypeScript SDK, and web UI so users can
list, inspect, disable, re-enable, repair, reconnect, rotate supported credentials,
update supported routes, inspect foreground reply and background delivery status, and
provide metadata-only support evidence without raw config edits or log access.

The implementation is additive. It does not add a connector or replace the existing
Discord, Telegram, Slack, or Matrix contracts. It adds management projections,
paginated deterministic list behavior, permission gates, mutation serialization,
audit fail-closed behavior, disablement precedence, 15-minute diagnostic freshness,
90-day default support evidence retention, metadata-only support evidence, API/schema
contracts, SDK methods, web product flows, and a test-environment walkthrough covering
at least two connector kinds.

## Technical Context

**Language/Version**: Go 1.24 daemon/control-plane code; TypeScript client SDK and web
product UI; JSON Schema contracts under `schemas/`; markdown operator/channel docs.  
**Primary Dependencies**: Existing `daemon/internal/connectors`,
`daemon/internal/api`, `daemon/internal/store`, `daemon/internal/store/tenancy`,
`daemon/internal/setupwizard`, `daemon/internal/integrations` diagnostics,
`daemon/internal/delivery`, `daemon/internal/events`, `daemon/internal/identity`,
`schemas/api`, `schemas/events`, `sdk/ts/src`, `web/src`, and channel docs under
`docs/channels`.  
**Storage**: Existing SQLite daemon store remains authoritative. Additive persistence
may be needed for channel management projection state, connector enablement/audit
records, repair actions, route-policy snapshots, support evidence bundles, foreground
reply and background delivery projection links, retention expiry, and mutation locks.
Existing connector setup, diagnostic, conformance, delivery, and provider-specific
records remain readable.  
**Testing**: Targeted Go tests under `daemon/internal/connectors`,
`daemon/internal/api`, `daemon/internal/store`, `daemon/internal/delivery`, and
`daemon/internal/contracts`; schema/fixture validation via `make daemon-contract-test`;
SDK/web client coverage via `pnpm test:clients`; client build via `pnpm build`; full
daemon coverage via `go test ./...` from `daemon/`; `go mod tidy` from `daemon/` after
implementation.  
**Target Platform**: Local-first daemon and hosted daemon behavior, with web and
TypeScript SDK product surfaces. Default local verification uses `~/.dope-test` and
`127.0.0.1:19192`.  
**Project Type**: Multi-surface daemon product feature spanning API, persistence,
contracts, SDK, web UI, delivery/connector projections, diagnostics, audit, and docs.  
**Performance Goals**: Authorized users can identify status, health, diagnostic
freshness, and next action for 100% of configured tenant production connectors within
2 minutes. Paginated connector lists default to 20 items and return deterministic,
non-duplicated page sequences. Authorized support can reconstruct a representative
connector incident within 5 minutes.  
**Constraints**: Manage existing production connectors only. Do not add a new connector,
marketplace, mobile push app, memory-driven ranking, autonomous remediation, or raw
message-content support evidence. Redacted inspection requires `credentials.inspect`;
diagnostic inspection requires `integrations.diagnostics.read`; disable, re-enable,
route edits, and repair starts require `connectors.manage`; reconnect or credential
rotation additionally requires `secrets.manage`. Mutations serialize per connector and
fail closed if audit evidence cannot be recorded. Disablement takes precedence for new
inbound work and background delivery eligibility until validated re-enable succeeds.  
**Scale/Scope**: One whole roadmap slice, Phase 53. Required surfaces are API,
TypeScript SDK, and web product flows for list, detail, disable, re-enable, repair,
diagnostics, route policy, reply status, delivery status, and support evidence.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Roadmap closure** - PASS. This plan closes Roadmap 53 as a complete product and
  repair surface over existing production connectors: list/detail, enablement,
  repair/reconnect/rotation, diagnostics, route policy, foreground/background status,
  support evidence, API/SDK/web surfaces, contracts, tests, docs, and walkthrough.
- **Production-grade, minimal, reversible change** - PASS. The design is additive over
  existing connector, setup, diagnostic, delivery, audit, schema, SDK, and web surfaces.
  Rollback can hide management routes and web flows and block new management mutations
  while preserving existing connector runtime behavior and already-written evidence.
- **Contracts and auditability** - PASS. Public API/schema/event/SDK behavior is captured
  in [contracts/channel-management-repair.md](./contracts/channel-management-repair.md).
  Mutations are tenant-scoped, permission-gated, serialized per connector, and fail
  closed on required audit-write failure.
- **Verification and observability** - PASS. Verification covers API, SDK, web,
  pagination, deterministic ordering, permission denials, disable/re-enable, disabled
  ingress suppression, delivery eligibility blocking, repair-to-setup linkage, route
  updates, foreground/background separation, diagnostics, stale refresh, redaction,
  retention, audit fail-closed, restart recovery, and conformance regression for at
  least two connector kinds.
- **Environment and secrets** - PASS. Local work defaults to `~/.dope-test` with fake
  connector evidence. Live connector credentials and production tenants are not required
  for automated acceptance. Support evidence is metadata-only and must not display raw
  provider payloads or channel message bodies.

No violations require justification. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/038-channel-management-repair/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── channel-management-repair.md
├── checklists/
│   └── requirements.md
└── tasks.md                # /speckit.tasks output, not created by /speckit.plan
```

### Source Code (repository root)

```text
daemon/
├── internal/
│   ├── connectors/
│   │   ├── supervisor.go          # enablement state, list ordering, mutation behavior
│   │   ├── diagnostics.go         # shared freshness, redaction, retention vocabulary
│   │   ├── conformance.go         # conformance regression after disable/re-enable
│   │   └── */                     # provider-specific setup/route/capability evidence
│   ├── api/
│   │   ├── server.go              # channel management API routes
│   │   └── *_test.go              # API permission, mutation, projection coverage
│   ├── store/
│   │   ├── *.go                   # SQLite management/evidence persistence if needed
│   │   └── tenancy/               # tenant-safe access helpers
│   ├── setupwizard/               # repair/reconnect/credential-rotation linkage
│   ├── delivery/                  # background delivery eligibility/status projection
│   ├── integrations/              # diagnostic reason/freshness reuse
│   ├── events/                    # connector management/audit/retention events
│   └── contracts/                 # schema and fixture contract tests

schemas/
├── api/                           # channel management resources and responses
└── events/                        # management, audit, retention, redaction events

sdk/ts/src/
├── index.ts                       # resource types and client methods
└── index.test.ts                  # SDK endpoint/typing coverage

web/src/
├── app/App.tsx                    # channel management product view integration
├── app/App.test.tsx               # web flows and states
└── features/                      # reusable diagnostics/status components if needed

docs/
└── channels/                      # channel management and repair operator docs
```

**Structure Decision**: Implement channel management as a cross-connector product
projection rather than a new connector package. Existing provider packages remain
authoritative for setup, route policy, capabilities, diagnostics, and delivery evidence.
Shared management orchestration lives in `daemon/internal/connectors`, API projection in
`daemon/internal/api`, durable tenant-safe evidence in `daemon/internal/store`, repair
links through `setupwizard`, delivery status through `delivery`, contracts in
`schemas/`, client access in `sdk/ts`, and the user workflow in `web`.

## Roadmap 53 Planning Contracts

The implementation plan MUST keep this artifact complete before `/speckit.tasks`:

- [contracts/channel-management-repair.md](./contracts/channel-management-repair.md)
  - Tenant-scoped list/detail resources, pagination and default ordering, permission
    gates, enablement mutations, repair/reconnect/rotation actions, route policy updates,
    diagnostic projection, foreground reply status, background delivery status,
    metadata-only support evidence, retention, redaction, audit fail-closed behavior,
    event/schema/SDK/web expectations, compatibility, and rollback.

This artifact is a planning gate. Implementation is incomplete if channel management can
list, mutate, repair, reconnect, rotate, update routes, expose support evidence, or show
reply/delivery status without a contract row and proving test.

## Migration And Rollback Plan

1. Add or extend API/schema resources for channel management list/detail projections,
   pagination metadata, deterministic ordering, enablement state, management actions,
   repair actions, support evidence, and audit outcomes without changing existing
   connector runtime contracts.
2. Add tenant-safe store accessors or tables only where current connector/setup/
   diagnostic/delivery records cannot already supply required management evidence.
   Preserve existing Discord, Telegram, Slack, Matrix, diagnostic, delivery, conformance,
   and setup records.
3. Add per-connector mutation serialization around disable, re-enable, route edit,
   repair, reconnect, credential rotation, and delivery eligibility changes. Ensure
   disablement wins over in-flight repair and delivery eligibility until validated
   re-enable succeeds.
4. Add required audit recording before committing connector mutations. If required audit
   evidence cannot be recorded, fail closed and leave connector state unchanged.
5. Add repair action projection that links diagnostic reasons to setup sessions,
   reconnect, supported credential rotation, route revalidation, and diagnostic reruns
   while enforcing `connectors.manage` and `secrets.manage` where required.
6. Add metadata-only support evidence bundles that aggregate setup, diagnostic, route,
   enablement, repair, reply, delivery, audit, redaction, and retention metadata without
   channel message bodies or raw provider payloads.
7. Expose the management behavior through API, TypeScript SDK methods/types, and web
   product flows. Keep TUI out of scope for Phase 53 unless later recut.
8. Update docs and walkthroughs for test-environment channel management and repair over
   at least two connector kinds.

Rollback hides or disables new channel management routes, SDK/web entry points, repair
actions, route-edit controls, and support evidence exports while retaining existing
connector runtime behavior and already-written audit/evidence records until retention
expiry. Additive persistence remains inert rather than destructively removed. Existing
connector setup, diagnostics, ingress, delivery, and conformance behavior must continue
to work without the management UX.

## Post-Design Constitution Re-check

- **Roadmap closure** - PASS. `research.md`, `data-model.md`, contract, and quickstart
  cover the full Roadmap 53 product surface across API, SDK, web, support evidence,
  repair, disable/re-enable, route policy, diagnostics, reply/delivery status, and
  verification.
- **Production-grade, minimal, reversible change** - PASS. Design artifacts preserve
  existing connector sources of truth and add management projections, evidence, and
  controls without replacing provider runtimes.
- **Contracts and auditability** - PASS. The contract defines API/SDK/web behavior,
  event/audit expectations, mutation serialization, and fail-closed audit behavior.
- **Verification and observability** - PASS. The quickstart and contract identify daemon,
  contract, SDK, web, retention, redaction, and manual walkthrough verification.
- **Environment and secrets** - PASS. The quickstart defaults to the test daemon and
  metadata-only evidence; live credentials and production tenants remain out of default
  verification.

## Complexity Tracking

No constitution violations require justification.
