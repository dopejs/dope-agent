# Quickstart: Evaluation And Replay Harness

## Purpose

Validate roadmap 33 in `KURA_ENV=test` by loading curated replay fixtures, launching a
default non-live replay from the web operator shell, generating a plane-level comparison,
and confirming evaluation history survives daemon restart.

## Prerequisites

- Current branch: `018-evaluation-replay-harness`
- Active feature plan: `specs/018-evaluation-replay-harness/plan.md`
- Test environment only unless live validation is explicitly requested
- Local daemon dependencies installed

## Automated Verification

Run after implementation:

```bash
cd daemon
go test ./internal/evaluation ./internal/api ./internal/store ./internal/contracts ./internal/app
go mod tidy
cd ..
make daemon-contract-test
pnpm test:sdk
pnpm test:web
pnpm build:web
```

Expected coverage:

- curated candidate eligibility
- fixture manifest loading
- default non-live replay behavior
- replay readiness and blocked states
- replay attempt persistence
- plane-level comparison summaries
- event and API schema conformance
- SDK typing
- web operator shell replay flows

Observed implementation run:

- `cd daemon && GOCACHE=/tmp/kura-go-cache go test ./internal/evaluation ./internal/api ./internal/store ./internal/contracts ./internal/app`: pass
- `GOCACHE=/tmp/kura-go-cache make daemon-contract-test`: pass
- `pnpm build:sdk`: pass
- `pnpm test:sdk`: pass
- `pnpm test:web -- --runInBand`: pass
- `pnpm build:web`: pass
- `cd daemon && GOCACHE=/tmp/kura-go-cache go mod tidy`: pass with no `go.mod` or
  `go.sum` changes

## Manual Acceptance Flow

1. Start the test daemon:

   ```bash
   make daemon-run-test
   ```

2. Confirm daemon health:

   ```bash
   make daemon-test-status
   ```

3. Pair or reuse a local bearer token for the web shell.

4. Load or seed the engineer-managed replay fixtures for:

   - schedule-driven work
   - integration-backed work
   - computer-use work

5. Open the web operator shell.

6. Navigate to the Evaluation/Replay surface.

7. Confirm only curated candidates and engineer-managed fixtures appear.

8. Select one candidate and inspect readiness:

   - `fully_replayable`
   - `partially_replayable`
   - `blocked`
   - `unreplayable`

9. Launch a replay without enabling live validation.

10. Confirm the replay attempt records:

    - `mode: non_live`
    - source candidate linkage
    - environment scope
    - safety scope
    - `resultRunId` pointing at an `evaluation.replay` runtime run
    - `resultWorkflowId` pointing at the completed replay workflow envelope
    - blocked or evidence-only treatment for approval-gated and side-effecting steps

11. Generate a comparison for the replay attempt.

12. Confirm the comparison shows:

    - terminal status
    - runtime summary
    - policy summary
    - integration summary
    - delivery summary
    - evidence summary
    - drift findings grouped by plane where material differences exist

13. Restart the daemon.

14. Reopen the web shell and confirm replay candidates, attempts, comparisons, and drift
    findings remain inspectable.

## Expected Operator Outcomes

- Operators can launch a default non-live replay from the web shell.
- Operators can identify why a candidate is blocked or unreplayable.
- Operators can determine within 5 minutes of replay completion whether differences are
  primarily runtime, policy, integration, delivery, or evidence-summary drift.
- Evaluation history remains visible after restart.
- Test and live evidence never mix in the shell.

## Recorded Manual Evidence

`KURA_ENV=test` daemon walkthrough on `127.0.0.1:19192`:

- Pairing flow created a local bearer token without exposing token material in logs.
- `GET /v1/evaluation/replay-candidates` returned 3 curated fixture-backed candidates.
- `POST /v1/evaluation/replay-candidates` supports explicit curated-work registration
  for representative run or workflow envelopes; uncurated completed work is still not
  automatically eligible. Candidate registration rejects missing source provenance, and
  fixture candidates remain repo-managed rather than API-authored.
- `GET /v1/evaluation/fixtures` returned 3 fixtures covering schedule, integration, and
  computer-use classes, for a supported-fixture classification rate of 3/3.
- `POST /v1/evaluation/replay-candidates/{candidateId}/attempts` launched a default
  `non_live` replay from captured fixture evidence and returned `completed` immediately,
  under the 10-minute target, with `resultRunId` and `resultWorkflowId` linking the
  attempt to an `evaluation.replay` runtime run and completed replay workflow envelope.
- `evaluation.replay_completed` included result run/workflow IDs in both event scope and
  payload.
- `POST /v1/evaluation/replay-attempts/{attemptId}/compare` returned `matched`
  immediately, under the 5-minute drift-determination target.
- `live_validation` replay requests currently return a blocked attempt until a real live
  replay executor and approval flow exist.
- After daemon restart, `GET /v1/evaluation/replay-attempts`,
  `GET /v1/evaluation/comparisons`, and `GET /v1/evaluation/fixtures` still exposed 1
  manual attempt, 1 comparison, and 3 fixtures; the attempt remained `completed` and the
  comparison remained `matched`.

Route timing note: local candidate, fixture, replay, and comparison calls completed within
the synchronous local request path for the current fixture scale, satisfying the planned
<=1 s list-route and <=2 s replay-launch targets.

## Rollback

Rollback is additive:

- disable or remove `/v1/evaluation/*` routes
- hide the web shell Evaluation/Replay surface
- stop loading engineer-managed replay fixtures
- preserve existing run, workflow, schedule, integration, delivery, policy, and
  computer-use history
- retain already-recorded evaluation rows for audit if the deployed storage migration
  created them, unless an explicit migration rollback is provided
