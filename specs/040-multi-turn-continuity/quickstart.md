# Quickstart: Non-Knowledge Multi-Turn Continuity

This walkthrough verifies Roadmap 55 in the local test environment. Default to
`~/.dope-test` and `127.0.0.1:19192`; do not use production tenants or live connector
credentials unless a later release-readiness gate explicitly approves a safe live smoke.

## Prerequisites

- Branch: `040-multi-turn-continuity`
- Active feature directory: `specs/040-multi-turn-continuity`
- Roadmap 54 thread/session lifecycle implemented and passing
- Test daemon environment only
- Fake or seeded chat/channel evidence
- At least one active thread, one reset thread, one archived/reopened thread, and one
  duplicate/replayed source-message fixture

## Seed Assumptions

Default verification uses fake or seeded connector/runtime evidence. The minimum manual
walkthrough seed should include:

- One active chat-origin thread with at least 14 prior turns in the current segment.
- One active channel-origin thread with current source linkage.
- One reset thread with pre-reset and post-reset turns.
- One archived thread with prior turns and runtime evidence.
- One reopened thread whose source remains eligible.
- One turn with a user-visible, safely redacted artifact excerpt.
- One artifact reference that is not user-visible or not safely redacted.
- One duplicate or replayed connector source message.
- One turn older than the active 30-day continuity window.
- One expired continuity preview beyond normal inspection retention.

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
- Data directory is `~/.dope-test`.
- No production connector credentials are required.

## 2. Run Focused Daemon Tests

From `daemon/`:

```sh
go test ./internal/threads ./internal/chat ./internal/api ./internal/store ./internal/events ./internal/connectors ./internal/im ./internal/contracts
```

Expected coverage:

- Chat requests without `threadId` remain single-turn.
- Chat requests with `threadId` persist request/response turns.
- Continuity includes no more than 12 eligible prior turns.
- Included turns are assembled oldest-to-newest by daemon acceptance sequence.
- Source timestamps are retained as evidence, not ordering authority.
- Reset excludes pre-reset turns and records `reset_boundary` evidence.
- Archived threads block continuity until reopened or replaced.
- Unsupported sources and missing thread identity do not infer continuity.
- Duplicate/replayed source messages do not create duplicate turns.
- Artifact content is included only as user-visible, safely redacted excerpts tied to
  included turns.
- Preview evidence records included and excluded items with stable reason codes.
- Restart recovery preserves turns, acceptance sequence, inclusion decisions, preview
  evidence, and reset boundaries for active, reset, archived, and reopened threads.
- Default-window continuity assembly meets p95 under 500 ms in the verification
  environment.
- Redaction failure suppresses unsafe detail and records safe metadata.
- Retention expiry removes turns/previews from normal inclusion and inspection.
- This phase creates no memory writes, memory recall, semantic retrieval, summaries,
  knowledge graph behavior, context packing, preference learning, or cross-thread
  personalization.

## 3. Validate Contracts

From repo root:

```sh
make daemon-contract-test
```

Expected:

- Chat request/response schemas accept additive thread/continuity fields and preserve
  existing single-turn fixtures.
- Stream started and terminal event schemas expose additive continuity metadata where
  applicable.
- Thread detail schema includes bounded preview summaries.
- Preview detail response schema validates included and excluded items.
- Event schemas for continuity turn and preview evidence validate.
- Contract fixtures cover chat continuity request/response payloads, preview detail
  payloads, and continuity turn/preview event payloads.
- Fixtures contain no raw provider payloads, disallowed message bodies, tokens,
  credentials, authorization grants, or cross-tenant identifiers.

## 4. Run SDK, Web, And TUI Tests

From repo root:

```sh
pnpm test:clients
pnpm build
```

Expected SDK coverage:

- `queryChat` and `streamChatQuery` accept optional `threadId`.
- Existing chat calls without `threadId` remain compatible.
- Chat responses expose `continuityPreviewId`, status, included count, and excluded
  count when continuity is evaluated.
- Preview detail fetch preserves tenant headers and denial behavior.

Expected web coverage:

- Thread-aware chat sends explicit thread identity only when selected.
- Thread detail shows recent continuity preview summaries.
- Preview detail shows included/excluded references and stable reason codes.
- Empty, blocked, disabled, redaction-failed, and permission-denied states are visible.
- UI copy labels the feature as recent-thread continuity, not memory.

Expected TUI/operator shell coverage:

- Chat command can pass an explicit thread ID.
- Thread detail output lists recent continuity preview IDs and statuses.
- Preview inspection command prints included/excluded references without unsafe content.
- Permission changes force reauthorization and do not display stale evidence.

