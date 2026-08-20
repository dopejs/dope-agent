# Implementation Plan: Real Calendar Provider Closure (Feishu/Lark)

**Branch**: `main` | **Spec**: [spec.md](./spec.md) | **Upstream**: [docs/specs/045-real-calendar-provider-closure.md](../../docs/specs/045-real-calendar-provider-closure.md)
**Phase / Roadmap**: Phase 60 — Roadmap 60 (first real calendar provider closure)

## Summary

Close one real calendar provider — **Feishu/Lark Calendar** — end to end, implemented as an
adapter on the external integration adapter plane (Roadmap 59), not as an in-process
`calendar.Backend`. The daemon-owned operation ledger, idempotency, artifacts, diagnostics
vocabulary, and live-validation classification are reused unchanged. The fake backend stays
mandatory and green. Net deliverables:

1. A real Feishu/Lark calendar **provider** (`internal/integrations/providers/feishulark`)
   that maps the Feishu Open Platform Calendar API to the calendar domain resources
   (account projection, event list/detail, busy/free, timed create/update/cancel), with an
   injectable HTTP transport so it is exercisable against synthetic/recorded responses in CI.
2. A reusable **provider serve harness** (`internal/integrations/adapterprovider`) that runs
   the capability RPC stdio loop against a real provider handler, replacing the empty-payload
   reference adapter for real providers while preserving the contract (version handshake,
   deadline, failure-kind mapping, redacted diagnostics).
3. The `kura-integration-adapter` binary gains a real provider mode selected by
   `KURA_ADAPTER_PROVIDER=feishu_lark` (default stays the reference skeleton).
4. App wiring: a `feishu_lark` secret-backed `IntegrationCredentialFetcher` so per-call
   scoped tokens reach the adapter; selection of the real provider when configured.
5. Stable diagnostics: the calendar adapter shim maps `adapterrpc.FailureKind`
   (auth/scope/rate_limited/unavailable) to stable operation failure classes so OAuth/scope/
   token failures land on the existing `feishu_lark` diagnostics reason vocabulary.
6. A calendar real-account smoke entry in `opsreadiness` that runs against safe operator
   credentials when available and otherwise records an explicit structured skip.

## Technical Context

- **Language / runtime**: Go 1.24 (daemon). No client/web changes — additive daemon + schema
  only; existing calendar API/event schemas are reused unchanged.
- **Primary dependencies**: `internal/integrations/adapterrpc` (RPC contract, client,
  credential resolver), `internal/integrations/adapterref` (reference skeleton, test pipe
  client), `internal/calendar` (Backend interface, Manager ledger), `internal/capabilities`
  (adapter supervision/runtime), `internal/integrations` (diagnostics classifier,
  `feishu_lark` provider kind), `internal/opsreadiness` (real-account smoke).
- **Storage**: none new — account projections, operations, artifacts reuse existing
  persistence shapes. Adapter is stateless.
- **Testing**: provider unit tests over an `httptest` Feishu server (synthetic responses for
  read/write/failure); adapter-shim tests via in-process pipe client; manager/ledger tests
  proving single-ledger + ambiguous-commit + diagnostics mapping; `make daemon-contract-test`
  for any additive schema; existing fake-backend suite stays green.
- **Performance**: per-call deadline bounded daemon-side (`adapterrpc` default 30s); adapter
  holds no durable state.
- **Constraints**: no second calendar execution ledger; no new calendar resource shapes; no
  attendee/RSVP/recurrence/all-day/alternate-calendar semantics (rejected with existing
  out-of-scope reasons by the Manager before dispatch); zero credential/token material in any
  log, event, diagnostic, artifact, or smoke output.

## Constitution Check

- **Roadmap closure**: satisfies the upstream DoD — one real calendar provider hosted-ready
  for the Roadmap 29 capability set; fake backend remains required.
- **Production-grade**: failure-closed credential resolution, deadline-bounded calls,
  ambiguous-commit safety, redacted diagnostics, supervised adapter lifecycle.
- **Contracts first**: provider maps onto existing calendar/integration resources; any schema
  change is additive (provider-kind enum on the integration-adapter capability schema) and
  validated by contract tests.
- **Verification**: unit + adapter + manager + live-validation + smoke coverage; the existing
  fake-backend regression suite is unchanged.
- **Environment**: `KURA_ENV=test` defaults to the fake backend with no live credentials; the
  real provider runs only when `KURA_INTEGRATION_ADAPTER` + `KURA_ADAPTER_PROVIDER=feishu_lark`
  are explicitly set with operator-provided safe credentials.

## Project Structure

```
specs/045-real-calendar-provider-closure/
  spec.md  plan.md  tasks.md  checklists/

daemon/
  internal/integrations/providers/feishulark/   # NEW: real provider mapping
    client.go            # Feishu Open API HTTP client (injectable base URL + http.Client)
    calendar.go          # calendar operation mapping (Feishu JSON <-> calendar types)
    credential.go        # parse scoped credential envelope (access token, scopes)
    errors.go            # provider error -> adapterrpc.FailureKind + redacted diagnostic
    *_test.go            # httptest-backed read/write/failure unit tests
  internal/integrations/adapterprovider/         # NEW: real-provider stdio serve harness
    serve.go             # RPC loop dispatching to a provider Handler
    serve_test.go
  internal/calendar/adapter_backend.go           # EDIT: FailureKind -> stable failure class
  internal/opsreadiness/real_account_smoke.go    # (reused) calendar smoke status
  internal/opsreadiness/calendar_smoke.go        # NEW: calendar smoke matrix + structured skip
  internal/app/app.go                            # EDIT: feishu_lark credential fetcher + provider select
  cmd/kura-integration-adapter/main.go           # EDIT: KURA_ADAPTER_PROVIDER real mode

schemas/capability/integration-adapter/          # additive provider-kind enum if needed
docs/providers/                                   # provider-architecture + real-account-smoke notes
```

## Complexity Tracking

No constitution violations. The provider is additive behind the existing `calendar.Backend`
contract and the existing `adapter_rpc` backend kind; no existing seam shape changes.
</content>
</invoke>
