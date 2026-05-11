# Quickstart: Channel Management And Repair UX

This walkthrough verifies Roadmap 53 in the local test environment. Default to
`~/.dope-test` and `127.0.0.1:19192`; do not use production tenants or live connector
credentials unless a later release-readiness gate explicitly approves a safe live smoke.

## Prerequisites

- Branch: `038-channel-management-repair`
- Active feature directory: `specs/038-channel-management-repair`
- Test daemon environment only
- At least two connector kinds available through fake or seeded test evidence

## Seed Assumptions

Default verification uses fake or seeded connector evidence for at least two connector
kinds. The minimum manual walkthrough seed should include one ready connector and one
action-required or degraded connector. No live provider credentials, raw provider
payloads, channel message bodies, or production tenants are required.

Record the connector kinds, seed source, operator, timestamp, and any skipped live
validation in the final verification notes before closing the phase.

## 1. Start The Test Daemon

```sh
make daemon-run-test
```

In another shell:

```sh
make daemon-test-status
```

Expected:

- Daemon responds on the test bind address.
- Data directory is `~/.dope-test`.
- No production connector credentials are required.

## 2. Run Focused Daemon Tests

From `daemon/`:

```sh
go test ./internal/connectors ./internal/api ./internal/store ./internal/delivery ./internal/contracts
```

Expected coverage:

- Paginated deterministic channel connector list.
- Permission gates for `credentials.inspect`, `integrations.diagnostics.read`,
  `connectors.manage`, and `secrets.manage`.
- Disable and re-enable behavior.
- Disabled inbound suppression.
- Background delivery eligibility blocking.
- Repair-to-setup linkage.
- Route policy updates.
- Foreground reply and background delivery separation.
- Support evidence redaction and 90-day retention.
- Audit fail-closed mutation behavior.

## 3. Validate Contracts

From repo root:

```sh
make daemon-contract-test
```

Expected:

- API schemas for channel management resources and responses validate.
- Event schemas for management, audit, redaction, and retention evidence validate.
- Contract fixtures contain no raw provider payloads, channel message bodies, tokens, or
  authorization grants.

## 4. Run SDK And Web Tests

From repo root:

```sh
pnpm test:clients
pnpm build
```

Expected SDK coverage:

- List channel connectors with `limit` and `cursor`.
- Get connector management detail.
- Disable and re-enable connector.
- Start repair action.
- Read diagnostics and support evidence.
- Tenant headers are preserved.

Expected web coverage:

- Empty state.
- Paginated fleet list with attention-needed ordering.
- Connector detail.
- Disable and re-enable controls.
- Repair from diagnostic next step.
- Foreground reply and background delivery status as separate sections.
- Metadata-only support evidence.
- Permission-denied and unsupported-action states.

## 5. Manual Test-Environment Walkthrough

Use fake or seeded connector evidence for at least two connector kinds, such as Telegram
and Matrix or Slack and Matrix.

1. Open the web product surface against the test daemon.
2. Confirm the channel list shows all tenant connectors across pages with default page
   size 20.
3. Confirm action-required, unavailable, and degraded connectors appear before disabled
   and ready connectors.
4. Open a connector detail view.
5. Confirm setup state, health, diagnostic freshness, route summary, next action,
   foreground reply status, background delivery status, and support evidence availability.
6. Disable a ready connector.
7. Send or simulate an inbound provider event for the disabled connector.
8. Confirm no agent run is created and the route decision is recorded as disabled,
   ignored, or blocked.
9. Confirm the disabled connector is not eligible for new background delivery.
10. Re-enable the connector after current setup, health, diagnostic, and route checks
    pass.
11. Induce a repairable diagnostic, then start repair from the next action.
12. Confirm repair links to setup, reconnect, route revalidation, or diagnostic rerun as
    appropriate.
13. Inspect support evidence and confirm it is metadata-only.

## 6. Retention And Redaction Checks

Seed support evidence older than 90 days and run the retention application path.

Expected:

- Expired diagnostic, repair, routing, reply, delivery, and support evidence disappears
  from normal inspection unless a longer authorized tenant retention policy applies.
- Audit evidence for retention application remains visible to authorized users.
- Redaction tests find zero raw provider payloads, channel message bodies, tokens,
  authorization grants, or prohibited route details.

## 7. Final Verification

Before considering implementation complete:

```sh
cd daemon && go test ./...
cd daemon && go mod tidy
make daemon-contract-test
pnpm test:clients
pnpm build
```

Record any skipped live connector validation as a structured skip with owner, reason,
remaining risk, validation timestamp, retention expiry, and redaction status.

### Verification Results - 2026-05-10

- Focused daemon: `go test ./internal/connectors ./internal/store ./internal/api ./internal/delivery ./internal/contracts ./internal/events ./internal/setupwizard` passed.
- Contract: `make daemon-contract-test` passed.
- Clients: `pnpm test:clients` passed, including SDK, web, TUI, build, and roadmap 7 smoke.
- Build: `pnpm build` passed.
- Full daemon: `cd daemon && go test ./...` passed.
- Module hygiene: `cd daemon && go mod tidy` completed with no `go.mod` or `go.sum` diff.

