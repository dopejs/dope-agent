# Phase 0 Research: External Integration Adapter Plane

All items below are plan-level implementation unknowns. Spec-level ambiguities were resolved
in the spec `## Clarifications` (Q1 deadline ownership, Q2 restart/circuit-break, Q3 exact
contract-version match) and are treated here as fixed inputs.

## R1. RPC transport mechanism

- **Decision**: Local child process spawned by the daemon, communicating over stdio with
  newline-delimited JSON request/response envelopes that validate against the
  `schemas/capability/integration-adapter/` schemas. One request in flight per call,
  correlated by `requestId`, with the daemon enforcing the deadline.
- **Rationale**: Stdlib-only (`os/exec`, `encoding/json`, `context`); no port/socket
  management or new external dependency (Constitution II + Technical Context constraint);
  process lifecycle is naturally coupled to the pipe, so a crashed adapter is detected
  immediately; matches the repository's JSON-Schema-as-contract convention.
- **Alternatives considered**: Unix domain socket (supports multi-client and reconnection
  but adds socket-path lifecycle and cleanup; unnecessary for one supervised child per
  domain) — deferred until multi-consumer need exists. gRPC/proto (mature streaming and
  typing but introduces a new dependency and codegen toolchain) — rejected per no-new-deps.

## R2. Reference adapter language/runtime

- **Decision**: An in-repo Go binary `daemon/cmd/dope-integration-adapter` implementing the
  RPC contract over stdio, with no real provider (returns deterministic synthetic responses
  and supports seeded failure modes for conformance).
- **Rationale**: Reuses the existing Go toolchain and build (`make daemon-build`); lets the
  conformance harness build and drive it deterministically in `go test`; keeps the failure
  injection (crash/hang/timeout/auth/malformed) in one controllable place.
- **Alternatives considered**: Node reference adapter (matches some IM connector ecosystems
  and proves cross-language capability) — deferred; the module map permits per-adapter
  language choice later, but a Go reference keeps this phase's test surface minimal.

## R3. Adapter process lifecycle and circuit-break

- **Decision**: Reuse the existing `capabilities.Supervisor`. A new
  `capabilities/adapter_runtime.go` bridge owns `os/exec` spawn, the readiness gate, and the
  heartbeat, and reports state into the supervisor via `Register` / `ReportHealth` /
  `ReportFailure` / `Restart`. The supervisor's existing semantics provide the Q2 policy
  directly: exponential backoff (`5 * 2^(n-1)`, capped 300s) on failure, and
  `FailureCount >= 5 → StatusFailed`. `StatusFailed` is mapped to "integration unavailable"
  and surfaced via existing integration diagnostics; no further auto-restart until repair.
- **Rationale**: Avoids a parallel lifecycle (Constitution II, Engineering Constraints); the
  supervisor already persists/recovers via `recoverPersistedStateWithSecrets`, so adapter
  supervision survives restart with no new table.
- **Open implementation values** (set during tasks, not blocking): default per-operation
  deadline (proposed 30s) and readiness deadline (proposed 10s), both overridable; restart
  bound stays the supervisor's existing `5`. These are constants/config, not contract.
- **Alternatives considered**: A dedicated adapter supervisor — rejected; duplicates working
  lifecycle logic. Pure declarative supervision with no real subprocess — rejected; it would
  not deliver the failure isolation that justifies the plane (and would be dishonest about
  enforcement strength, Constitution V).

## R4. Backend selection and rollback

- **Decision**: Add `integrations.BackendKind` value `adapter_rpc`. Register an
  adapter-backed `Backend` shim under that kind in both `calendar.Manager.backends` and the
  mail manager's backend map. Selection is driven by `resource.BackendBinding.BackendKind`,
  exactly like `fake_local` today. Rollback for an integration is changing its binding back
  to `fake_local`/`native`.
- **Rationale**: Reuses the existing config-driven selection seam with zero change to the
  `Backend` interfaces or operation managers; additive and reversible per call/integration.
- **Alternatives considered**: A global feature flag switching all integrations — rejected;
  coarser blast radius and not per-integration reversible.

## R5. Per-call scoped credential injection

- **Decision**: The daemon resolves scoped, short-lived credential material from the Roadmap
  37 secret path at dispatch time and places it in the request envelope's credential field
  for that single call. The adapter holds it only for the call and never writes it to disk,
  logs, or process-durable state. Evidence/support/fixture/log output reuses existing
  redaction.
- **Rationale**: Keeps secret semantics owned by Roadmap 37 (Constitution V); per-call
  injection matches the "shared per-domain process with per-call tenant isolation" decision
  (spec Q-clarification) and prevents cross-tenant credential bleed.
- **Alternatives considered**: Injecting credentials at process start via env/args — rejected;
  it pins one tenant's credentials to a shared process and violates per-call isolation and
  no-persistence.

## R6. Single-ledger preservation and ambiguous-commit

- **Decision**: The shim returns normalized domain results only; the existing manager paths
  continue to own operation records, idempotency, artifacts, and live-validation
  classification. When an adapter fails during a side-effecting write, the shim surfaces a
  typed failure that the manager classifies as ambiguous-commit via the existing
  `live_validation` path — no second ledger, no assumed outcome.
- **Rationale**: Directly satisfies FR-002/003/007a/012 and SC-002; preserves replayability
  (Constitution III).
- **Alternatives considered**: Letting the adapter return ledger/idempotency state —
  rejected; that is the forbidden "second execution ledger".

## R7. Contract versioning

- **Decision**: The request/response envelopes carry a `contractVersion`. At adapter
  readiness the daemon performs an **exact-match** handshake; any mismatch refuses dispatch
  with a `contract_mismatch` diagnostic and attempts no operation (Q3).
- **Rationale**: Simplest deterministic rule while all adapters ship in-repo with the daemon;
  range/min-version negotiation is deferred until third-party adapters exist (out of scope).
- **Alternatives considered**: Semver range / minimum-version — rejected as premature.

## Resolved unknowns summary

| Unknown | Resolution |
|---------|-----------|
| RPC transport | Subprocess over stdio, newline-delimited JSON envelopes (stdlib only) |
| Reference adapter | In-repo Go binary `daemon/cmd/dope-integration-adapter` |
| Lifecycle/circuit-break | Reuse `capabilities.Supervisor` (backoff + `FailureCount>=5 → Failed`) |
| Backend selection | New `BackendKind` `adapter_rpc`; select via `BackendBinding`; rollback = rebind |
| Credentials | Per-call scoped injection in envelope; no adapter persistence; reuse redaction |
| Single ledger | Shim returns normalized results only; manager owns ledger + ambiguous-commit |
| Contract version | Exact-match handshake at readiness; mismatch refused |
