# Discord Channel Loop

## Purpose

This document explains how to operate the first closed IM loop in DopeAgent.

The current supported IM channel is:

- `Discord Bot`

The current supported delivery mode is:

- `gateway`

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

## Discord Bot Requirements

Operator prerequisites:

- create a Discord application and bot
- obtain the bot token
- enable the `Message Content Intent` in the Discord developer portal if you expect content in guild and DM messages
- invite the bot to the target server with message read and send permissions

## Configuration

Configuration can come from `~/.dope/config.json` and `DOPE_*` environment variables.

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

## Behavior Rules

Direct messages:

- accepted only when `respondInDM=true`

Guild messages:

- rejected if `allowedGuildIds` is configured and the guild is not in the allowlist
- rejected if `allowedChannelIds` is configured and the channel is not in the allowlist
- rejected if `requireMention=true` and the bot is not mentioned

Mention normalization:

- when a guild message mentions the bot, the bot mention is stripped before the query is sent to the provider

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

- `connector.healthy`
- `connector.failed`
- `connector.ingress_accepted`
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

Current failure classes for Discord transport:

- `auth_error`
- `transport_error`

They currently appear in connector failure event payloads.

Examples:

- invalid or revoked bot token -> `auth_error`
- gateway or send failure that is not auth-related -> `transport_error`

When reply sending fails:

- the outbound delivery record is marked failed
- the step is marked failed
- a `connector.reply_failed` event is emitted

## Current Boundaries

This roadmap intentionally does not include:

- multiple channel implementations
- rich media replies
- attachments, reactions, voice, or typing indicators
- daemon-managed multi-turn chat history
- memory or context engineering for IM

## Verification Summary

This channel loop is currently verified through:

- config parsing tests
- API and contract tests
- Discord transport normalization tests
- Discord outbound request shaping tests
- Discord auth failure classification tests
- IM loop success and failure tests
- Discord runtime end-to-end tests using the real connector runtime boundary with a test transport
- full `go test ./...` on the daemon module
