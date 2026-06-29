# Feature Specification: External Integration Adapter Plane

**Feature Branch**: `044-integration-adapter-plane`  
**Created**: 2026-06-29  
**Status**: Draft  
**Input**: User description: "$speckit-specify 结合 docs/specs/044-external-integration-adapter-plane.md 完成 phase 59 的工作"

**Upstream authority**: `docs/specs/044-external-integration-adapter-plane.md` is the authoritative upstream document for this work (Roadmap 59). This specification translates that document into testable scenarios, requirements, and success criteria. Where the upstream document and this spec disagree, the upstream document wins and this spec must be updated.

**Dependencies**: Roadmap 27 (Personal Integrations Platform) and Roadmaps 29/30 (Calendar / Mail Integration) provide the in-daemon integration operation model, the per-domain `Backend` seam, and the fake backend baseline. Roadmap 37 (Hosted Secrets, Integrations, And Connector Isolation, `docs/specs/022-...`) owns scoped credential and redaction semantics reused here. Roadmap 42 (Integration Health And Permission Diagnostics, `docs/specs/027-...`) provides the diagnostics surface that adapter failures map into. Roadmap 48 (Channel Connector Conformance Contract, `docs/specs/033-...`, `specs/033-channel-connector-conformance/`) provides the conformance pattern this plane reuses for adapter verification. This plane is the prerequisite for Roadmap 60 (Real Calendar Provider Closure) and Roadmap 63 (Real Mail Provider Closure), which implement their first real providers as adapters on this plane.

## Clarifications

### Session 2026-06-29

- Q: Should each tenant get its own adapter process, or is the adapter process shared across tenants with per-call isolation? → A: Shared supervised adapter process per integration domain; tenant isolation is enforced by per-call scoped credential injection and the daemon-owned operation ledger, never by adapter-resident tenant state (the upstream "credentials injected per call, adapters MUST NOT persist credentials or content" decision dictates this).
- Q: Does this phase ship a real provider? → A: No. It ships the plane, the contract, the supervisor integration, the in-daemon RPC `Backend` shim, and a reference adapter skeleton with no real provider. Real calendar/mail providers are Roadmap 60/63.
- Q: Does the in-daemon fake backend move onto the plane? → A: No. The fake backend stays in-daemon, unchanged, and remains the test-env default. The plane is an alternate `Backend` implementation selected only for real-provider use.
- Q: Who owns the per-operation timeout for an adapter call? → A: The daemon supplies a per-operation deadline propagated across the RPC; the adapter must honor it, and a default bound applies when no deadline is given. Deadline expiry is the trigger for hang/timeout failure classification.
- Q: What happens when an adapter exceeds its restart bound (restart storm)? → A: The supervisor restarts within a bounded window; on exceeding the bound it circuit-breaks — stops auto-restarting, marks the integration unavailable, and surfaces the degraded state via diagnostics until repair/manual recovery.
- Q: How is daemon/adapter contract-version compatibility determined? → A: Exact contract-version match is required; any mismatch is refused with the contract-mismatch diagnostic before any operation is attempted (range/min-version negotiation deferred until third-party adapters exist).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Run Integration Operations Through A Supervised Out-Of-Process Adapter (Priority: P1)

A calendar or mail operation that targets a real provider is dispatched from the daemon, across a schema-defined RPC boundary, to a supervised external adapter process that maps the request to the provider and returns a normalized response. The daemon — not the adapter — records the operation, its side-effect evidence, idempotency key, and artifacts in the single operation ledger.

**Why this priority**: This is the foundational capability. Without a working dispatch path that preserves the existing operation ledger, no real provider can be closed and the plane delivers nothing.

**Independent Test**: Can be tested by configuring an integration to use the reference adapter, executing each `Backend` operation (project account, list/get events or messages, availability, create/update/cancel or draft/send/reply/forward) across the RPC boundary, and confirming each returns a normalized result recorded exactly once in the daemon operation ledger with unchanged operation, evidence, idempotency, and artifact semantics.

**Acceptance Scenarios**:

1. **Given** an integration bound to a supervised adapter, **When** a non-side-effecting read operation is executed, **Then** the daemon dispatches it over the RPC contract and returns the same normalized resource shape as the fake backend, recorded once in the operation ledger.
2. **Given** an integration bound to a supervised adapter, **When** a side-effecting write operation is executed, **Then** the daemon records the side-effect evidence, idempotency key, and artifacts itself, and the adapter performs provider mapping only.
3. **Given** the same idempotency key is replayed, **When** the operation is re-dispatched, **Then** the daemon-owned idempotency guarantee holds regardless of the adapter and no duplicate side effect is recorded.

---

