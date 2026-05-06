# Proof Targets And Rollback Contract

## OpenAI-Compatible Provider Credential Setup

Purpose: prove submitted-secret setup end to end.

Required behavior:
- Target id: `provider.openai_compatible`.
- Setup style: `submitted_secret`.
- Mutating setup requires `secrets.manage` and `integrations.manage`.
- The user submits one API-key-style secret value.
- The value is written or rotated through tenant secret behavior and is never returned.
- Setup links to provider profile/check or provider readiness evidence.
- Ready state requires redacted credential metadata and diagnostic/check success.
- Invalid or missing credential results in `action_required` with a stable reason code.
- Provider unavailable or network failure results in `unavailable`.
- Replace rotates or rewires the current credential for future use.
- Disable blocks dependent OpenAI-compatible credential-bearing use while preserving
  redacted status metadata.

Default verification uses fake credential values such as
`R46_FAKE_OPENAI_COMPATIBLE_KEY_DO_NOT_LEAK`.

## Feishu/Lark OAuth Setup

Purpose: prove OAuth setup end to end.

Required behavior:
- Target id: `integration.feishu_lark`.
- Setup style: `oauth`.
- Mutating setup requires `secrets.manage` and `integrations.manage`.
- OAuth start creates an opaque setup state reference.
- OAuth callback completion records only redacted completion metadata.
- Raw authorization code, access token, refresh token, callback payload, provider token,
  and authorization header are never retained in setup evidence.
- Completion links to Feishu/Lark diagnostic result.
- Tenant approval pending, missing scope, token missing, token expired, token revoked,
  tenant mismatch, provider unavailable, network failed, and rate limited map to stable
  setup reason codes.
- Retry and replace preserve historical redacted evidence and update current state.
- Disable blocks dependent Feishu/Lark credential-bearing use while preserving redacted
  status metadata.

Default verification uses safe OAuth fixtures. Real Feishu/Lark account smoke is not
required for Roadmap 46 implementation closure unless a later release-readiness gate
explicitly selects it.

## Dependent-Use Gating

| Setup state | Dependent use |
|-------------|---------------|
| `ready` | normal credential-bearing use permitted |
| `degraded` | only target-declared and diagnostic-confirmed limited safe capabilities permitted |
| `action_required` | blocked |
| `unavailable` | blocked |
| `cancelled` | blocked |
| `disabled` | blocked |

Dependent-use checks must be tenant-scoped. Cross-tenant targets with the same external
account key or secret ref do not satisfy each other.

OpenAI-compatible and Feishu/Lark degraded behavior must be explicit: if no limited-safe
capability is declared by both the target and diagnostic result, degraded state blocks
dependent credential-bearing use.

## Unsupported And Action-Required Targets

Targets outside the two v1 proof targets may appear in the wizard only if they expose:
- `unsupported` with a stable unsupported reason, or
- `action_required` with a stable reason and remediation owner.

They must not start a partial setup flow that can appear ready without full diagnostic
and redaction coverage.

## Migration

Setup-session persistence is additive. Existing tables and records for tenant secrets,
provider auth, integration readiness, diagnostics, provider checks, and audit remain
authoritative. Migrations must not copy raw secret values or OAuth token material into
setup tables.

## Rollback

Rollback steps:
1. Hide setup wizard entry points.
2. Disable setup-session mutation routes.
3. Keep setup-session read routes available for authorized support when possible.
4. Preserve existing tenant secrets, provider auth, integration readiness, diagnostics,
   and audit records.
5. Leave setup audit history readable and redacted.

Rollback must not delete tenant secrets, provider auth state, integration records,
diagnostic records, or audit evidence.

## Manual Evidence

Quickstart evidence must record:
- test daemon health check
- OpenAI-compatible submitted-secret setup using a fake secret value
- proof that the fake secret value is absent from output
- Feishu/Lark OAuth fixture setup
- action-required/unavailable diagnostic drill
- retry/replace/cancel/disable walkthrough
- restart recovery before and after setup completion
