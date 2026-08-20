# Tasks: External Integration Adapter Plane

**Input**: Design documents from `specs/044-integration-adapter-plane/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/
**Upstream authority**: `docs/specs/044-external-integration-adapter-plane.md` (Roadmap 59)

**Tests**: REQUIRED. The constitution mandates verification at every changed boundary and
contract tests for any schema/event surface change; the spec's Verification Expectations name
contract, supervisor-lifecycle, conformance, credential, failure-isolation, and single-ledger
coverage. Test tasks are therefore included per story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on an incomplete task)
- **[Story]**: US1–US6 from spec.md (Setup/Foundational/Polish have no story label)
- All paths are repository-relative.

## Path Conventions

Go daemon module under `daemon/`; shared contracts under `schemas/`; reference adapter binary
under `daemon/cmd/`; capability process home under `capabilities/`.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: scaffolding and the backend-selection enum value

- [X] T001 Add `BackendKind` value `adapter_rpc` (with constant + validation) in `daemon/internal/integrations/types.go`
- [X] T002 [P] Create capability schema directory and index `schemas/capability/integration-adapter/README.md`
- [X] T003 [P] Scaffold reference adapter binary entrypoint (stdio loop stub, no handlers yet) in `daemon/cmd/kura-integration-adapter/main.go`
- [X] T004 [P] Document the adapter process-home boundary in `capabilities/integrations/README.md`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the RPC contract, transport, client, and reference adapter that ALL stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T005 Author RPC request schema `schemas/capability/integration-adapter/request.json` (from `contracts/integration-adapter-request.schema.json`)
- [X] T006 [P] Author RPC response schema `schemas/capability/integration-adapter/response.json` (from `contracts/integration-adapter-response.schema.json`)
- [X] T007 [P] Author adapter health event schema `schemas/events/integrations/adapter-health.json` (from `contracts/adapter-health-event.schema.json`)
- [X] T008 Register the new capability + event schemas in the contract-test set so `make daemon-contract-test` validates them (`daemon/internal/contracts/`)
- [X] T009 Implement RPC transport: subprocess spawn, newline-delimited JSON framing, `requestId` correlation, `context` deadline enforcement in `daemon/internal/integrations/adapterrpc/transport.go`
- [X] T010 Implement RPC client: per-operation `dispatch(ctx, op, payload, creds)`, exact `contractVersion` handshake at readiness, response validation in `daemon/internal/integrations/adapterrpc/client.go`
- [X] T011 Implement reference adapter contract handlers (deterministic `ok` responses for all calendar+mail operations) in `daemon/cmd/kura-integration-adapter/main.go`
- [X] T012 [P] Unit tests for transport framing + deadline + correlation in `daemon/internal/integrations/adapterrpc/transport_test.go`

**Checkpoint**: contract + transport + client + reference adapter exist; stories can begin

---

## Phase 3: User Story 1 - Run Operations Through A Supervised Out-Of-Process Adapter (Priority: P1) 🎯 MVP

**Goal**: dispatch each calendar/mail `Backend` operation across the RPC boundary, with the
daemon recording the operation exactly once and the adapter doing provider mapping only.

**Independent test**: bind an integration to `adapter_rpc`, run each operation against the
reference adapter, confirm normalized results recorded once with unchanged ledger semantics.

- [X] T013 [US1] Implement `calendar.Backend` shim over the RPC client in `daemon/internal/calendar/adapter_backend.go` (provider mapping only; no ledger/idempotency state)
- [X] T014 [P] [US1] Implement mail `Backend` shim over the RPC client in `daemon/internal/mail/adapter_backend.go`
- [X] T015 [US1] Register `backends[adapter_rpc]` in `daemon/internal/calendar/manager.go`
- [X] T016 [P] [US1] Register `backends[adapter_rpc]` in `daemon/internal/mail/manager.go`
- [X] T017 [US1] Propagate the daemon-supplied per-operation deadline across dispatch (FR-007b) in the client/shims
- [X] T018 [US1] Tests: read + side-effecting write dispatch happy paths return normalized results recorded once (calendar) in `daemon/internal/calendar/adapter_backend_test.go`
- [X] T019 [P] [US1] Tests: read + write dispatch happy paths (mail) in `daemon/internal/mail/adapter_backend_test.go`

**Checkpoint**: real-shaped operations flow through the adapter with the ledger intact (MVP)

---

## Phase 4: User Story 2 - Isolate Adapter Failures From The Daemon (Priority: P1)

**Goal**: adapter crash/hang/timeout/unavailable/malformed/auth never destabilizes the daemon;
the in-flight operation is classified with correct single-ledger evidence.

**Independent test**: inject each failure mode mid-operation; assert daemon healthy, correct
(incl. ambiguous-commit) classification, diagnostic emitted, no partial/duplicate ledger entry.

- [X] T020 [US2] Map adapter failure kinds to existing integration diagnostics + live-validation classification in `daemon/internal/integrations/adapterrpc/client.go`
- [X] T021 [US2] Classify unconfirmed side-effecting writes as ambiguous-commit (FR-007a) via the existing manager live-validation path in `daemon/internal/calendar/adapter_backend.go` and `daemon/internal/mail/adapter_backend.go`
- [X] T022 [US2] Extend reference adapter with seeded failure modes (crash, hang-past-deadline, unavailable, malformed payload, auth/scope) in `daemon/cmd/kura-integration-adapter/main.go`
- [X] T023 [US2] Tests: failure-injection proves daemon stays healthy, ambiguous-commit classification, diagnostic emitted, no partial ledger, and fail-closed when required evidence recording cannot complete (FR-013) in `daemon/internal/integrations/adapterrpc/failure_test.go`

**Checkpoint**: adapter failures are contained and correctly classified

---

## Phase 5: User Story 3 - Inject Scoped Credentials Per Call Without Persistence (Priority: P1)

**Goal**: per-call scoped, short-lived credentials in the envelope; no adapter-side
persistence; redaction preserved across the boundary.

**Independent test**: run operations, confirm credentials are per-call and scoped, absent from
adapter state/logs afterward, content redacted, and concurrent multi-tenant calls on one
shared adapter do not cross-bleed credentials, content, or results.

- [X] T024 [US3] Implement per-call scoped credential resolution + envelope injection from the Roadmap 37 secret path in `daemon/internal/integrations/adapterrpc/credentials.go`
- [X] T025 [US3] Ensure no adapter-side credential/content persistence and reuse existing redaction for evidence/logs (reference adapter + client)
- [X] T026 [US3] Tests: credential no-persistence, per-call scoping, and preserved redaction in `daemon/internal/integrations/adapterrpc/credentials_test.go`
- [X] T026a [P] [US3] Tests: concurrent multi-tenant operations on one shared per-domain adapter are isolated per call — no cross-bleed of credentials, content, or results (FR-005, FR-015) in `daemon/internal/integrations/adapterrpc/isolation_test.go`

**Checkpoint**: credential hygiene holds across the RPC boundary

---

## Phase 6: User Story 4 - Verify Any Adapter Against A Conformance Contract (Priority: P2)

**Goal**: a conformance harness verifies any adapter against the RPC contract incl. failure
modes, and fails a contract-violating or version-mismatched adapter.

**Independent test**: run the harness against the reference adapter (pass) and a seeded
contract-violating adapter (fail); confirm version mismatch is refused before any operation.

- [X] T027 [US4] Implement adapter conformance harness mirroring the connector pattern in `daemon/internal/integrations/adapterrpc/conformance.go`
- [X] T028 [US4] Implement exact contract-version mismatch refusal + `contract_mismatch` diagnostic (FR-008, Q3) in `daemon/internal/integrations/adapterrpc/client.go`
- [X] T029 [US4] Tests: harness passes reference adapter (incl. failure modes), fails a contract-violating adapter, and refuses a version mismatch in `daemon/internal/integrations/adapterrpc/conformance_test.go`

**Checkpoint**: new adapters are verifiable behind one contract

---

## Phase 7: User Story 5 - Observe Adapter Health Alongside Connector Health (Priority: P2)

**Goal**: adapter readiness/health/restart are daemon operational truth on the existing
capability/connector surface, with circuit-break to "unavailable".

**Independent test**: start/fail/restart the reference adapter; confirm readiness, health, and
restart history are visible within a bounded interval and ≥5 failures circuit-break to unavailable.

- [X] T030 [US5] Implement adapter runtime bridge (spawn, readiness gate, heartbeat → `capabilities.Supervisor`) in `daemon/internal/capabilities/adapter_runtime.go`
- [X] T031 [US5] Map supervisor backoff + `FailureCount>=5 → StatusFailed` to integration "unavailable" + diagnostic (FR-004a, Q2) in `daemon/internal/capabilities/adapter_runtime.go`
- [X] T032 [US5] Emit additive adapter health events on transitions and add additive adapter-health/readiness fields in `schemas/api/integrations/*.json` (+ fixtures)
- [X] T033 [US5] Wire adapter runtime startup + manager backend registration + recovery into `daemon/internal/app/app.go`
- [X] T034 [US5] Tests: readiness gate, heartbeat loss, bounded restart, circuit-break, and health-surface visibility in `daemon/internal/capabilities/adapter_runtime_test.go`

**Checkpoint**: adapter liveness is observable and self-protecting

---

## Phase 8: User Story 6 - Preserve The Single Ledger And Fake Backend Baseline (Priority: P2)

**Goal**: no second execution ledger; fake backend unchanged and still the test-env default.

**Independent test**: run operations via adapter and fake; confirm one daemon ledger; run
existing fake suites unchanged; confirm test-env default backend is fake.

- [X] T035 [US6] Test: operations via adapter and fake land in the single daemon-owned ledger with no second ledger/duplicate side effect (FR-012, SC-002) in `daemon/internal/calendar/adapter_backend_test.go` and `daemon/internal/mail/adapter_backend_test.go`
- [X] T036 [P] [US6] Test: existing fake-backend suites pass unchanged and the test-env default backend remains `fake_local` (FR-011, SC-006)
- [X] T037 [US6] Verify managers retain operation/idempotency/artifact ownership across the RPC boundary (no alternate execution plane) — assertion test in `daemon/internal/integrations/adapterrpc/single_ledger_test.go`

**Checkpoint**: correctness/determinism guarantees preserved

---

## Phase 9: Polish & Cross-Cutting Concerns

- [X] T038 [P] Write operator runbook (backend selection, rollback, diagnostics, circuit-break) in `docs/runtime/integration-adapter-plane.md`
- [X] T039 [P] Regenerate client types for additive adapter-health fields and confirm SDK/web consume additive-only (no logic change)
- [X] T040 Run `make daemon-contract-test` and `cd daemon && go test ./...`; record results and any residual gaps
- [X] T041 [P] Walk through `quickstart.md` end to end against the test daemon + reference adapter

---

## Dependencies

```text
Setup (T001–T004)
   └─▶ Foundational (T005–T012)   ← blocks all stories
          ├─▶ US1 (T013–T019)  🎯 MVP
          │      ├─▶ US2 (T020–T023)   (failure handling on dispatch)
          │      ├─▶ US3 (T024–T026a)  (credential injection + per-call isolation on dispatch)
          │      └─▶ US6 (T035–T037)   (single-ledger relies on dispatch)
          ├─▶ US4 (T027–T029)          (conformance over contract + reference adapter)
          └─▶ US5 (T030–T034)          (lifecycle over supervisor; app wiring)
   └─▶ Polish (T038–T041)              (after stories land)
```

- US2, US3, US6 depend on US1 (dispatch path).
- US4 and US5 depend only on Foundational; they can proceed in parallel with US1.
- T033 (app.go wiring) should land after T015/T016 (manager registration) and T030 (runtime).

## Parallel Execution Examples

- **Foundational**: T006, T007 (schemas) and T012 (transport test) run in parallel with T005/T009/T010 authored first.
- **US1**: calendar vs mail are independent files — run T014/T016/T019 in parallel with T013/T015/T018.
- **Cross-story**: once Foundational is done, one developer takes US1→US2→US3 while another takes US4 and US5 concurrently.

## Implementation Strategy

- **MVP = Phase 1 + Phase 2 + US1**: a real-shaped operation flows through a supervised
  out-of-process adapter with the daemon ledger intact. This alone proves the plane viable.
- **Increment 2 (P1 completion)**: US2 + US3 — failure isolation and credential hygiene make
  the plane safe enough for a real provider.
- **Increment 3 (P2)**: US4 (conformance), US5 (observability/lifecycle), US6 (guardrail tests)
  harden it for Roadmap 60/63 adoption.
- Roadmap 59 is closed only when all six stories and Polish pass; no real provider ships here.

## Notes

- Total tasks: 42 (under the 50-task branch budget).
- No new external Go dependency is expected (stdlib transport per research R1).
- Test-env default remains the fake backend throughout; the plane is opt-in per integration.