### User Story 2 - Isolate Adapter Failures From The Daemon (Priority: P1)

When an adapter crashes, hangs, times out, becomes unavailable, or returns an auth/scope error, the daemon stays healthy, the in-flight operation is classified with the correct side-effect/ambiguity evidence in the single ledger, and the failure is surfaced through existing integration diagnostics — without restarting or blocking the daemon.

**Why this priority**: The entire justification for moving provider mapping out of process is failure isolation. If an adapter failure can destabilize the daemon or corrupt the ledger, the plane is a net negative over in-process backends.

**Independent Test**: Can be tested by injecting each failure mode (crash, hang past timeout, unavailable, malformed RPC payload, auth/scope error) into the reference adapter mid-operation and confirming the daemon process remains healthy, the operation is classified with correct (including ambiguous-commit) evidence, a diagnostic is emitted, and no second ledger entry or duplicate side effect appears.

**Acceptance Scenarios**:

1. **Given** an adapter that crashes during a side-effecting write, **When** the operation is in flight, **Then** the daemon stays healthy, the operation is classified as ambiguous-commit with single-ledger evidence, and a diagnostic is emitted.
2. **Given** an adapter that hangs beyond the operation timeout, **When** the deadline elapses, **Then** the daemon abandons the call without blocking other work and classifies the outcome via existing live-validation classification.
3. **Given** an adapter that returns a malformed or contract-violating RPC payload, **When** the response is received, **Then** the daemon rejects it safely, does not record a partial or corrupt ledger entry, and emits a diagnostic.

---

### User Story 3 - Inject Scoped Credentials Per Call Without Adapter Persistence (Priority: P1)

The daemon injects short-lived, scoped credential material into each adapter call. The adapter uses it only for that call and persists neither credentials nor message/event content. Redaction policy is preserved across the boundary.

**Why this priority**: Real providers require real credentials. Pushing provider calls out of process must not weaken the credential and redaction guarantees owned by Roadmap 37; a leak here is a security regression.

**Independent Test**: Can be tested by executing operations through the adapter and confirming credentials are supplied per call, are scoped and short-lived, are absent from adapter-side persistence and logs after the call, and that any content surfaced for evidence, support, fixtures, or logs is redacted per existing policy.

**Acceptance Scenarios**:

1. **Given** an operation requiring provider credentials, **When** it is dispatched, **Then** scoped credential material is injected for that call only and is not written to any adapter-side durable store.
2. **Given** a completed adapter call, **When** adapter state and logs are inspected, **Then** no credential material and no raw message/event content remain.
3. **Given** evidence, support, fixture, or log output is produced, **When** it is generated, **Then** sensitive content is redacted per existing policy across the RPC boundary.

---

### User Story 4 - Verify Any Adapter Against A Conformance Contract (Priority: P2)

A developer integrates a new real provider by implementing the adapter RPC contract and passing a conformance harness — including failure-mode behavior — without modifying the daemon operation plane.

**Why this priority**: The plane's payoff over per-provider in-process code is that providers are added behind one verified contract. Without an enforceable conformance harness, each new adapter risks silent contract drift.

**Independent Test**: Can be tested by running the conformance harness against the reference adapter skeleton and confirming it verifies every contract operation plus required failure modes (crash, hang, timeout, auth failure, malformed payload), and that a contract-violating adapter fails the harness.

**Acceptance Scenarios**:

1. **Given** the reference adapter skeleton, **When** the conformance harness runs, **Then** all contract operations and required failure modes pass.
2. **Given** an adapter that violates the contract, **When** the harness runs, **Then** the harness fails and identifies the violated contract obligation.

---

### User Story 5 - Observe Adapter Health Alongside Connector Health (Priority: P2)

An operator can inspect an integration adapter process's health, readiness, and restart history through the same operational surface used for connector health, and sees adapter state reflected as daemon operational truth.

**Why this priority**: Operators must diagnose a degraded integration without a new ad hoc surface; adapter liveness must be first-class daemon operational state, consistent with the connector supervisor.

**Independent Test**: Can be tested by starting, failing, and restarting the reference adapter and confirming its readiness, health, and restart history are visible as daemon operational truth within a bounded time of each state change.

**Acceptance Scenarios**:

1. **Given** a supervised adapter, **When** it starts and passes readiness, **Then** its readiness and health are visible as daemon operational state.
2. **Given** a supervised adapter that fails and is restarted by policy, **When** the restart occurs, **Then** the restart is recorded and observable in restart history.

---

### User Story 6 - Preserve The Single Ledger And Fake Backend Baseline (Priority: P2)

Introducing the plane does not create a second execution ledger and does not change the in-daemon fake backend, which remains the deterministic verification baseline and the default in the test environment.