## 5. Manual Test-Environment Walkthrough

1. Open the web product surface or TUI against the test daemon.
2. Select an active thread with at least 14 prior turns.
3. Ask a follow-up using the selected thread ID.
4. Confirm the response uses immediate prior context and exposes a preview ID.
5. Inspect the preview and confirm at most 12 prior turns are included.
6. Confirm older eligible turns are excluded with `over_limit`.
7. Confirm included items are ordered by daemon acceptance sequence.
8. Reset the thread using an account with `connectors.manage`.
9. Ask a follow-up that would require pre-reset context.
10. Confirm pre-reset turns are excluded with `reset_boundary`.
11. Inspect a turn with a user-visible artifact excerpt and confirm only the safe excerpt
    can be included.
12. Inspect an unsafe or non-user-visible artifact and confirm it is reference-only or
    excluded with a stable reason.
13. Simulate a duplicate or replayed channel message and confirm it does not create a
    duplicate continuity turn.
14. Archive the thread and attempt continuation; confirm continuity is blocked and
    inspectable.
15. Reopen when source/session rules allow it and confirm future continuity uses only the
    eligible current segment.
16. Revoke or simulate loss of `credentials.inspect`; retry preview inspection and
    confirm no preview content leaks.
17. Measure time for an operator to explain one response using product evidence; it must
    be 5 minutes or less.

## 6. Retention, Redaction, And Latency Checks

Seed continuity turns and previews older than the active 30-day window and older than
the normal 90-day inspection retention limit.

Expected:

- Turns older than the active window are excluded from continuity.
- Expired turns/previews disappear from normal inspection after retention expiry.
- Authorized longer tenant policy applies only when explicitly configured.
- Redaction validation finds zero raw provider payloads, disallowed message bodies,
  tokens, authorization grants, credentials, or cross-tenant identifiers.
- Performance validation reports p95 continuity assembly under 500 ms for the default
  window.

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
Record measured p95 continuity assembly latency and at least one operator preview
inspection timing before closing the phase.

## Verification Notes: 2026-05-11

Environment: local test workspace, fake/seeded daemon evidence only. Production state and
live connector credentials were not used.

Implementation measurements:

- p95 continuity default-window lookup latency:
  `TestContinuityDefaultWindowLookupP95` reported `241.875µs` for the 12-turn window.
- p95 continuity default-window assembly latency:
  `TestContinuityDefaultWindowAssemblyP95` reported `3.577792ms` for end-to-end
  chat continuity assembly and dispatch through the fake provider.
- Operator preview inspection timing: automated preview detail route and UI/TUI evidence
  inspection tests complete inside the focused client/API suites; representative
  operator evidence path is comfortably below the 5-minute target using seeded evidence.
- Live connector validation: structured skip. Owner: roadmap 55 implementer. Reason:
  this pass used fake/seeded local evidence only and no explicit safe live smoke was
  approved. Remaining risk: provider-specific live source identity quirks may still
  need smoke validation. Validation date: 2026-05-11. Retention expiry: 2026-08-09.
  Redaction status: redacted.

Command results:

- `make daemon-contract-test`: PASS (`internal/contracts`).
- `go test ./internal/store -run TestContinuityDefaultWindowLookupP95 -count=1 -v`:
  PASS, p95 `241.875µs`.
- `go test ./internal/chat -run TestContinuityDefaultWindowAssemblyP95 -count=1 -v`:
  PASS, p95 `3.577792ms`.
- `go test ./internal/im ./internal/connectors`: PASS.
- `go test ./internal/store ./internal/chat ./internal/threads ./internal/contracts`:
  PASS.
- `go test ./internal/api -run 'Test(ChatQueryRouteContinuityFieldsRemainAdditive|ResetContinuityRequiresManageAndExcludesPreResetTurns|ThreadContinuityPreviewDetailRoute|ChatQueryStreamContinuityParity)'`:
  PASS.
- `go test ./...` from `daemon/`: PASS after registering Roadmap 55 continuity tables
  in the schema inventory.
- `go mod tidy` from `daemon/`: PASS, no `go.mod` or `go.sum` diff.
- `pnpm test:clients`: PASS; SDK 41 tests, Web 37 tests, TUI 12 tests, Roadmap 7 smoke
  all passed.
- `pnpm build`: PASS.

Redaction and secret-output audit:

- Continuity schemas, fixtures, docs, Web, TUI, and SDK tests were scanned for raw
  prompt/memory/semantic/knowledge fields and secret-shaped material.
- Matches were limited to existing provider `api_key` enum strings outside the Roadmap
  55 continuity surfaces.
