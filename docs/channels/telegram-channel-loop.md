# Telegram Channel Loop

Telegram is a hosted-ready channel connector for bot-based text messaging. It uses the
shared connector supervisor, hosted setup wizard, IM loop, durable message identity,
diagnostics, redaction, conformance, and connector-backed delivery boundaries.

## Hosted Setup

Telegram setup uses the submitted-secret setup path for a bot token. A setup can report
`ready` only when all of these gates pass:

- bot credential validation succeeds
- a Telegram bot account binding is recorded as redacted evidence
- at least one Telegram user, direct chat, or group allowment is explicitly configured
- connector conformance gates pass

Valid credentials without explicit allowment remain `action-required`. Invalid, missing,
or revoked credentials produce an auth diagnostic without exposing raw token material.
Provider and network failures produce `unavailable` with retry guidance. Cancelled setup
preserves redacted audit evidence and does not delete unrelated integration state.

Setup evidence uses the same 90-day default retention window as shared connector
diagnostics. Raw bot tokens, authorization headers, raw provider payloads, and
cross-tenant data must not appear in setup output, logs, fixtures, diagnostics, or
support evidence.

## Routing

Telegram ingress is text-only in phase 50.

- Direct messages are accepted only from explicitly allowed Telegram users or direct
  chats.
- Group messages are accepted only when the group is explicitly allowed and the message
  includes a bot mention or command.
- Allowed group messages without mention or command are ignored without a public reply.
- Unknown users, chats, and groups are blocked without a reply.
- Attachments, voice, payments, mini apps, broad media transfer, thinking visibility,
  and incremental visible updates are unsupported outcomes.

Durable duplicate suppression uses tenant, connector account, Telegram chat ID, and
Telegram message ID. Telegram update ID is retained only as redacted provider delivery
evidence for replay, reconnect, and support inspection.

## Replies And Delivery

Accepted foreground messages receive final-only Telegram replies. Telegram reply outcome
is recorded separately from assistant execution outcome. Telegram can also be used as a
connector-backed background delivery target after setup and allowment validate; background
delivery success, retry, suppression, and failure remain separate from foreground reply
truth.

## Diagnostics

Telegram setup, routing, reply, delivery, and smoke failures map into the shared
connector diagnostic reason codes:

- `auth_missing`
- `permission_missing`
- `rate_limited`
- `provider_unavailable`
- `network_failed`
- `unsupported_capability`
- `blocked_route`
- `duplicate_inbound`
- `reply_failed`
- `unknown_connector_failure`

Diagnostics older than 15 minutes are stale. Failed connector actions must publish current
diagnostic truth before remediation is shown. If evidence cannot be confidently redacted,
detailed evidence is suppressed and a redaction-failure marker is recorded.

## Live Smoke

Automated validation does not require real Telegram credentials. Safe live credentials
may be used only when they belong to a non-production bot, are scoped to a test tenant and
test chats/groups, are explicitly approved by an operator, and remain redacted in all
evidence. If safe credentials are unavailable, release validation records a structured
skip with owner, reason, date, remaining risk, and redaction status.

The fake safe-live path records pass evidence without contacting Telegram and keeps the
remaining live-provider risk visible.

Operators record smoke evidence through `POST /v1/live-validations/telegram-smoke` with
`live_validation.execute` permission. The endpoint accepts redacted `passed`, `failed`,
or `skipped` evidence for `fake`, `safe_live`, or `unavailable` credential modes and
persists the tenant-scoped evidence for later `GET /v1/live-validations/telegram-smoke`
inspection. It does not accept raw Telegram bot tokens or provider payloads; unsafe
evidence is suppressed before persistence.

## Rollback

Rollback disables Telegram setup, runtime ingress, and delivery-target eligibility while
retaining redacted setup, allowment, retained update, diagnostic, conformance, reply,
delivery, and smoke evidence until retention expiry. Existing Discord and shared
connector behavior must remain unaffected.
