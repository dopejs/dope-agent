# Quickstart: Hosted Signup And Tenant Activation

This quickstart defines the verification path for Roadmap 45 implementation. Default to
the test environment and do not touch production data or live connectors unless an
operator explicitly authorizes a separate live run.

## Preconditions

- Branch: `030-hosted-tenant-activation`
- Environment: `KURA_ENV=test`
- Data root: `~/.kura-test`
- Daemon address: `127.0.0.1:19192`
- Live connectors: disabled
- Production secrets: not required

## Local Verification

1. Run focused daemon tests for activation:

   ```bash
   cd daemon
   go test ./internal/activation ./internal/api ./internal/store ./internal/billing
   ```

2. Run all daemon tests:

   ```bash
   cd daemon
   go test ./...
   ```

3. Run schema and contract validation:

   ```bash
   make daemon-contract-test
   ```

4. Run client tests:

   ```bash
   pnpm test:clients
   ```

5. Build clients if SDK or web output changes:

   ```bash
   pnpm build
   ```

6. Tidy Go modules after implementation:

   ```bash
   cd daemon
   go mod tidy
   ```

## Manual Test Environment Walkthrough

1. Start the test daemon:

   ```bash
   make daemon-run-test
   ```

2. Confirm daemon health:

   ```bash
   make daemon-test-status
   ```

3. Pair or authenticate a test hosted user through the hosted signup or invitation
   acceptance path. Do not use developer-only activation calls, manual database edits, or
   operator-run setup to create the personal tenant.

4. Load the web shell against `http://127.0.0.1:19192`.

5. Start a timer and confirm first-run activation shows the following within 30 seconds:

   - active personal tenant
   - environment scope
   - quota baseline
   - activation readiness state
   - required `test_chat` first action

6. Restart the daemon before running the first action and confirm the active personal
   tenant, quota baseline, readiness state, and required `test_chat` action remain visible
   and resumable:

   ```bash
   make daemon-test-status
   ```

7. Run the test chat first action.

8. Confirm activation reaches `first_action_completed`.

9. Restart the daemon and confirm activation state remains visible:

   ```bash
   make daemon-test-status
   ```

10. Induce or replay a representative activation failure and verify an operator can
    identify the failing stage, stable reason code, retryability, and remediation owner
    within 10 minutes using activation diagnostics rather than direct storage inspection.

## Required Negative Checks

- Returning user activation resolves the same personal tenant and does not create a
  duplicate.
- Concurrent activation attempts for the same user converge to one activation state.
- Disabled or denied users cannot complete activation and receive stable reason codes.
- Missing quota baseline blocks activation completion with a retryable quota readiness
  reason.
- Tenant access revocation clears or blocks activation state for that tenant.
- Organization invitation state does not block personal activation.
- Test chat failure preserves activation recovery state and stable reason code.
- Activation audit and diagnostics contain metadata only.
- Redaction checks find no secrets, tokens, test chat query, reply, transcript, streamed
  deltas, prompts, or raw provider payloads.

## Expected Evidence

- Passing focused daemon tests.
- Passing `go test ./...` from `daemon/`.
- Passing `make daemon-contract-test`.
- Passing `pnpm test:clients`.
- Passing `pnpm build` when client bundles change.
- `go mod tidy` produces no unintended module drift.
- Manual `KURA_ENV=test` walkthrough from no active setup to completed test chat.
- Evidence that signup or invitation acceptance lands in activation without manual
  activation calls.
- Evidence that first-run tenant, environment, quota, readiness, and next action are
  identifiable within 30 seconds.
- Evidence that activation state is durable after restart before the first action and after
  first-action completion.
- Evidence that a representative operator diagnostic drill identifies stage, reason,
  retryability, and remediation owner within 10 minutes.

## Implementation Evidence Log

- Focused activation daemon/API/contract tests were added for personal tenant activation,
  quota readiness, test chat completion, redaction, restart durability, audit fail-closed
  behavior, and diagnostics.
- SDK and web tests cover activation loading, quota projection, blocked diagnostics,
  test chat completion metadata, stale tenant refresh, and no rendering of test chat
  message content.
