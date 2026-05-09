# Slack Channel Loop

Slack is a hosted-ready work-channel connector for tenant-owned Slack workspaces. It
uses the shared connector supervisor, hosted setup wizard, IM loop, durable message
identity, diagnostics, redaction, conformance, and connector-backed delivery boundaries.

## Hosted Setup

Slack setup uses hosted Slack app installation/OAuth only. A setup can report `ready`
only when all of these gates pass:

- OAuth installation validation succeeds
- exactly one Slack workspace is bound to the connector
- required scopes and app installation are present
- at least one selected channel or explicit DM user/user-group allowment is configured
- connector conformance gates pass

Starting the OAuth setup session returns a Slack `/oauth/v2/authorize` installation URL
with the tenant setup state and bot scopes needed for phase 51 routing, validation, and
reply behavior. The authorization URL does not contain client secrets or token material.

When a hosted OAuth callback includes a Slack authorization code, the daemon exchanges it
through Slack `oauth.v2.access`, stores the bot token in the tenant secret store under
the connector token secret ref, and keeps only redacted workspace, scope, route, and
diagnostic evidence in setup projections.

Valid OAuth installation without selected channel or explicit DM allowment remains
`action-required`. Missing OAuth grants, revoked installation, missing scopes, workspace
approval requirements, workspace mismatch, provider unavailability, and network failure
produce actionable terminal-state diagnostics without exposing raw tokens, signing
material, authorization payloads, or provider payloads.

Submitted raw bot tokens, signing secrets, and local-only credentials are unsupported
setup inputs for phase 51.

Retry returns the setup session to `in_progress` without deleting redacted evidence.
Replacement starts a new validation path for the same tenant-owned connector boundary.
Cancellation records `cancelled` with metadata-only evidence. Disablement records
`disabled_by_user` and makes Slack delivery ineligible without removing unrelated
integration state.

Setup validation must produce a terminal outcome within five minutes. Timeouts report
`unavailable` with `setup_timeout` so operators can distinguish slow hosted validation
from missing tenant action.

## Routing

Slack ingress is text-first in phase 51.

- Direct messages are accepted only from explicitly allowed Slack users or members of
  explicitly allowed Slack user groups.
- Channel messages are accepted only from selected channels and only when they mention
  the agent or use another explicitly supported invocation signal.
- Selected channel messages without a mention are ignored without a public reply.
- Unknown users, unselected channels, disabled connectors, and wrong workspaces are
  blocked without a reply.
- Marketplace publication, enterprise grid administration, memory-based team context,
  files, voice clips, huddles, canvases, workflow buttons, interactive blocks, rich
  media, thinking visibility, and incremental visible updates are unsupported outcomes.

Durable duplicate suppression uses tenant, connector, workspace, conversation, and
Slack message identity. Slack event identity is retained only as redacted provider
delivery evidence for replay, reconnect, delayed event delivery, and support inspection.

## Replies And Delivery

Accepted direct messages receive final-only Slack replies in the DM conversation.
Accepted channel mentions receive final-only Slack replies in a thread rooted at the
triggering channel message. Slack reply outcome is recorded separately from assistant
execution outcome.

Slack can also be used as a connector-backed background delivery target after setup and
destination policy validate. Background delivery success, retry, suppression, and failure
remain separate from foreground reply truth.

## Diagnostics

Slack setup, routing, reply, delivery, event-delivery, and smoke failures map into the
shared connector diagnostic reason codes:

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

Diagnostics distinguish missing OAuth grants, missing or revoked workspace installation,
workspace approval requirements, channel access or membership loss, rate limits,
event-delivery failures, provider unavailability, local network failure, duplicate input,
blocked routes, unsupported behavior, and unknown failures when evidence permits.

Diagnostics older than 15 minutes are stale. Failed connector actions must publish
current diagnostic truth before remediation is shown. If evidence cannot be confidently
redacted, detailed evidence is suppressed and a redaction-failure marker is recorded.

## Live Smoke

Automated validation does not require real Slack authorization. Safe live authorization
may be used only when it belongs to a non-production or explicitly approved test Slack
workspace, is scoped to a test tenant and selected test users/channels, is explicitly
approved by an operator, and remains redacted in all evidence. If safe authorization is
unavailable, release validation records a structured skip with owner, reason, date,
remaining risk, and redaction status.

## Rollback

Rollback disables Slack setup, runtime ingress, and delivery-target eligibility while
retaining redacted setup, workspace binding, route policy, retained event, diagnostic,
conformance, reply, delivery, and smoke evidence until retention expiry. Existing
Discord, Telegram, and shared connector behavior must remain unaffected.
