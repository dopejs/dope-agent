# Contract: Telegram Channel Connector

This contract is the planning handoff for Roadmap 50. It specializes the shared phase 48
channel connector conformance contract for the new Telegram connector.

## Hosted Readiness Gates

| Gate | Required Result | Failure State | Verification |
|------|-----------------|---------------|--------------|
| Credential authentication | Bot credential validates without exposing token material | `action-required` or `degraded` with `auth_missing` diagnostic | Fake transport auth tests; redaction tests |
| Bot account binding | Tenant-owned Telegram bot account is bound and inspectable through redacted metadata | `degraded` or `action-required` with repair evidence | Account binding tests; tenant isolation tests |
| Explicit allowment | At least one Telegram user/chat/group is explicitly allowed before ingress can create runs | `action-required` until configured | Allowment validation tests |
| Direct-message sender authorization | Direct messages create runs only for explicitly allowed users/chats | `blocked` route outcome | DM allowment tests |
| Group routing gate | Group messages create runs only when the group is explicitly allowed and the message includes a bot mention or command | `ignored` or `blocked` route outcome | Group gate tests |
| Text-only scope | Text and command messages are the only supported ingress payloads | `unsupported` route outcome | Unsupported surface tests |
| Durable inbound identity | Dedupe key is tenant, connector account, Telegram chat ID, and Telegram message ID; update ID is retained as evidence | `duplicate` route outcome on replay | Dedupe/reconnect tests |
| Conformance core invariants | Every phase 48 core invariant passes | Not ready | Telegram conformance regression |
| Optional surfaces | Every optional Telegram surface is supported, limited, or unsupported explicitly | Not ready if silent | Capability profile tests |
| Live hosted smoke | Passed with safe credentials, or structured skip exists | Release risk remains visible | Live smoke or skip tests |

## Setup State Contract

| Input Condition | Terminal State | Ready? | Required Evidence |
|-----------------|----------------|--------|-------------------|
| Valid bot credential, valid account binding, and explicit valid allowment | `ready` | Yes, if conformance gates also pass | Validation timestamp and account binding summary |
| Valid credential but no explicit allowed user/chat/group | `action-required` | No | Missing allowment remediation evidence |
| Valid credential but selected allowment is blocked, stale, or missing permission | `degraded` or `action-required` | No | Per-allowment validation result |
| Telegram provider unavailable or network validation fails | `unavailable` | No | Provider/network diagnostic and retry guidance |
| Missing, malformed, invalid, or revoked credential | `action-required` | No | Redacted credential diagnostic |
| User cancels setup | `cancelled` | No | Redacted audit evidence, no unrelated state deleted |

Terminal states must reuse the hosted setup wizard vocabulary: `ready`, `degraded`,
`unavailable`, `cancelled`, and `action-required`.

## Allowment Contract

Telegram allowment must produce redacted tenant-scoped results for:

- allowed Telegram users
- allowed direct chats
- allowed groups
- group mention-or-command gate status
- blocked or missing allowment
- stale, inaccessible, or invalid provider scopes
- delivery-target eligibility

Allowment evidence must include enough safe metadata for repair without exposing bot token
material, authorization headers, raw provider payloads, inaccessible message content, or
cross-tenant state.

## Routing Contract

| Scenario | Required Outcome |
|----------|------------------|
| Direct text from explicitly allowed user/chat | Accepted and eligible for one agent run |
| Direct text from unknown sender/chat | Blocked with no assistant reply |
| Group behavior disabled | Ignored or blocked with no assistant reply |
| Group not explicitly allowed | Blocked with no assistant reply |
| Allowed group without bot mention or command | Ignored with no public reply |
| Allowed group with bot mention | Accepted; mention artifact normalized before assistant handling |
| Allowed group with command | Accepted; command artifact normalized before assistant handling |
| Attachment, voice, payment, mini app, media transfer, or unsupported surface | Unsupported with no agent run |
| Duplicate Telegram chat/message identity after retry/reconnect/restart | Duplicate with at most one assistant reply |
| Missing tenant/account/chat/message identity | Failed closed unless an explicit equivalent identity rule is documented and tested |