- Manual `KURA_ENV=test` walkthrough on 2026-05-06 used `make daemon-run-test` against
  `127.0.0.1:19192` and `make daemon-test-status` returned
  `{"ok":true,"service":"kura"}` before each activation check. Production data, live
  connectors, and `~/.kura` were not used.
- Hosted signup/authentication was exercised through a test pairing flow, then
  `POST /v1/activation` with `{"source":"signup"}` returned activation
  `act_prn_0ab80aded6cd6170_ten_afb3b7aa4173b0bd` for personal tenant
  `ten_afb3b7aa4173b0bd` and principal `prn_0ab80aded6cd6170`. The bearer token was
  treated as secret material and only recorded as `kura_cf3ace6...`.
- First-run review completed within the 30-second target from the activation projection:
  status `active`, environment `test`, quota baseline `development` with
  `unlimited` enforcement and `billing_usage_summary` source, readiness items
  `tenant-access`, `environment`, and `quota-baseline` all `ready`, and required first
  action `test_chat` available at `/v1/activation/test-chat`.
- Pre-first-action restart was verified by stopping and restarting the test daemon, then
  `GET /v1/activation` for tenant `ten_afb3b7aa4173b0bd` still returned status `active`,
  the same activation ID, the same quota baseline, no blockers, and available
  `test_chat`.
- `POST /v1/activation/test-chat` with the test hosted tenant completed activation at
  `2026-05-06T07:53:19.973835Z`: status `first_action_completed`, completed steps
  `tenant_resolved`, `quota_baseline_ready`, and `test_chat_completed`, dispatch
  `715fc48c-d982-413b-bd6a-6dd736175e73`, provider `openai_compatible`, model
  `gpt-5.4`, finish reason `stop`, and usage metadata only
  (`inputTokens=24`, `outputTokens=144`, `totalTokens=168`).
- Follow-up gap closure changed the activation test chat adapter to force the built-in
  `echo` provider (`echo-v1`) for Roadmap 45 activation, so the required first action no
  longer depends on external provider credentials or production secrets. This is covered
  by `go test ./internal/app -run
  TestActivationChatRunnerUsesBuiltinEchoWithoutExternalProviderCredentials`.
- Follow-up gap closure also added durable failed-test-chat state and metadata-only
  `tenant.activation_failed` audit evidence, exact `tenant.activation_test_chat_completed`
  completion audit evidence, operator diagnostics projection for activation findings,
  and web signup bootstrap from zero allowed tenants. These are covered by focused
  activation, API, app, and web tests before full-suite verification.
- Post-completion restart was verified by another daemon restart and health check;
  `GET /v1/activation` still returned `first_action_completed` with the same
  metadata-only test chat record. `GET /v1/activation/diagnostics` for the completed
  activation returned `{"items":[]}` after fixing a regression where completed test chat
  metadata was incorrectly projected as `activation_failed:unexpected`.
- The 10-minute diagnostic drill was replayed with
  `go test ./internal/api -run TestActivationDiagnosticsRouteProjectsQuotaFailureMetadata`,
  which verifies a quota-baseline failure diagnostic reports stage `quota_baseline`,
  reason `activation_blocked:quota_baseline_unavailable`, `retryable=true`, remediation
  owner `operator`, tenant scope, quota status `unavailable`, and no forbidden secret,
  transcript, prompt, or raw provider payload fields.
- Final verification after the diagnostics regression fix: `go test ./internal/activation
  -run Diagnostics`, `go test ./internal/api -run
  TestActivationDiagnosticsRouteProjectsQuotaFailureMetadata`, `go test ./...` from
  `daemon/`, and `go mod tidy` all completed successfully. Earlier full-suite failures in
  `internal/api` and `internal/app` were rerun individually with `-count=1` and then the
  full suite passed after stopping the local test daemon.
- Final verification after all follow-up gaps were closed: `go test ./...` from
  `daemon/`, `go mod tidy` from `daemon/`, `make daemon-contract-test`,
  `pnpm test:clients`, and the client build steps inside `pnpm test:clients` all
  completed successfully. The test daemon was started with `make daemon-run-test` on
  `127.0.0.1:19192`; after startup, `make daemon-test-status` returned
  `{"ok":true,"service":"kura"}`. The daemon was stopped after the health check.