**Why this priority**: The single daemon-owned ledger and the fake backend are the project's correctness and determinism guarantees; the plane must be additive and must not fork either.

**Independent Test**: Can be tested by running the existing fake-backend test suites unchanged with the plane present, and confirming operations through the adapter and through the fake backend land in the same single operation ledger with no second ledger or alternate execution plane.

**Acceptance Scenarios**:

1. **Given** the plane is present, **When** existing fake-backend tests run, **Then** they pass unchanged and the test env still defaults to the fake backend.
2. **Given** operations run through both the adapter and the fake backend, **When** the ledger is inspected, **Then** all operations are recorded in the same single daemon-owned ledger.

---

### Edge Cases

- What happens when an adapter never reaches readiness within the readiness deadline? The integration is reported unavailable via diagnostics; operations fail closed without a partial ledger entry.
- How does the system handle an adapter that hangs mid side-effecting write? The outcome is classified as ambiguous-commit with single-ledger evidence; no duplicate side effect is assumed.
- What happens when the daemon and adapter advertise contract versions that are not an exact match? Dispatch is refused with a clear contract-mismatch diagnostic; no operation is attempted.
- How does the system handle concurrent operations to one shared adapter process? Calls are isolated per call; one tenant's call cannot read another's credentials, content, or results.
- What happens when scoped credential material expires between dispatch and provider call? The call fails with an auth/scope diagnostic and is classified without recording a successful side effect.
- How does the system handle an adapter restart storm? The supervisor restarts within a bounded window; once the bound is exceeded it circuit-breaks — stops auto-restarting, marks the integration unavailable, and surfaces the degraded state via diagnostics until repair — without blocking the daemon spine.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST expose a schema-defined capability RPC contract covering the calendar and mail `Backend` operations, validated by contract tests.
- **FR-002**: The system MUST provide an in-daemon `Backend` implementation that dispatches each operation to an external adapter process over the RPC contract while keeping operation, side-effect evidence, idempotency, artifact, and persistence truth in the daemon.
- **FR-003**: Adapters MUST perform provider request/response mapping only and MUST NOT create or maintain a second execution ledger, idempotency record, or side-effect evidence store.
- **FR-004**: The daemon Capability Supervisor MUST spawn, readiness-gate, heartbeat, and restart integration adapter processes under an explicit restart policy, and MUST surface adapter readiness, health, and restart history as daemon operational truth.
- **FR-004a**: The restart policy MUST bound restarts within a window; on exceeding the bound the supervisor MUST circuit-break — stop auto-restarting, mark the integration unavailable, and surface the degraded state via diagnostics until repair or manual recovery — without blocking the daemon.
- **FR-005**: Adapter processes MUST be supervised per integration domain and shared across tenants, with tenant isolation enforced by per-call scoped credential injection and the daemon-owned ledger, never by adapter-resident tenant state.
- **FR-006**: The system MUST inject scoped, short-lived credential material per call; adapters MUST NOT persist credentials or message/event content, and existing redaction policy MUST be preserved across the RPC boundary.
- **FR-007**: Adapter failures — crash, hang/timeout, unavailable, malformed/contract-violating payload, and auth/scope error — MUST map to existing integration diagnostics and live-validation classifications and MUST NOT crash or block the daemon.
- **FR-007a**: A side-effecting operation whose outcome cannot be confirmed due to adapter failure MUST be classified with ambiguous-commit evidence in the single ledger rather than assumed committed or assumed failed.
- **FR-007b**: Each adapter call MUST carry a daemon-supplied per-operation deadline propagated across the RPC; the adapter MUST honor it, a default deadline bound MUST apply when none is supplied, and deadline expiry MUST trigger hang/timeout failure classification without blocking the daemon.
- **FR-008**: The system MUST refuse dispatch with a clear contract-mismatch diagnostic when the daemon and adapter advertise contract versions that are not an exact match, without attempting the operation.
- **FR-009**: The system MUST provide an adapter conformance harness that verifies any adapter against the RPC contract including required failure modes, and that fails a contract-violating adapter.
- **FR-010**: The system MUST include a reference adapter process skeleton, with no real provider, usable by the conformance harness and tests.
- **FR-011**: The in-daemon fake backend path MUST remain available and unchanged and MUST remain the default integration backend in the test environment.
- **FR-012**: Operations executed through an adapter MUST be recorded in the same single daemon-owned operation ledger as fake-backend operations, with no alternate execution plane.
- **FR-013**: Operation outcomes MUST fail closed and leave no partial or corrupt ledger entry when adapter dispatch, response validation, or required evidence recording cannot complete.
- **FR-014**: Adapter lifecycle and failure events MUST emit observable records (health/readiness/restart state plus failure diagnostics) consistent with the connector health surface.
- **FR-015**: Concurrent operations to one shared adapter MUST be isolated per call so that no operation can access another operation's credentials, content, or results.

