# Implementation Plan: External Integration Adapter Plane

**Branch**: `044-integration-adapter-plane` | **Date**: 2026-06-29 | **Spec**: [`spec.md`](./spec.md)
**Input**: Feature specification from `specs/044-integration-adapter-plane/spec.md`
**Upstream authority**: `docs/specs/044-external-integration-adapter-plane.md` (Roadmap 59)

## Summary

Add a supervised, out-of-process adapter plane that real personal-data integrations
(calendar and mail first) plug into, without moving any operation, ledger, evidence, or
persistence truth out of the daemon. The work delivers (a) a schema-defined capability RPC
contract under `schemas/capability/` covering the existing `calendar.Backend` and
`mail.Backend` operations, (b) an in-daemon RPC transport/client that spawns and talks to a
local adapter subprocess with deadline propagation, per-call scoped credential injection,
and an exact contract-version handshake, (c) per-domain `Backend` shims registered in the
calendar and mail managers under a new `integrations.BackendKind` (`adapter_rpc`) so backend
selection stays config-driven via the existing `BackendBinding`, (d) adapter lifecycle
wired into the existing `capabilities.Supervisor` (readiness gate, heartbeat, bounded
restart with backoff, circuit-break to failed/unavailable), (e) a reference adapter process
skeleton with no real provider, (f) an adapter conformance harness mirroring the connector
conformance pattern, and (g) failure-isolation, single-ledger, credential-no-persistence,
and contract-version regressions.

This closes Roadmap 59 and is the prerequisite for Roadmap 60 (real calendar) and Roadmap 63
(real mail), which will implement their first real providers as adapters on this plane. No
real provider ships in this roadmap; the in-daemon fake backend remains unchanged and the
test-environment default.

## Technical Context

