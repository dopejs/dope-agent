# Quickstart: Hosted Credential And OAuth Setup Wizard

## Environment

Use the test daemon only:

```bash
make daemon-run-test
make daemon-test-status
```

Expected health response:

```json
{"ok":true,"service":"dope"}
```

Do not use `~/.dope`, production secrets, live provider credentials, or enterprise
identity credentials for default verification.

## Required Automated Verification

From `daemon/`:

```bash
go test ./internal/setupwizard ./internal/api ./internal/store ./internal/secrets ./internal/providers ./internal/integrations -count=1
go test ./...
go mod tidy
```

From repository root:

```bash
make daemon-contract-test
pnpm test:clients
pnpm build
```

## Manual Test Walkthrough

### 1. Activate tenant context

Use the Roadmap 45 hosted activation flow in the test environment. Confirm the active
tenant is available before opening setup.

Evidence to record:
- active tenant id
- environment scope
- activation status

### 2. Open setup targets

List or view setup targets for the active tenant.

Expected:
- OpenAI-compatible provider credential setup is visible as a submitted-secret proof
  target.
- Feishu/Lark OAuth setup is visible as an OAuth proof target.
- Unsupported targets are classified as unsupported or action-required and cannot become
  ready through partial setup.

### 3. OpenAI-compatible submitted-secret setup

Use a fake value:

```text
R46_FAKE_OPENAI_COMPATIBLE_KEY_DO_NOT_LEAK
```

Expected:
- The value can be submitted once.
- The response and UI show only redacted secret metadata.
- The setup session reaches ready, degraded, action-required, unavailable, or cancelled
  according to the configured fake provider/check result.
- The fake value is absent from API output, SDK output, web output, diagnostics, audit,
  events, fixtures, and logs.

### 4. Feishu/Lark OAuth fixture setup

Use the safe OAuth fixture path.

Expected:
- OAuth start creates an opaque setup state reference.
- Callback completion stores only redacted metadata.
- Healthy fixture reaches ready with diagnostic linkage.
- Missing scope and tenant approval fixtures produce action-required.
- Provider unavailable and network failed fixtures produce unavailable.
- Tenant mismatch does not leak inaccessible tenant details.

### 5. Retry, replace, cancel, and disable

For both proof targets, verify:
- retry repairs action-required or unavailable states when corrected evidence exists
- replace updates current credential/authorization state for future use
- cancel leaves unrelated integration and provider metadata inspectable
- disable blocks dependent credential-bearing use and preserves redacted metadata

### 6. Permission denial

Verify:
- missing `secrets.manage` blocks mutation
- missing `integrations.manage` blocks mutation
- missing `credentials.inspect` blocks redacted inspection
- denials do not reveal inaccessible tenant credential existence

### 7. Restart recovery

Restart the test daemon after:
- setup started before submission
- secret submitted before diagnostics complete
- OAuth callback completed before diagnostics complete
- setup reached ready/action-required/unavailable/cancelled/disabled

Expected:
- setup state is durable and resumable or terminal as appropriate
- no raw credential or OAuth material appears after restart

### 8. Operator diagnostic drill

Induce representative failures:
- credential missing
- scope missing
- tenant approval pending
- token expired or revoked
- provider unavailable
- network failed
- redaction failed closed

Expected:
- operator can identify setup stage, reason, retry safety, remediation owner, and tenant
  scope in 10 minutes or less
- detail route points to setup diagnostics
- no forbidden evidence appears

## Evidence Log

Record final implementation evidence here during `/speckit-implement`:

- Automated daemon tests: `go test ./...` from `daemon/` passed on 2026-05-06.
- Contract tests: `make daemon-contract-test` passed.
- SDK/web tests: `pnpm test:clients` and `pnpm build` passed.
- Test daemon health: `make daemon-run-test` started `127.0.0.1:19192`; `make daemon-test-status` returned `{"ok":true,"service":"dope"}` before and after restart.
- OpenAI-compatible fake secret redaction proof: live test-daemon drill submitted `R46_FAKE_OPENAI_COMPATIBLE_KEY_DO_NOT_LEAK`; setup response, diagnostics, and operator diagnostics did not contain the fake value.
- Feishu/Lark OAuth fixture proof: live test-daemon drill completed `integration.feishu_lark` OAuth fixture to `ready`.
- Retry/replace/cancel/disable proof: live test-daemon drill exercised OpenAI-compatible `retry`, `replace`, `cancel`, and `disable`; final state was `disabled`.
- Restart recovery proof: `daemon/internal/store` restart durability test reopens SQLite and recovers current setup session plus append-only attempts; test daemon health also passed after daemon restart.
- Operator diagnostic drill proof: live test-daemon readiness diagnostics returned one `credential_setup` finding for the disabled setup session with redacted evidence only.
