# Contract: Provider Classification

## Goal

Map provider-specific failures into stable system-level reason codes, retry-safety
categories, and remediation owners. Feishu/Lark is the full proof domain for Roadmap 42.

## Reason Code Catalog

| Reason Code | Category | Retry Safety | Remediation Owner |
|-------------|----------|--------------|-------------------|
| `healthy` | healthy | no_action_needed | none_required |
| `app_authorization_missing` | authorization | blocked | operator |
| `bot_authorization_missing` | authorization | blocked | operator |
| `user_authorization_missing` | authorization | blocked | product_user |
| `tenant_approval_pending` | tenant_approval | blocked | tenant_admin |
| `scope_missing` | scope | blocked | tenant_admin |
| `token_missing` | token | blocked | product_user |
| `token_expired` | token | blocked | product_user |
| `token_revoked` | token | blocked | product_user |
| `refresh_credentials_missing` | token | blocked | operator |
| `token_refresh_failed` | token | blocked | operator |
| `tenant_mismatch` | tenant_mismatch | blocked | operator |
| `rate_limited` | quota | retryable | provider |
| `provider_unavailable` | provider | retryable | provider |
| `transient_provider_failure` | provider | retryable | provider |
| `network_failed` | network | retryable | operator |
| `ambiguous_downstream_commit` | retry_safety | unsafe_to_retry | operator |
| `unsafe_to_retry` | retry_safety | unsafe_to_retry | operator |
| `operator_action_needed` | retry_safety | operator_action_needed | operator |
| `limited_diagnostic` | unsupported | no_action_needed | operator |
| `unsupported_diagnostic` | unsupported | no_action_needed | operator |
| `redaction_failed_closed` | redaction | blocked | operator |
| `unknown_provider_error` | unknown | operator_action_needed | operator |

New reason codes require schema fixtures, SDK type updates, API contract tests, docs, and
at least one provider or fake-backend fixture.

## Retry Safety Values

- `no_action_needed`
- `retryable`
- `blocked`
- `unsafe_to_retry`
- `operator_action_needed`

Retry safety must be consistent with Roadmap 40 live-validation evidence when an
operation may have committed downstream.

## Feishu/Lark Classification Coverage

Feishu/Lark fixtures must cover:

- app or bot authorization missing,
- user authorization missing,
- tenant approval pending,
- provider scope missing,
- token missing,
- token expired,
- token revoked,
- refresh credentials missing,
- token refresh failed,
- tenant mismatch,
- rate limited,
- provider unavailable,
- transient provider failure,
- local network failure,
- ambiguous downstream commit,
- healthy readiness,
- redaction failure.

## Ambiguity Rules

- If provider evidence cannot distinguish missing scope from tenant approval, use an
  explicit ambiguous or unknown permission classification and choose the safest
  remediation owner.
- If provider evidence cannot prove whether a side-effecting operation committed, use
  `ambiguous_downstream_commit` or `unsafe_to_retry`.
- If diagnostic evidence cannot be confidently redacted, use `redaction_failed_closed`
  and suppress the detail.
- Raw provider text may be preserved only as redacted detail and never as the stable
  contract consumed by clients.

## Provider Evidence Fields

Provider classification records may include redacted:

- provider kind,
- operation class,
- provider status class,
- provider error code,
- request id when non-secret,
- retry-after class,
- evidence confidence,
- classifier version.

They must not include:

- OAuth tokens,
- refresh tokens,
- app secrets,
- authorization headers,
- raw provider request or response bodies containing credential material,
- local CLI auth material.

## Contract Tests

Required tests:

- each required Feishu/Lark fixture maps to the expected reason code,
- ambiguous provider evidence does not produce over-specific classifications,
- side-effecting ambiguous failures map to unsafe retry behavior,
- reason-code meanings are consistent across diagnostic results, smoke reports, audit
  events, user-facing remediation, SDK fixtures, and web fixtures,
- redaction-uncertain evidence fails closed.
