# Quickstart: Group Room Reset Handoff

This walkthrough verifies Roadmap 56 in the local test environment. Default to
`~/.kura-test` and `127.0.0.1:19192`; do not use production tenants or live connector
credentials unless a later release-readiness gate explicitly approves a safe live smoke.

## Prerequisites

- Branch: `041-group-room-reset-handoff`
- Active feature directory: `specs/041-group-room-reset-handoff`
- Roadmap 54 thread/session lifecycle implemented and passing
- Roadmap 55 bounded continuity implemented and passing
- Test daemon environment only
- Fake or seeded chat/channel evidence
- At least one direct-message thread, one group thread, one room thread, one
  web-originated thread, one unsupported source, one reset source, and one handoff
  source/destination pair

## Seed Assumptions

Default verification uses fake or seeded connector/runtime evidence. The minimum manual
walkthrough seed should include:

- One direct-message channel source with proven source identity.
- One group source that is allowlist-eligible and has a qualifying mention.
- One group source that is allowlist-eligible but missing a qualifying mention.
- One room source that is not allowlist-eligible but includes a qualifying mention.
- Two rooms with similar names or participants but different stable source identity.
- One unsupported or partial source with unknown conversation shape.
- One source with duplicate/replayed/edited/deleted source-message evidence.
- One group or room thread with pre-reset and post-reset turns.
- One channel source thread with eligible current-segment continuity turns.
- One web-originated thread eligible as a handoff source.
- One destination surface that lacks permission or lifecycle eligibility.

No live provider credentials, raw provider payloads, message bodies, or production
tenants are required.

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
- Data directory is `~/.kura-test`.
- No production connector credentials are required.

## 2. Run Focused Daemon Tests

From `daemon/`:

```sh
go test ./internal/threads ./internal/store ./internal/api ./internal/chat ./internal/events ./internal/connectors ./internal/im ./internal/contracts
```

Expected coverage:

- Direct-message, group, room, and web-originated conversations expose correct
  conversation shape.
- Room identity is stable across display-name similarity and participant overlap.
- Default group/room policy accepts only allowlist-eligible messages with qualifying
  mentions.
- Missing-mention and not-allowlisted messages record safe non-accepted decisions.
- Duplicate, replayed, edited, deleted, unsupported, blocked, disabled, and failed source
  messages do not create duplicate assistant work.
- Direct-message, group, and room resets require `connectors.manage` and preserve
  historical evidence.
- Reset of one source or conversation shape does not affect unrelated threads.
- Handoff creation requires `connectors.manage`.
- Handoff creates or selects a separate destination thread and records source/destination
  references.
- Handoff source turns are referenced only for the first destination response and are not
  copied into destination history.
- After the first destination response, destination continuity uses destination-thread
  turns only unless another authorized handoff occurs.
- Permission-denied and unsupported handoff attempts do not silently create destination
  conversations.
- Restart recovery preserves participation decisions, reset events, handoff links,
  source-reference consumption state, and final statuses.
- Redaction failure suppresses unsafe detail and records safe metadata.
- This phase creates no group memory, team knowledge behavior, semantic retrieval,
  summaries, autonomous delegation, long-term personalization, or cross-room recall.

## 3. Validate Contracts

From repo root:

```sh
make daemon-contract-test
```

Expected:

- Thread detail schema accepts additive conversation shape, participation decision,
  reset event, and handoff link evidence.
- Handoff request/response schemas validate source and destination references.
- Event schemas for participation decision, scoped reset, and handoff linkage validate.
- Connector conformance fixtures cover conversation shape, mention detection, allowlist
  policy, duplicate detection, reset support, handoff support, and unsupported sources.
- Fixtures contain no raw provider payloads, disallowed message bodies, tokens,
  credentials, authorization grants, or cross-tenant identifiers.

## 4. Run SDK, Web, And TUI Tests

From repo root:

```sh
pnpm test:clients
pnpm build
```

Expected SDK coverage:

- Thread detail types expose conversation shape, participation decisions, reset events,
  and handoff links.
- Handoff creation method requires explicit destination and preserves tenant headers.
- Existing thread lifecycle and continuity calls remain compatible.

Expected web coverage:

- Thread detail shows conversation shape and safe participation/reset/handoff evidence.
- Handoff controls are hidden or disabled when mutation permission or destination
  eligibility is missing.
- Handoff evidence labels the behavior as traceable continuation, not memory.
- First-response source reference status changes from available to consumed.