## Durable Identity Contract

Telegram inbound dedupe identity is:

- `tenantId`
- `connectorAccountId`
- `telegramChatId`
- `telegramMessageId`

Telegram update identity must be retained as redacted provider delivery evidence for
diagnostics, replay, reconnect, and support inspection. It must not replace chat/message
identity as the canonical duplicate suppression key.

## Reply And Delivery Contract

| Scenario | Required Outcome |
|----------|------------------|
| Accepted foreground message completes assistant work | Send at least one final Telegram reply |
| Assistant completes but Telegram reply fails | Assistant execution and Telegram reply outcomes remain separate |
| Telegram rate limits foreground reply | Diagnostic reason recorded; retry/degradation evidence retained |
| Telegram selected as background delivery target | Delivery success, retry, suppression, or failure is recorded separately from foreground replies |
| Background delivery fails after work succeeds | Work result remains separate from delivery failure truth |

Phase 50 reply progression declaration:

- Final-only foreground replies: required.
- Thinking visibility: unsupported unless explicitly recut.
- Incremental visible updates: unsupported unless explicitly recut.
- Connector-backed background delivery: supported only through explicit delivery-target
  eligibility and separate delivery outcome evidence.

## Diagnostic Mapping

| Telegram Condition | Shared Reason Code | Lifecycle/Terminal State | Remediation Owner |
|--------------------|--------------------|--------------------------|-------------------|
| Missing, invalid, revoked, or unusable bot token | `auth_missing` | `action-required` | `product_user` |
| Bot cannot send or inspect required chat/group behavior | `permission_missing` | `degraded` or `action-required` | `tenant_admin` |
| Unknown sender/chat or group not explicitly allowed | `blocked_route` | `degraded` | `tenant_admin` |
| Allowed group lacks mention or command | `blocked_route` | `degraded` | `none_required` |
| Telegram rate limit on reply, delivery, or update handling | `rate_limited` | `degraded` | `provider` |
| Telegram provider unavailable | `provider_unavailable` | `unavailable` | `provider` |
| Network unavailable, reconnect failed, or retry exhausted | `network_failed` | `unavailable` or `degraded` | `operator` |
| Duplicate inbound chat/message after replay | `duplicate_inbound` | `degraded` | `none_required` |
| Reply send failed after assistant work | `reply_failed` | `degraded` | `operator` |
| Attachment, voice, payment, mini app, media transfer, thinking, or incremental update | `unsupported_capability` | `unsupported_capability` | `none_required` |
| Unclassified Telegram connector failure | `unknown_connector_failure` | `degraded` | `operator` |

## Freshness, Retention, And Redaction

- Cached Telegram diagnostics may be shown, but diagnostics older than 15 minutes must be
  marked `stale`.
- Telegram actions that fail must produce current diagnostic truth before remediation is
  shown.
- Telegram setup evidence, allowment evidence, diagnostic evidence, conformance evidence,
  smoke evidence, retained update evidence, and redaction-failure outcomes expire from
  normal inspection after 90 days unless an authorized longer retention policy applies.
- If evidence cannot be confidently redacted, detailed evidence is suppressed and a safe
  generic classification plus redaction-failure marker is recorded for authorized
  operators.

## Safe Live Credential Rule

Safe live Telegram credentials must satisfy all of these conditions before live smoke can
run:

- credentials belong to a non-production Telegram bot
- credentials are scoped to a test tenant and test users/chats/groups only
- an operator explicitly approves use for the validation path
- evidence is redacted and retained under the same 90-day default retention rule
- production tenants, production chats/groups, and normal live connector state are not
  touched

## Capability Declaration

Telegram must declare:

- Core invariants: all pass before ready.
- Direct messages: supported only for explicitly allowed users/chats.
- Group messages: supported only for explicitly allowed groups with bot mention or
  command gating.
- Mention gating: supported for groups.
- Command gating: supported for groups.
- Final-only foreground replies: required minimum for accepted messages.
- Connector-backed background delivery: supported only when destination eligibility is
  explicit and delivery truth remains separate.
