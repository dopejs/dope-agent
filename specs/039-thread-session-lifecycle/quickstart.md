# Quickstart: Daemon-Owned Thread And Session Lifecycle

This walkthrough verifies Roadmap 54 in the local test environment. Default to
`~/.dope-test` and `127.0.0.1:19192`; do not use production tenants or live connector
credentials unless a later release-readiness gate explicitly approves a safe live smoke.

## Prerequisites

- Branch: `039-thread-session-lifecycle`
- Active feature directory: `specs/039-thread-session-lifecycle`
- Test daemon environment only
- Fake or seeded evidence for at least two connector kinds
- Seeded sessions/runs plus at least one workflow, approval, foreground reply, and
  background delivery projection where possible

## Seed Assumptions

Default verification uses fake or seeded connector/runtime evidence. The minimum manual
walkthrough seed should include:

- One active channel-origin thread with a current session segment.
- One reset thread with prior and current session segments.
- One archived thread with prior runtime work.
- One reopened thread whose source remains eligible.
- One legacy or partial session projection.
- One ignored, blocked, duplicate, disabled, unsupported, or failed inbound message
  evidence record.
- One unknown-source, stale-source, or inaccessible-tenant-binding inbound message
  evidence record.
- One tenant with the default 90-day retention policy and one tenant with an authorized
  longer retention policy.

No live provider credentials, raw provider payloads, message bodies, or production
tenants are required.

