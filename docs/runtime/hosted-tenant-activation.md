# Hosted Tenant Activation Operations

Roadmap 45 adds a hosted first-run activation path for authenticated users. Activation
creates or resolves one personal tenant for the principal, projects readiness and quota
baseline, requires a metadata-only `test_chat` first action, and exposes diagnostics
without requiring direct storage inspection.

## Operator Boundaries

- Default validation is `DOPE_ENV=test` with `~/.dope-test` and `127.0.0.1:19192`.
- Activation must not require live connectors, production secrets, payment checkout, or
  organization setup.
- The required activation `test_chat` action uses the built-in `echo` provider so hosted
  activation remains available even when no external provider credentials are configured.
- Existing organization onboarding remains non-blocking for personal tenant activation.
- Test chat request and response content must not be retained in activation state, audit
  events, diagnostics, fixtures, or logs. Persisted evidence is limited to dispatch,
  provider, model, token usage, finish reason, status, and timestamps.

## Readiness And Diagnostics

- `POST /v1/activation` resolves the personal tenant and persists activation state.
- `GET /v1/activation` returns the current tenant-scoped activation projection.
- `POST /v1/activation/test-chat` completes the first action only when readiness passes.
- `GET /v1/activation/diagnostics` returns stable reason, stage, retryability,
  remediation owner, tenant scope, readiness item ids, and quota/test-chat metadata.

Stable operator-facing reason codes include principal denial, tenant access revocation,
quota baseline unavailable, test chat unavailable or failed, audit write failure,
persistence failure, and unexpected failure.

## Rollback

This feature is additive. To roll back behavior, remove activation routes from the daemon
API setup and stop constructing the activation service. Existing tenant, auth, billing,
chat, and onboarding routes remain compatible. The `activation_states` table can be left
in place during rollback because no existing runtime path depends on it.

## Verification

Before production promotion, run:

```bash
make daemon-contract-test
pnpm test:clients
pnpm build
cd daemon && go test ./... && go mod tidy
```

Then complete the `DOPE_ENV=test` walkthrough in
`specs/030-hosted-tenant-activation/quickstart.md`, including restart durability before
and after test chat plus a diagnostic drill for a quota-blocked activation.
