# Real-Account Smoke Policy

Fake-backend coverage is mandatory for every supported integration domain.
Real-account smoke is opt-in and uses operator-provided safe credentials only.

For each supported domain:

- run real-account smoke when safe credentials are available
- record an explicit skip reason when credentials are unavailable, expired,
  revoked, or unsafe
- keep fake-backend coverage passing regardless of real-account availability
- never log, report, back up, restore, or expose raw credential material

Readiness may pass with recorded skips only when all fake-backend and
operational evidence passes and the skipped real-account domains have explicit
reasons.

## Calendar (Feishu/Lark, Roadmap 60)

The calendar domain has a real Feishu/Lark provider implemented as an adapter on the
external integration adapter plane (Roadmap 59). Its real-account smoke is built by
`opsreadiness.CalendarRealAccountSmoke`:

- When safe operator-provided Feishu/Lark credentials are available and enabled, the
  create/update/cancel live-validation rows are exercised and the status reports `pass`.
- Otherwise an explicit structured skip is recorded (default reason: "safe Feishu/Lark
  calendar credentials unavailable in this environment"); overall readiness can still pass
  because fake-backend and operational evidence pass.
- The raw provider message is never forwarded into diagnostics or smoke output; only the
  stable, redacted failure-class token and reason code are recorded.

The real provider runs only when `DOPE_INTEGRATION_ADAPTER` names the adapter binary and
`DOPE_ADAPTER_PROVIDER=feishu_lark` is set; default development/CI uses the fake backend.

## Mail (Feishu/Lark, Roadmap 63)

The mail domain has a real Feishu/Lark provider on the same adapter plane
(`DOPE_ADAPTER_DOMAIN=mail`). Its smoke is `opsreadiness.MailRealAccountSmoke`: pass with safe
operator credentials + exercised send/reply/forward rows, else an explicit structured skip
(default reason: "safe Feishu/Lark mail credentials unavailable in this environment"). No
message content beyond redacted evidence and no credential material is exposed.
