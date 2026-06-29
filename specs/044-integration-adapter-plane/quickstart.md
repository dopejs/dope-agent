# Quickstart: External Integration Adapter Plane

Audience: a developer verifying the plane locally. Default environment is `~/.dope-test`
(`127.0.0.1:19192`). The test env defaults to the in-daemon **fake** backend; the adapter
plane is opt-in per integration.

## Build

```bash
make daemon-build                     # daemon binary
go build ./daemon/cmd/dope-integration-adapter   # reference adapter skeleton (no real provider)
```

## Run the contract + plane tests

```bash
make daemon-contract-test             # validates schemas/capability/integration-adapter/*
cd daemon && go test ./internal/integrations/adapterrpc/...   # transport, client, conformance, failure modes
cd daemon && go test ./internal/calendar/... ./internal/mail/...   # shim + single-ledger + failure isolation
cd daemon && go test ./internal/capabilities/...              # adapter runtime lifecycle + circuit-break
```

## Exercise the plane against the reference adapter

1. Start the test daemon: `make daemon-run-test`.
2. Create/bind a calendar (or mail) integration with `BackendBinding.BackendKind = adapter_rpc`
   pointing at the reference adapter capability id.
3. Confirm readiness: the adapter registers with the capability supervisor and reaches
   `ready` after the exact contract-version handshake; an adapter-health event is emitted.
4. Run read + write operations; confirm each result is recorded **once** in the daemon
   operation ledger with unchanged operation/evidence/idempotency/artifact semantics.

## Verify the guarantees

- **Failure isolation**: drive the reference adapter into crash / hang / timeout / auth /
  malformed (seeded modes) and confirm the daemon stays healthy, the in-flight write is
  classified ambiguous-commit, and a diagnostic is emitted (SC-001, FR-007/007a/007b).
- **Single ledger**: run operations through both the adapter and the fake backend; confirm
  one daemon-owned ledger, no second ledger or duplicate side effect (SC-002, FR-012).
- **Credential hygiene**: after a call, inspect adapter state/logs and confirm no credential
  material and no raw provider content persist; confirm redaction holds (SC-004, FR-006).
- **Circuit-break**: force ≥5 failures and confirm the supervisor stops auto-restart, marks
  the integration unavailable, and surfaces the degraded state (SC-005, FR-004a, Q2).
- **Contract version**: point the daemon at an adapter advertising a different
  `contractVersion`; confirm dispatch is refused with `contract_mismatch` before any
  operation (SC-007, FR-008, Q3).

## Rollback

Change the integration's `BackendBinding.BackendKind` back to `fake_local` (or `native`).
No data migration is involved; the operation ledger is unchanged.
