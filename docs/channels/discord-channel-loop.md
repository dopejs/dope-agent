# Discord Channel Loop

## Purpose

This document explains how to operate the first closed IM loop in DopeAgent.

The current supported IM channel is:

- `Discord Bot`

The current supported delivery mode is:

- `gateway`

Current reply progression level:

- `thinking + streaming`

Hosted production readiness is gated separately from local compatibility. A local gateway
configuration may continue to run, but hosted-ready requires a submitted tenant-owned bot
credential plus explicit selected guild/channel or DM behavior that has redacted
validation evidence.

## What The Daemon Does

For an accepted Discord inbound message, the daemon now performs one closed single-turn loop:

1. accept and normalize the Discord message
2. resolve or create session truth
3. create a run and a step
4. invoke the configured provider through the chat service
5. send the assistant reply back to Discord
6. persist inbound and outbound delivery records
7. persist runtime and connector events

This path is intentionally single-turn:

- no memory assembly
- no context compaction
- no multi-turn daemon-owned conversation state

The current Discord reply progression is:

1. emit a Discord typing indicator immediately
2. keep refreshing typing while generation is still underway
3. stream provider output inside daemon-owned progression logic
4. send the first visible reply message when enough output exists
5. edit the same Discord message with throttled updates
6. finalize that same message when generation completes

## Discord Bot Requirements

Operator prerequisites:

- create a Discord application and bot
- obtain the bot token
- enable the `Message Content Intent` in the Discord developer portal if you expect content in guild and DM messages
- invite the bot to the target server with message read and send permissions

## Configuration

Configuration comes from the active environment config file and `DOPE_*` environment variables.

- development/test default: `~/.dope-test/config.json`
- production explicit env: `~/.dope/config.json`

The Discord connector config shape is:

```json
{
  "connectors": {
    "discord": {
      "enabled": true,
      "connectorId": "discord-main",
      "displayName": "Discord Main",
      "deliveryMode": "gateway",
      "botTokenEnv": "DOPE_DISCORD_BOT_TOKEN",
      "requireMention": true,
      "respondInDM": true,
      "allowedGuildIds": ["123456789012345678"],
      "allowedChannelIds": ["234567890123456789"]
    }
  }
}
```

Relevant environment variables:

- `DOPE_CONNECTORS_DISCORD_ENABLED`
- `DOPE_CONNECTORS_DISCORD_CONNECTOR_ID`
- `DOPE_CONNECTORS_DISCORD_DISPLAY_NAME`
- `DOPE_CONNECTORS_DISCORD_DELIVERY_MODE`
- `DOPE_CONNECTORS_DISCORD_BOT_TOKEN`
- `DOPE_CONNECTORS_DISCORD_BOT_TOKEN_ENV`
- `DOPE_CONNECTORS_DISCORD_REQUIRE_MENTION`
- `DOPE_CONNECTORS_DISCORD_RESPOND_IN_DM`
- `DOPE_CONNECTORS_DISCORD_ALLOWED_GUILD_IDS`
- `DOPE_CONNECTORS_DISCORD_ALLOWED_CHANNEL_IDS`

Current `deliveryMode` support:

- only `gateway`

Reply progression support:

- `thinking`: supported
- `incremental output`: supported only when the hosted conformance profile has explicit
  rate-limit and degradation evidence; otherwise the connector degrades toward final-only
  replies

Hosted readiness projection is exposed in `GET /v1/config` under
`connectors.discord.hostedReadiness`. Detailed tenant-scoped setup evidence is exposed
through `GET /v1/connectors/{connectorId}/discord-setup` for authorized operators. These
responses are redacted: token material, authorization headers, raw provider payloads, and
inaccessible message content must not appear in API output, events, logs, fixtures, or
support evidence.

Hosted readiness states:

- `hosted_ready`: valid credential plus explicit selected destinations and passing
  destination validation
- `degraded_needs_repair`: valid credential but missing explicit destinations, or at
  least one selected destination is invalid
- `failed`: missing, invalid, revoked, or unusable credential
- `disabled`: connector disabled

## Behavior Rules

Direct messages:

- accepted only when `respondInDM=true`

Guild messages:

