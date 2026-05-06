# Feishu/Lark Hosted OAuth Setup

The hosted setup wizard exposes `integration.feishu_lark` as the OAuth proof target.

Expected behavior:

- target kind: `integration`
- setup style: `oauth`
- mutation permissions: `secrets.manage` and `integrations.manage`
- inspection permission: `credentials.inspect`
- OAuth start creates an opaque state reference
- OAuth callback stores only redacted metadata
- ready state requires diagnostic linkage
- degraded use is limited to target-declared and diagnostic-confirmed capabilities

Negative OAuth outcomes map to non-failed setup states:

- denied, expired, replay, and tenant mismatch: `action_required`
- abandoned: `cancelled`
- provider error: `unavailable`

Raw OAuth authorization codes, access tokens, refresh tokens, callback payloads,
authorization headers, provider tokens, and client secrets must not be persisted or
returned by setup state, diagnostics, audit, SDK, web, logs, or fixtures.

Real Feishu/Lark account smoke is not required for Roadmap 46 closure. Use safe fixture
callbacks unless a later release-readiness gate explicitly selects live-account smoke.