### Key Entities *(include if feature involves data)*

- **Integration Adapter Process**: A supervised, out-of-process worker, scoped per integration domain, that maps `Backend` operations to a provider. Holds no durable tenant state, credentials, or content.
- **Adapter RPC Contract**: The schema-defined request/response surface (per `Backend` operation) plus failure semantics and a contract-version identifier.
- **Adapter Backend Shim**: The in-daemon `Backend` implementation that dispatches to an adapter and preserves daemon-owned operation truth.
- **Adapter Supervision Record**: Daemon-owned operational truth for an adapter: readiness, health, restart history, and current contract version.
- **Scoped Credential Injection**: Short-lived, per-call credential material supplied by the daemon and never persisted by the adapter.
- **Operation Ledger Entry**: The single daemon-owned record of an integration operation, its side-effect evidence, idempotency key, ambiguity classification, and artifacts — unchanged by the RPC boundary.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Additive. A new capability RPC contract is introduced under the schema set; API and event payloads gain only additive adapter-health/readiness fields. The existing `Backend` interface, in-daemon operation model, and fake backend are unchanged. No breaking changes to existing integration API routes or event payloads.
- **Migration / Rollback**: No data migration. The plane is an alternate `Backend` selection; rollback is selecting the in-daemon backend for an integration. No schema-version migration of persisted operation data is required by this phase.
- **Verification Strategy**: Contract tests for the RPC schema; supervisor lifecycle tests (spawn, readiness gate, heartbeat loss, restart policy); conformance harness tests against the reference adapter including crash/hang/timeout/auth-failure/malformed modes; credential-injection tests proving no adapter-side credential or content persistence and preserved redaction; failure-isolation tests proving daemon health and correct single-ledger classification under adapter failure; a single-ledger test proving no second ledger across the RPC boundary; full existing fake-backend regression suites remain green.
- **Observability Impact**: Adapter readiness, health, and restart history become observable daemon operational state on the connector-health surface; adapter failures emit existing integration diagnostics plus structured restart/contract-mismatch logs. No leakage of credentials or raw provider content into evidence, support, fixtures, or logs.
- **Environment & Secrets**: Test environment continues to default to the fake backend; the plane is exercised in tests via the reference adapter skeleton. No new secrets are introduced; scoped credential semantics are reused from Roadmap 37. No live connector behavior changes and no real provider is shipped in this phase.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In failure-injection tests covering every adapter failure mode, the daemon process experiences zero crashes or restarts and continues serving other work.
- **SC-002**: Across all tested operations through the adapter, 100% are recorded exactly once in the single daemon-owned operation ledger, with zero second-ledger or duplicate side-effect occurrences.
- **SC-003**: The first real calendar and mail providers (Roadmap 60/63) are integrated by satisfying only the adapter conformance contract, with zero changes to the daemon operation plane.
- **SC-004**: In credential-injection tests, zero credential material and zero raw provider content persist in adapter state or logs after a call, and redaction policy holds in 100% of evidence/support/fixture/log outputs checked.
- **SC-005**: Every adapter readiness, health, and restart-state change is reflected as daemon operational truth on the connector-health surface within 5 seconds (asserted bound; readiness gate deadline 10s, default per-operation deadline 30s per plan research R3).
- **SC-006**: All pre-existing fake-backend test suites pass unchanged with the plane present, and the test environment default backend remains the fake backend (zero regressions).
- **SC-007**: A contract-violating adapter fails the conformance harness in 100% of seeded violation cases, and a daemon/adapter contract-version mismatch is refused before any operation is attempted.

## Assumptions

- The existing per-domain `Backend` interface (calendar and mail) is a sufficient seam for RPC dispatch; no change to its operation surface is required by this phase.
- The daemon Capability Supervisor concept (spawn, heartbeat, readiness, restart, registration) is the lifecycle owner for adapter processes, reusing connector-supervisor patterns rather than introducing a parallel lifecycle.
- Scoped, short-lived credential material and redaction policy from Roadmap 37 are available and are reused unchanged; this phase does not redefine secret semantics.
- The conformance pattern from Roadmap 48 (connector conformance) is reusable for adapter conformance and is not duplicated.
- A shared per-domain adapter process with per-call tenant isolation is acceptable for the first hosted release scale; per-tenant physical adapter processes are out of scope.
- IM connectors are not migrated to this plane in this phase, and no marketplace or third-party adapter distribution is in scope.