- Attachments, voice, payments, mini apps, media transfer, memory behavior, thinking
  visibility, and incremental visible updates: unsupported for phase 50.

## API, Schema, Event, And Documentation Impact

Any exposed public shape must update schemas and fixtures with the implementation:

- `GET /v1/config` or connector list projections include Telegram connector health,
  capability, account binding, and diagnostic summaries additively when exposed.
- `GET /v1/connectors/{connectorId}/telegram-setup` returns a
  `TelegramHostedSetupResource` with setup terminal state, account binding summary,
  allowment summaries, diagnostic linkage, retention expiry, and redaction status for
  authorized operators.
- Telegram setup persistence stores terminal state, account binding, allowment,
  diagnostic linkage, and redacted evidence by tenant and connector.
- Telegram inbound message persistence stores canonical chat/message dedupe identity and
  retained update identity as redacted evidence.
- Connector capability profile and conformance results include Telegram provider
  surfaces.
- Connector diagnostic state maps Telegram failures to shared reason codes.
- Delivery target resources represent Telegram eligibility only after setup and
  allowment validate.
- `connector.telegram_setup_validated`, `connector.diagnostic_state_changed`,
  `connector.route_outcome_recorded`, `connector.inbound_duplicate_detected`,
  `connector.foreground_reply_failed`, and connector-backed delivery events expose only
  redacted evidence if implemented as provider-specific events.
- Foreground reply events carry separate `assistantExecutionOutcome` and
  `telegramReplyOutcome` fields when public reply outcome detail is exposed.
- `docs/channels/telegram-channel-loop.md`, `docs/channels/channel-connector-conformance.md`,
  and related operator docs describe setup, routing, unsupported surfaces, diagnostics,
  live smoke, and rollback behavior.

Implemented Phase 50 surface names:

- API: `GET /v1/connectors/{connectorId}/telegram-setup`,
  `GET /v1/connectors/{connectorId}/telegram-smoke`,
  `GET /v1/live-validations/telegram-smoke`, and
  `GET /v1/live-validations/telegram-conformance`.
- Config: `connectors.telegram` in `config.json`, `KURA_CONNECTORS_TELEGRAM_*`
  environment overrides, and `/v1/config.connectors.telegram`.
- Schemas: `telegram-hosted-setup-resource`, `telegram-allowment-resource`,
  `telegram-smoke-evidence-resource`, and
  `connector-telegram-setup-validated.event`.
- Storage: `telegram_hosted_setups`, `telegram_allowments`,
  `telegram_smoke_evidence`, and `telegram_update_evidence`.
- SDK: `getTelegramSetup`, `getTelegramSmokeEvidence`, and
  `getTelegramConformanceEvidence`.

## Verification Gates

Implementation is incomplete until these cases are covered:

- valid setup becomes ready only after bot credential and explicit allowment validate
- invalid/revoked/malformed credential fails without leaking token material
- provider unavailable setup returns `unavailable`
- missing explicit user/chat/group allowment returns `action-required`
- cancelled setup preserves audit evidence without deleting unrelated state
- direct message from allowed sender/chat is accepted
- direct message from unknown sender/chat is blocked
- group disabled behavior is ignored or blocked
- allowed group without bot mention or command is ignored
- allowed group with bot mention or command is accepted
- unsupported attachments, voice, payments, mini apps, media transfer, thinking, and
  incremental updates produce unsupported outcomes
- duplicate inbound suppression by chat/message identity after retry/reconnect/restart
- Telegram update identity is retained as redacted delivery evidence
- final-only foreground reply success
- foreground reply failure separated from assistant execution
- connector-backed background delivery success, retry, suppression, and failure separated
  from foreground reply truth
- diagnostic freshness after 15 minutes and current truth on failed actions
- 90-day retention expiry
- redaction suppression on unreliable evidence
- tenant isolation and permission denial
- live hosted smoke pass or structured skip
- schema/contract fixtures and docs updated with public shape changes
