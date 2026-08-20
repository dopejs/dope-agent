# Quickstart: Channel Connector Conformance

## Environment

Use the local test environment by default.

```bash
make daemon-run-test
make daemon-test-status
```

Do not use live connector credentials, production tenants, or `~/.kura` for Phase 48
acceptance unless an operator explicitly chooses a separate live validation path.

## Targeted Verification

Run targeted daemon tests while implementing:

```bash
cd daemon
go test ./internal/connectors ./internal/im ./internal/delivery ./internal/store ./internal/api ./internal/contracts
```

Required targeted coverage:

- fake connector conformance matrix;
- Discord connector regression;
- tenant-scoped account binding and permission denial;
- standard inbound message identity and equivalent durable identity;
- duplicate retry and restart replay suppression;
- direct, group, mention, room, and thread routing outcomes;
- reply progression degradation;
- foreground reply vs background delivery separation;
- required connector diagnostic classifications;
- 15-minute diagnostic staleness;
- current diagnostic truth on connector action failure;
- redaction fail-closed behavior;
- 90-day retention expiry.

## Contract Verification

Run schema and event contract validation after changing `schemas/` or event payloads:

```bash
make daemon-contract-test
```

If SDK, web, or TUI surfaces are changed, also run:

```bash
pnpm test:clients
pnpm build
```

## Full Daemon Verification

Before treating implementation as complete:

```bash
cd daemon
go test ./...
go mod tidy
```

Then confirm `daemon/go.mod` and `daemon/go.sum` did not change unexpectedly.

## Manual Test-Environment Walkthrough

1. Start the daemon in the test environment.
2. Register or seed a fake connector capability profile that passes all core invariants.
3. Run fake conformance scenarios and confirm every core invariant passes and every
   provider-specific surface is supported, limited, or unsupported with no silent skips.
4. Replay duplicate inbound events and confirm only one assistant reply is recorded.
5. Trigger blocked group/room/thread routing and confirm no assistant reply is sent.
6. Trigger connector diagnostics for auth missing, permission missing, rate limited,
   provider unavailable, network failed, unsupported capability, blocked route,
   duplicate inbound, reply failed, and unknown connector failure.
7. Confirm cached diagnostics older than 15 minutes are marked stale.
8. Confirm redaction-uncertain diagnostic evidence is suppressed and records a
   redaction-failure outcome.
9. Confirm conformance and diagnostic evidence expires from normal inspection after the
   90-day default retention window unless an authorized longer policy applies.
10. Run Discord regression with fake transport and confirm Discord either passes required
    contract areas or records explicit unsupported/limited provider-specific surfaces.

## Rollback Check

Rollback should be possible by disabling conformance gating/projections and diagnostic
publication while preserving existing Discord foreground replies, existing connector
routes, and already-written redacted conformance/diagnostic evidence until retention
expiry.

## Implementation Notes

Implemented Phase 48 as additive connector contract infrastructure:

- API and event schemas for capability profiles, conformance results, diagnostics,
  account binding summaries, routing outcomes, duplicate detection, foreground reply
  failures, and connector delivery separation.
- R48 SQLite migration for conformance results, diagnostic states, redaction failures,
  delivery boundaries, and standard connector message identity columns.
- Shared conformance and diagnostic helpers, Discord capability projection, standard
  inbound identity normalization, tenant-scoped message dedupe, and retention-aware store
  accessors.
- Operator-facing diagnostics and delivery-boundary documentation for future provider
  handoff.

Residual risk: Phase 48 provides shared contract and Discord/fake baseline behavior, not
Telegram, Slack, WhatsApp, or Matrix provider implementations. Live connector validation
remains out of scope unless explicitly run in a separate live test path.
