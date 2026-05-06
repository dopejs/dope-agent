# OpenAI-Compatible Hosted Setup

The hosted setup wizard exposes `provider.openai_compatible` as the submitted-secret
proof target.

Expected behavior:

- target kind: `provider`
- setup style: `submitted_secret`
- mutation permissions: `secrets.manage` and `integrations.manage`
- inspection permission: `credentials.inspect`
- ready state requires redacted secret metadata and diagnostic/check linkage
- degraded use is limited to target-declared and diagnostic-confirmed capabilities
- action-required, unavailable, cancelled, and disabled states block credential-bearing use

Default verification uses:

```text
R46_FAKE_OPENAI_COMPATIBLE_KEY_DO_NOT_LEAK
```

That value is accepted only by the inbound mutation body. It must never appear in setup
responses, SDK objects, web rendering, diagnostics, audit records, events, logs, or
contract fixtures.

Recovery:

- retry moves recoverable states back to `in_progress`
- replace starts a new current credential path while preserving historical attempts
- cancel records `user_cancelled`
- disable records `disabled_by_user` and blocks dependent use without deleting unrelated
  provider or tenant-secret metadata