**Language/Version**: Go 1.24 (daemon); JSON Schema contracts (`schemas/`); TS clients
consume only additive adapter-health fields, no client logic changes required.
**Primary Dependencies**: stdlib only for the new surface — `os/exec` (subprocess),
`encoding/json` (envelope framing), `context` (deadline propagation), `net/http` mux
(existing). Reuses existing packages: `daemon/internal/integrations` (Resource,
BackendKind, BackendBinding, diagnostics, redaction, provenance), `daemon/internal/calendar`
and `daemon/internal/mail` (operation managers + `Backend` seam + fake backends),
`daemon/internal/capabilities` (Supervisor), `daemon/internal/connectors` (conformance +
supervisor patterns to mirror). No new external module dependency is expected; the
RPC-transport choice (Phase 0) is constrained to a stdlib-only option.
**Storage**: SQLite (existing, `modernc.org/sqlite`). No new tables expected: adapter
supervision state reuses the existing `capabilities.Capability` records and recovery path
(`recoverPersistedStateWithSecrets` already threads `capabilitySupervisor`); backend
selection reuses the existing integration `BackendBinding`.
**Testing**: `cd daemon && go test ./...`; `make daemon-contract-test` for the new
capability RPC schema; new conformance harness + failure-injection tests alongside the new
packages and in `daemon/internal/{calendar,mail,integrations,capabilities}/*_test.go`.
**Target Platform**: Local-first daemon binary (macOS / Linux), single daemon process plus
one supervised local adapter subprocess per integration domain. Hosted deployment uses the
same binary; no deploy-topology change.
**Project Type**: Server daemon (Go) + shared JSON Schema contracts + JS client SDK
(consumer of additive fields only).
**Performance Goals**: RPC dispatch overhead MUST be negligible relative to real provider
latency (provider latency is out of the plane's control); the fake-backend path MUST show no
measurable regression. Per-operation work is bounded by a daemon-supplied deadline (Q1).
**Constraints**: Additive and reversible (backend selection is a `BackendBinding` change;
rollback = select `fake_local`/`native`). Single daemon-owned operation ledger; adapters
create no second ledger and perform provider mapping only. Exact contract-version match
(Q3). Per-call scoped credential injection with no adapter-side persistence; redaction
preserved (Constitution V, Roadmap 37). Circuit-break reuses the existing supervisor
`FailureCount >= 5 → StatusFailed` semantics mapped to "integration unavailable" (Q2).
**Scale/Scope**: ~100 tenants per daemon for the first hosted release; one shared
per-domain adapter process with per-call tenant isolation (no per-tenant adapter process).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Roadmap closure** — PASS. The plan closes Roadmap 59 end-to-end: RPC contract, in-daemon
  shim + transport, supervisor integration, reference adapter skeleton, conformance harness,
  and the full regression set. The roadmap was explicitly recut (renumber) before planning.
  No partial slice ships; no real provider is in scope (that is Roadmap 60/63).
- **Production-grade, minimal, reversible change** — PASS. The change is additive: a new
  `BackendKind` registration in two managers, new packages (transport, per-domain shims,
  conformance), and a new schema directory. The existing `Backend` interfaces, operation
  managers, fake backends, and API/event payloads are unchanged except additive
  adapter-health fields. Rollback is a `BackendBinding` selection back to the in-daemon
  backend. Blast radius is bounded to the new packages plus two manager registration points.
- **Contracts, compatibility, and auditability** — PASS. The new capability RPC contract is
  committed under `schemas/capability/integration-adapter/` with `make daemon-contract-test`
  coverage; additive adapter-health fields update `schemas/api` / `schemas/events` together
  with fixtures; design artifacts (`data-model.md`, `contracts/`) accompany the plan. No
  hidden side path: the adapter is reachable only through the registered `Backend` shim and
  the supervisor.
- **Verification and observability** — PASS. Required tests: capability RPC contract tests;
  supervisor lifecycle (readiness gate, heartbeat loss, bounded restart, circuit-break);
  conformance harness against the reference adapter including crash/hang/timeout/auth/
  malformed modes; credential-injection no-persistence + redaction tests; failure-isolation
  (daemon stays healthy, ambiguous-commit classification) tests; a single-ledger test across
  the RPC boundary; exact contract-version mismatch refusal test; unchanged fake-backend
  regressions. Observability: adapter readiness/health/restart on the existing
  capability/connector health surface; failures emit existing integration diagnostics plus
  structured restart/contract-mismatch logs; no credential/content leakage.
- **Environment and secrets** — PASS. Test environment continues to default to the fake
  backend; the plane is exercised in tests via the reference adapter skeleton. No new
  secrets; scoped credential material and redaction are reused from Roadmap 37 and injected
  per call, never persisted by the adapter, never logged. No live connector behavior change.

No violations require justification. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/044-integration-adapter-plane/
├── plan.md                 # This file
├── research.md             # Phase 0 output
├── data-model.md           # Phase 1 output
├── quickstart.md           # Phase 1 output
├── contracts/
│   ├── integration-adapter-rpc.md          # RPC contract description + version/failure semantics
│   ├── integration-adapter-request.schema.json
│   ├── integration-adapter-response.schema.json
│   └── adapter-health-event.schema.json
├── checklists/
│   └── requirements.md     # /speckit.specify quality checklist (existing)
└── tasks.md                # /speckit.tasks output (NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
daemon/
├── internal/
│   ├── integrations/
│   │   ├── types.go                         # add BackendKind "adapter_rpc"
│   │   └── adapterrpc/                       # NEW: stdlib RPC transport + client
│   │       ├── transport.go                  # subprocess spawn, JSON envelope framing, deadline propagation
│   │       ├── client.go                     # dispatch(op, deadline, scoped creds) -> response; exact version handshake
│   │       ├── credentials.go                # per-call scoped credential injection (no persistence)
│   │       ├── conformance.go                # adapter conformance harness (mirrors connectors/conformance.go)
│   │       └── *_test.go                      # transport, client, conformance, failure-mode tests
│   ├── calendar/
│   │   ├── adapter_backend.go                # NEW: calendar.Backend over adapterrpc.Client
│   │   ├── adapter_backend_test.go           # NEW: dispatch + failure-isolation + single-ledger tests
│   │   └── manager.go                        # register backends[adapter_rpc] = NewAdapterBackend(...)
│   ├── mail/
│   │   ├── adapter_backend.go                # NEW: mail.Backend over adapterrpc.Client
│   │   ├── adapter_backend_test.go           # NEW
│   │   └── manager.go                        # register backends[adapter_rpc] = NewAdapterBackend(...)
│   ├── capabilities/
│   │   ├── adapter_runtime.go                # NEW: bridges adapter process lifecycle -> Supervisor
│   │   │                                     #      (readiness gate, heartbeat, restart, StatusFailed -> unavailable)
│   │   └── adapter_runtime_test.go           # NEW
│   └── app/app.go                            # wire adapter runtime + register adapter backends in managers
├── cmd/
│   └── kura-integration-adapter/             # NEW: reference adapter skeleton binary (no real provider)
│       └── main.go
└── go.mod / go.sum                           # no new external deps expected

schemas/
├── capability/
│   └── integration-adapter/                  # NEW: RPC request/response/version/failure schemas
│       ├── request.json
│       ├── response.json
│       └── health-event.json
├── api/integrations/*.json                   # additive adapter-health/readiness fields where exposed
└── events/integrations/adapter-health.json   # NEW: additive adapter health event (or extend capability health)

capabilities/
└── integrations/
    └── README.md                             # document the reference adapter boundary (process home)

docs/
└── runtime/
    └── integration-adapter-plane.md          # NEW: operator notes — backend selection, rollback, diagnostics
```

**Structure Decision**: Single Go module (`daemon/`) plus shared `schemas/` contracts. The
RPC transport, client, credential injection, and conformance harness live in one auditable
package `daemon/internal/integrations/adapterrpc/` so the rule "adapters do provider mapping
only; the daemon owns the ledger" is enforced in one place shared by both domain shims. The
per-domain `Backend` shims live next to their managers and register under the new
`adapter_rpc` `BackendKind`, preserving the existing config-driven backend-selection seam.
Adapter process lifecycle reuses the existing `capabilities.Supervisor` via a thin
`adapter_runtime.go` bridge rather than a parallel lifecycle. The reference adapter is an
in-repo Go binary under `daemon/cmd/` (Phase 0 decision) so the conformance harness can drive
it deterministically without a new toolchain.

## Complexity Tracking

> Filled only when Constitution Check has unjustified violations. None for this plan.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| (none)    |            |                                      |