Record connector kinds, seed source, operator, timestamp, and any skipped live validation
in final verification notes before closing the phase.

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
go test ./internal/threads ./internal/router ./internal/im ./internal/api ./internal/store ./internal/events ./internal/connectors ./internal/delivery ./internal/contracts
```

Expected coverage:

- Thread list/detail pagination and deterministic ordering.
- `credentials.inspect` gates for list/detail/source/runtime evidence inspection.
- `connectors.manage` gates for reset, archive, and reopen.
- Permission revocation reauthorizes list/detail/action requests and does not leak stale
  thread evidence.
- Reset preserves thread ID and creates a new active session segment.
- Archive blocks future continuation without cancelling already accepted runtime work.
- Reopen preserves lifecycle history and restores continuation only when source/session
  rules allow it.
- Audit-write failure denies lifecycle mutations without changing state.
- Source continuation maps tenant, connector, source account, and source conversation to
  one current thread.
- Accepted inbound messages attach to daemon-owned thread/session truth.
- Rejected, duplicate, unknown-source, stale-source, and inaccessible-tenant-binding
  inbound messages record routing evidence without creating misleading assistant work.
- Restart recovery preserves lifecycle state, source linkage, and runtime projections.
- Existing `/v1/sessions`, session schemas, session events, and SDK behavior remain
  compatible.
- Legacy sessions remain inspectable as partial evidence.
- Default retention, longer tenant retention-policy override, and redaction behavior are
  metadata-only.

## 3. Validate Contracts

From repo root:

```sh
make daemon-contract-test
```

Expected:

- API schemas for thread resources, detail, list, lifecycle action requests, lifecycle
  action responses, source linkage, and runtime projections validate.
- Event schemas for lifecycle, source linkage, runtime projection, retention, redaction,
  and audit-fail-closed evidence validate.
- Contract fixtures contain no raw provider payloads, disallowed message bodies, tokens,
  credentials, authorization grants, or cross-tenant identifiers.
- Existing session contract fixtures remain valid and are not rewritten by thread
  lifecycle schemas or events.

## 4. Run SDK, Web, And TUI Tests

From repo root:

```sh
pnpm test:clients
pnpm build
```

Expected SDK coverage:

- List threads with `limit`, `cursor`, `state`, and `sourceKind`.
- Get thread detail with session segments, source linkage, runtime projections, and
  lifecycle action history.
- Reset, archive, and reopen thread.
- Tenant headers are preserved.
- Denial responses do not leak inaccessible resource existence.
- Existing session SDK calls remain backward compatible.

Expected web coverage:

- Empty, loading, error, and permission-denied states.
- Stale views reauthorize before refreshes and lifecycle actions after permission changes.
- Paginated thread list with lifecycle state and source summary.
- Thread detail with session segments and separate runtime projection sections.
- Reset/archive/reopen controls and disabled/unsupported states.
- Clear labeling that lifecycle inspection is not memory recall.

Expected TUI/operator shell coverage:

- List recent threads.
- Inspect thread detail.
- Reset, archive, and reopen with visible permission and side-effect expectations.
- Trace source message to thread, session, runtime, approval, reply, and delivery facts.
- Re-run list/detail/action commands after permission changes and confirm no stale
  evidence is displayed.

## 5. Manual Test-Environment Walkthrough

Use fake or seeded connector evidence for at least two connector kinds, such as Telegram
and Matrix or Slack and Matrix.

1. Open the web product surface or operator shell against the test daemon.
2. Confirm the thread list shows active, reset, reopened, archived, and partial legacy
   items for the active tenant only.
3. Traverse thread pages and confirm no missing or duplicated thread entries. Record the
   elapsed time to find and open a target recent tenant thread; it must be 2 minutes or
   less.
4. Open an active channel-origin thread.
5. Confirm source linkage shows connector, source account, source conversation, source
   message evidence, and current session segment using redacted metadata.
6. Confirm runtime projections show sessions, runs or workflows, approvals, foreground
   replies, and background deliveries as separate facts.
7. Reset the thread.
8. Confirm the thread ID remains stable and a new active session segment is created.
9. Send or simulate another accepted inbound message for the same source conversation.
10. Confirm it attaches to the same current thread and new active session segment.
11. Archive the thread while an accepted run, approval, workflow, reply, or delivery is
    present.
12. Confirm future continuation is blocked and already accepted runtime work is not
    cancelled by the archive action.
13. Reopen the thread when source and session rules allow it.
14. Confirm the archive and reopen actions remain visible in lifecycle history.
15. Inspect a legacy or partial session projection and confirm missing linkage is labeled
    rather than invented.
16. Revoke or simulate loss of `credentials.inspect` and `connectors.manage`, then retry
    list/detail/reset/archive/reopen from API, web, and TUI surfaces. Confirm each request
    reauthorizes and returns a stable denial without stale evidence.
17. Trace a representative channel incident from source message to thread, session, run or
    workflow, approval, reply, and delivery facts. Record elapsed time; it must be 5
    minutes or less.
18. Confirm all views label the feature as lifecycle/evidence inspection, not memory
    recall, semantic summary, or context packing.

## 6. Retention And Redaction Checks

Seed lifecycle, source, and runtime projection evidence older than 90 days and run the
retention application path. Include at least one tenant with the default policy and one
tenant with an authorized longer retention policy.

Expected:

- Expired lifecycle, source, and runtime projection evidence disappears from normal
  inspection under the default 90-day policy.
- Evidence covered by an authorized longer tenant retention policy remains inspectable
  until that policy's expiry.
- Audit evidence for retention application remains visible to authorized users.
- Redaction tests find zero raw provider payloads, disallowed message bodies, tokens,
  authorization grants, credentials, or cross-tenant identifiers.
- Redaction failures emit metadata-only audit evidence and suppress unsafe details.

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
Record measured list/detail and source-to-runtime trace timings in this quickstart before
closing the phase.

## Verification Notes: 2026-05-11

Environment: local test workspace, fake/seeded daemon evidence only. Production state and
live connector credentials were not used.

Command results:

- `cd daemon && go test ./internal/im ./internal/threads ./internal/store ./internal/api ./internal/delivery`: PASS.
- `cd daemon && go test ./internal/store ./internal/router ./internal/connectors ./internal/api ./internal/events ./internal/app ./internal/im`: PASS.
- `cd daemon && go test ./internal/threads ./internal/store ./internal/api`: PASS.
- `make daemon-contract-test`: PASS.
- `pnpm test:clients`: PASS.
- `pnpm build`: PASS.
- `cd daemon && go test ./...`: PASS after adding Roadmap 54 thread tables and events to the schema inventory.
- `cd daemon && go mod tidy`: PASS, with no `daemon/go.mod` or `daemon/go.sum` drift.

Measured timing notes:

- Focused thread lifecycle API/store/IM test path completed within the automated test
  budget; thread list/detail and trace assertions are covered by seeded in-memory
  fixtures.
- Client trace rendering was covered by `pnpm test:clients`; web and TUI tests exercised
  detail, source trace, runtime trace, non-memory labeling, and retention/redaction
  metadata.
- Source-to-runtime trace timing is recorded as automated fixture validation only. No
  live connector UI timing was collected.

Permission-change checks:

- API tests cover `credentials.inspect` denial for list/detail and `connectors.manage`
  denial for lifecycle mutations.
- Web and TUI tests cover stale/reauthorized lifecycle views and command output without
  stale evidence reuse.

Retention and redaction checks:

- Store tests cover default 90-day expiry, longer tenant retention override, expired
  lifecycle/source/runtime evidence filtering, and retained metadata visibility.
- Redaction/projection tests reject unsafe provider payload, source identity, message
  body, and memory-behavior fields.

Skipped live validation:

- Owner: local implementation.
- Reason: Roadmap 54 verification uses fake/seeded connector evidence; live connector
  credentials and production tenants are explicitly out of scope for this pass.
- Remaining risk: provider-specific live replay timing still needs a later safe-live
  smoke if release gates require it.
- Validation timestamp: 2026-05-11.
- Retention expiry: default 90-day lifecycle evidence policy unless tenant override.
- Redaction status: metadata-only, redacted or suppressed.