- rejected if `allowedGuildIds` is configured and the guild is not in the allowlist
- rejected if `allowedChannelIds` is configured and the channel is not in the allowlist
- rejected if `requireMention=true` and the bot is not mentioned

Mention normalization:

- when a guild message mentions the bot, the bot mention is stripped before the query is sent to the provider

Reply progression rules:

- Discord uses typing indicator for `thinking`
- Discord uses message edit for incremental output
- the daemon sends an initial reply and then edits the same message
- edits are throttled rather than emitted for every token
- if progression is not available, the daemon falls back to final-only reply behavior
- assistant execution success and Discord reply delivery success are separate outcomes;
  a failed Discord send/edit after assistant completion emits `connector.reply_failed`
  with `assistantExecutionOutcome` and `discordDeliveryOutcome`

## Observability

Key daemon surfaces:

- `GET /v1/connectors`
- `GET /v1/connectors/{connectorId}`
- `GET /v1/sessions`
- `GET /v1/runs`
- `GET /v1/runs/{runId}/events`
- `GET /v1/events/stream`
- `GET /v1/config`

Important event names:

- `connector.discord_setup_validated`
- `connector.diagnostic_state_changed`
- `connector.route_outcome_recorded`
- `connector.healthy`
- `connector.failed`
- `connector.ingress_accepted`
- `connector.thinking_started`
- `connector.thinking_failed`
- `connector.reply_stream_started`
- `connector.reply_stream_updated`
- `connector.reply_sent`
- `connector.reply_failed`
- `session.created`
- `session.routed`
- `run.created`
- `step.created`
- `step.status_changed`
- `run.status_changed`
- `llm.dispatch.requested`
- `llm.dispatch.completed`
- `llm.dispatch.failed`

## Failure Visibility

Discord failures are mapped into stable connector diagnostic reason codes:

- `auth_missing`: missing, invalid, revoked, or unusable bot credential
- `permission_missing`: missing guild/channel read/send permission or Message Content
  Intent when message content is required
- `blocked_route`: selected guild/channel/DM behavior blocks the message
- `rate_limited`: Discord rate limit on send, edit, or gateway operation
- `provider_unavailable`: Discord provider outage or 5xx-class failure
- `network_failed`: gateway disconnect, reconnect failure, or network failure
- `duplicate_inbound`: duplicate gateway event or replay suppressed
- `reply_failed`: assistant work completed but Discord reply delivery failed
- `unsupported_capability`: optional Discord surface is explicitly unsupported
- `unknown_connector_failure`: unclassified connector failure

Diagnostics older than 15 minutes are stale. Redacted setup, diagnostic, conformance, and
smoke evidence uses the 90-day default retention window. If evidence cannot be safely
redacted, details are suppressed and a safe generic classification remains inspectable.

When reply sending fails:

- the outbound delivery record is marked failed
- the step is marked failed
- a `connector.reply_failed` event is emitted with separate assistant execution and
  Discord delivery outcome fields

When thinking fails:

- the daemon emits `connector.thinking_failed`
- the daemon still continues toward a final reply if the rest of the path remains healthy

## Live Smoke Policy

Automated release validation does not require real Discord credentials. When safe live
credentials are available, they must belong to a non-production Discord application, be
scoped to a test tenant and test destinations, and be explicitly approved by an operator.
When safe credentials are unavailable or unsafe, Discord records a structured skip with
owner, reason, validation date, remaining risk, retention expiry, and redaction status.
The skip is evidence of residual release risk; it is not a silent pass.

## Current Boundaries

This roadmap intentionally does not include:

- multiple channel implementations
- rich media replies
- attachments, reactions, voice, or broad rich media
- daemon-managed multi-turn chat history
- memory or context engineering for IM

## Verification Summary

This channel loop is currently verified through:

- config parsing tests
- API and contract tests
- Discord transport normalization tests
- Discord typing indicator tests
- Discord outbound request shaping tests
- Discord reply edit tests
- Discord auth failure classification tests
- IM loop success and failure tests
- IM loop thinking and streaming progression tests
- Discord runtime end-to-end tests using the real connector runtime boundary with a test transport
- full `cargo test --workspace` on the daemon workspace