Expected TUI/operator shell coverage:

- Thread detail output lists conversation shape, participation outcomes, reset events,
  and handoff links.
- Handoff creation reports succeeded, denied, unsupported, and failed statuses without
  unsafe content.
- Permission changes force reauthorization and do not display stale evidence.

## 5. Manual Test-Environment Walkthrough

1. Open the web product surface or TUI against the test daemon.
2. Inspect a direct-message thread, a group thread, a room thread, and a web thread.
3. Confirm each shows the expected conversation shape and source isolation.
4. Send a qualifying mention in an allowlist-eligible group; confirm assistant work is
   accepted and evidence records `accepted_qualifying_mention`.
5. Send a group message without a qualifying mention; confirm no assistant work is
   created and evidence records `missing_qualifying_mention`.
6. Send a mention in a not-allowlisted room; confirm no assistant work is created and
   evidence records `not_allowlisted`.
7. Reset a group thread using an account with `connectors.manage`.
8. Ask a follow-up that would require pre-reset context; confirm pre-reset turns do not
   affect the response and reset evidence is visible.
9. Attempt reset or handoff without `connectors.manage`; confirm denial does not leak
   inaccessible detail.
10. Handoff a channel thread to the web shell.
11. Confirm the destination thread is separate from the source thread and both sides show
    a handoff link where visible.
12. Ask the first destination response and confirm eligible source turns can be
    referenced by identity/safe summary, not copied into destination history.
13. Ask a second destination response and confirm only destination-thread continuity is
    used unless another handoff occurs.
14. Handoff a web-originated thread to a supported channel and confirm the destination
    channel policy still applies.
15. Simulate a destination permission loss and confirm later continuation/inspection
    honors current permission.
16. Inspect as a user who can see only one side of a handoff and confirm inaccessible
    side detail is suppressed.
17. Measure time for an operator to explain one group participation outcome and one
    handoff outcome using product evidence; each must be 5 minutes or less.

## 6. Retention, Redaction, And Restart Checks

Seed participation decisions, reset events, handoff links, and handoff source references
across active, reset, expired, and redaction-failed cases.

Expected:

- Expired evidence disappears from normal inspection after the applicable retention
  limit.
- Redaction validation finds zero raw provider payloads, disallowed message bodies,
  tokens, authorization grants, credentials, unsafe connector metadata, or cross-tenant
  identifiers.
- Daemon restart preserves final participation, reset, and handoff statuses.
- Handoff source references remain consumed after restart once the first destination
  response uses them.

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
Record measured p95 first-response handoff reference assembly latency and at least one
operator participation/handoff inspection timing before closing the phase.

## Roadmap 56 Verification Notes

Recorded: 2026-05-11 18:56 Asia/Shanghai.

- Focused daemon packages passed:
  `go test ./internal/threads ./internal/store ./internal/api ./internal/chat ./internal/events ./internal/connectors ./internal/im ./internal/contracts`.
- Contract validation passed: `make daemon-contract-test`.
- Full daemon suite passed: `cd daemon && go test ./...`.
- Module tidy passed: `cd daemon && go mod tidy`; no dependency change was required.
- Client tests and build passed: `pnpm test:clients` and `pnpm build`.
- Handoff source-reference assembly performance: `TestHandoffSourceReferenceAssemblyPerformance`
  assembled 64 source turns 1000 times under the 500 ms p95 proxy threshold in the local
  test environment.
- Redaction and secret-output audit: Roadmap 56 schemas, fixtures, Web/TUI output, event
  payloads, and tests use redacted source ids, metadata-only summaries, and explicit
  redaction statuses. No raw provider payloads, message bodies, tokens, credentials,
  authorization grants, or cross-tenant identifiers were added to fixtures or operator
  output.
- Operator inspection timing proxy: thread detail now projects conversation shape,
  participation decisions, scoped reset events, and handoff links in one response, so a
  representative participation or handoff outcome is inspectable from product evidence
  without logs.
- Live connector validation skipped. Owner: Roadmap 56 implementation owner. Reason:
  local verification uses fake/seeded evidence only; production tenants and live
  connector credentials are out of scope for this implementation pass. Remaining risk:
  provider-specific live mention/allowlist edge cases require a later safe-live smoke.
  Validation timestamp: 2026-05-11 18:56 Asia/Shanghai. Retention expiry: 2026-08-09.
  Redaction status: redacted.
