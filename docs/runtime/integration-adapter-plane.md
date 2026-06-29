# Integration Adapter Plane — Operator Runbook

Roadmap 59. Upstream spec: `docs/specs/044-external-integration-adapter-plane.md`.
Feature spec: `specs/044-integration-adapter-plane/`.

## What it is

A supervised, out-of-process plane that real personal-data integrations (calendar, mail)
plug into. The daemon retains all operation/ledger/evidence/idempotency/artifact truth; an
adapter performs provider request/response mapping only, behind the capability RPC contract
in `schemas/capability/integration-adapter/`.

Key packages:

- `daemon/internal/integrations/adapterrpc` — envelopes, codec, transport, client,
  credential resolver, conformance harness.
- `daemon/internal/integrations/adapterref` — reference adapter skeleton (no real provider).
- `daemon/cmd/dope-integration-adapter` — reference adapter binary.
- `daemon/internal/{calendar,mail}/adapter_backend.go` — `Backend` shims (kind `adapter_rpc`).
- `daemon/internal/capabilities/adapter_runtime.go` — supervised lifecycle bridge.

## Backend selection

An integration uses the adapter plane when its `BackendBinding.BackendKind == adapter_rpc`.
Otherwise it uses the in-daemon `fake_local` backend, which remains the default in every
environment. Both backends write to the same single operation ledger.

## Enabling adapters

Off by default. Set `DOPE_INTEGRATION_ADAPTER=<path-to-adapter-binary>` before daemon start
to spawn supervised calendar and mail adapters and register the `adapter_rpc` backends. With
the variable unset, daemon behavior is unchanged (fake backend only). The reference binary:

```bash
go build -o /tmp/dope-integration-adapter ./daemon/cmd/dope-integration-adapter
DOPE_INTEGRATION_ADAPTER=/tmp/dope-integration-adapter make daemon-run-test
```

## Readiness, health, and circuit-break

Adapters register as capabilities (kind `integration_adapter`) and are visible on the
capability surface. Readiness gates on an exact contract-version handshake (`Ready`):

- handshake/heartbeat ok → `healthy` → readiness `ready`.
- failures → exponential backoff (`backing_off`); after 5 failures the supervisor
  circuit-breaks to `failed`, surfaced as readiness `unavailable` — no further auto-restart
  until repair. (Restart resets backoff and re-probes.)

A contract-version mismatch is refused at readiness with `ErrContractMismatch` before any
operation is attempted.

## Failure classification

Adapter outcomes map onto existing diagnostics / live-validation:

- clean failure response (auth/scope/rate_limited/unavailable/malformed-request) → confirmed
  non-commit.
- crash, hang past the daemon-supplied deadline, transport break, or undecodable response →
  **ambiguous-commit** for side-effecting writes (operation recorded once as failed with
  `FailureClass = ambiguous_commit`). The daemon never assumes committed or failed.

## Credentials

Resolved per call from the resource (scoped, short-lived), failing closed if resolution
errors. Adapters must not persist credentials or content. Provider-backed secret resolution
(Roadmap 37) is installed where real providers are wired (Roadmap 60/63); the reference
adapter needs none.

## Rollback

Re-bind the integration's `BackendBinding.BackendKind` to `fake_local` (or unset
`DOPE_INTEGRATION_ADAPTER` and restart). No data migration is involved; the operation ledger
is unchanged.

## Verification

```bash
make daemon-contract-test
cd daemon && go test ./internal/integrations/adapterrpc/... ./internal/integrations/adapterref/... \
  ./internal/calendar/... ./internal/mail/... ./internal/capabilities/...
```
