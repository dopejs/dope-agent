# Hosted Credential Setup Operations

Roadmap 46 adds tenant-scoped setup sessions for hosted credential and OAuth onboarding.
The default proof targets are:

- `provider.openai_compatible` using `submitted_secret`
- `integration.feishu_lark` using `oauth`
- `connector.matrix` using tenant-provided bot credentials and tenant-selected
  homeserver metadata

Mutation requires both `secrets.manage` and `integrations.manage`. Redacted inspection
requires `credentials.inspect`. Permission denials must not disclose whether a tenant,
secret, OAuth account, or setup session exists.

## Operator Surfaces

- `GET /v1/setup/targets` lists supported proof targets and current state.
- `GET /v1/setup/sessions` lists current tenant setup sessions.
- `POST /v1/setup/sessions` starts a setup session.
- `POST /v1/setup/sessions/{id}/submit-secret` accepts submitted-secret input.
- `POST /v1/setup/sessions/{id}/oauth/start` and `/oauth/callback` drive OAuth fixtures.
- `POST /v1/setup/sessions/{id}/retry|replace|cancel|disable` perform recovery.
- `GET /v1/setup/sessions/{id}/diagnostics` returns redacted diagnostic linkage.

Operator diagnostics include setup findings with source kind `credential_setup` when a
session is `action_required`, `unavailable`, `degraded`, `disabled`, or redaction has
failed closed.

## Redaction Rules

Setup state, diagnostics, audit records, contract fixtures, SDK output, web output, and
operator diagnostics are metadata-only. Never persist or render raw submitted secrets,
OAuth authorization codes, access tokens, refresh tokens, callback payloads, authorization
headers, provider client secrets, or derived credential material.

If evidence cannot be proven safe, set `redactionStatus=failed_closed`, block ready state,
and require operator remediation.

Matrix setup stores only redacted readiness evidence. Bot access tokens, authorization
headers, raw Matrix sync payloads, event bodies, and room content are never rendered in
setup state, diagnostics, fixtures, logs, or smoke evidence.

## Rollback

Rollback is additive and should not delete existing tenant secrets, provider records,
integration records, or audit history.

1. Hide the setup wizard entry points in web clients.
2. Disable setup mutation routes at the API edge.
3. Keep authorized read and diagnostic routes available when support needs redacted state.
4. Leave setup tables intact for recovery and audit review.
5. Use existing tenant secret/provider/integration paths as the source of truth.

For Matrix rollback, additionally disable Matrix ingress and delivery eligibility while
retaining Matrix setup, route policy, event evidence, smoke evidence, and diagnostics for
authorized support inspection.

## Verification

Use the test daemon only. Required checks:

- `cargo test -p kura-setupwizard -p kura-api -p kura-store -p kura-providers -p kura-integrations`
- `make daemon-contract-test`
- `pnpm test:clients`
- `pnpm build`
- `make daemon-run-test`
- `make daemon-test-status`