### Verification Results - 2026-05-11

- Focused daemon: `go test ./internal/connectors ./internal/store ./internal/api ./internal/events` passed.
- Full daemon: `cd daemon && go test ./...` passed after intentionally updating the
  R37 boundary golden for the new `Supervisor.WithConnectorMutation` method and
  supervisor mutation-lock field.
- Contract: `make daemon-contract-test` passed.
- Clients: `pnpm test:clients` passed.
- Build: `pnpm build` passed.
- Module hygiene: `cd daemon && go mod tidy` completed with no `go.mod` or `go.sum`
  diff.

### Gap Closure Verification - 2026-05-11

- Focused API: `go test ./internal/api -run 'TestChannelManagementDisableFailsClosed|TestConnectorIngressRecordsDisabled|TestChannelManagementSupportEvidenceAggregates|TestChannelManagementRoutePolicy|TestChannelManagementSupportEvidence'` passed.
- Focused IM: `go test ./internal/im -run 'TestMessageLoopProcessesMatrixInbound|TestMessageLoopMarksFailureWhenReplySendFails|TestMessageLoopStreamsReply'` passed.
- Focused delivery: `go test ./internal/delivery -run TestDeliveryManagerBlocksDisabled` passed.
- Web action wiring: `pnpm --dir web exec vitest run src/app/App.test.tsx` passed.
- Web type check: `pnpm --dir web exec tsc --noEmit` passed.
- Full daemon: `cd daemon && go test ./...` passed.
- Contract: `make daemon-contract-test` passed.
- Clients: `pnpm test:clients` passed.
- Build: `pnpm build` passed.
- Module hygiene: `cd daemon && go mod tidy` completed.

### Walkthrough Notes - 2026-05-10

- Connector kinds covered by automated fake/seeded evidence: Discord, Telegram, Slack,
  and Matrix.
- Seed source: package-local fake supervisors, SQLite test stores, connector diagnostic
  fixtures, delivery connector adapters, and redacted contract JSON fixtures.
- Manual browser walkthrough: skipped for this implementation pass because the local
  daemon was not started in this turn. Remaining risk is limited to visual/manual timing
  evidence for the 2-minute fleet inspection and 5-minute support reconstruction targets;
  API, SDK, web render, persistence, contract, and delivery gates are covered by tests.
- Live validation: skipped. No live provider credentials, raw provider payloads, message
  bodies, production tenants, or safe-live connector authorization were used.
- Retention/redaction: redacted fixture and support-evidence tests passed; retention
  expiry was verified through the SQLite support evidence accessor test.

### Walkthrough Notes - 2026-05-11

- Test daemon: `make daemon-run-test` started `127.0.0.1:19192` with `~/.dope-test`;
  `make daemon-test-status` returned `{"ok":true,"service":"dope"}`.
- Seed source: local pairing token against the test daemon plus two seeded connector
  registrations, `telegram-r53-1778431447975` and `matrix-r53-1778431447975`.
- Connector kinds covered: Telegram ready connector and Matrix failed/action-required
  connector; no production tenant, live provider credential, raw provider payload, or
  message body was used.
- Fleet inspection timing: `GET /v1/channel-management/connectors?limit=20` returned
  HTTP 200 with 3 tenant-scoped items in 1 ms, below the 2-minute target.
- Disable and inbound suppression: disabling the Telegram connector returned
  `enablementState=disabled`; a disabled ingress attempt returned HTTP 409 before agent
  run creation.
- Re-enable: `POST /re-enable` restored the Telegram connector to `ready` and
  `deliveryEligible=true`.
- Support reconstruction timing: `GET /support-evidence` for the Matrix connector
  returned HTTP 200 with `redactionStatus=redacted` in 1 ms, below the 5-minute target.

### Cross-Artifact Review - 2026-05-10

Spec, plan, contract, and tasks agree on the main API/SDK/web surface, permission gates,
15-minute diagnostic staleness, 90-day retention, metadata-only support evidence, and
test-environment-only verification. Remaining implementation gaps are tracked in
`tasks.md`: manual timed walkthrough evidence, full persisted route/reply/delivery
outcome link projections, runtime event-bus emission for connector management events,
store-owned connector projection pagination, and complete route-policy editing UI.

### Cross-Artifact Review - 2026-05-11

The previously tracked gaps are now implemented and verified: store-owned paginated
connector projection reads, disabled ingress suppression in the connector supervisor,
persisted route policy snapshots plus route/reply/delivery outcomes, route-policy
editing and outcome sections in the web UI, connector management event constructors and
support-evidence event publication, and the timed two-connector test-environment
walkthrough.

## Rollback Check

Verify rollback can:

- Hide or disable new channel management API routes and web controls.
- Block new management mutations.
- Preserve existing connector runtime, setup, diagnostics, ingress, delivery, and
  conformance behavior.
- Preserve already-written audit and support evidence until retention expiry.
