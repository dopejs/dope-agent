# Channels

Kura speaks IM natively: Discord, Telegram, Slack, and Matrix
connectors run inside the daemon, each as a `channel-*` plugin. Feishu/
Lark calendar and mail ride the integrations plane.

Every channel gets **one context per thread**: continuity, memory capture,
and window budgets are thread-scoped automatically (`sourceKind: channel`
uses the tighter 16k session budget).

## Enabling a channel

Channels are **off by default** everywhere. Enable in
`<data_dir>/config.json` (tokens via env, preferably):

```json
{
  "connectors": {
    "discord": {
      "enabled": true,
      "botTokenEnv": "DISCORD_BOT_TOKEN",
      "requireMention": true,
      "respondInDM": true,
      "allowedGuildIds": [],
      "allowedChannelIds": []
    },
    "telegram": {
      "enabled": true,
      "botTokenEnv": "TELEGRAM_BOT_TOKEN",
      "botUsername": "my_dope_bot",
      "allowedUserIds": []
    },
    "slack": {
      "enabled": true,
      "botTokenSecretRef": "secret://slack-bot-token",
      "workspaceId": "T…",
      "botUserId": "U…",
      "allowedChannelIds": []
    },
    "matrix": {
      "enabled": true,
      "homeserverUrl": "https://matrix.example.org",
      "botUserId": "@dope:example.org",
      "botAccessTokenEnv": "MATRIX_TOKEN",
      "selectedRoomIds": []
    }
  }
}
```

Allowlists (`allowedGuildIds`, `allowedUserIds`, `selectedRoomIds`, …)
are the safety rails: empty lists mean the connector's own default
posture, and mention-gating keeps group channels quiet unless addressed.

## Plugin gating

Even with `enabled: true` in config, a channel only runs if its plugin is
enabled — `plugins.json` `"disabled": ["channel-discord"]` wins, and no
network or credential is touched. This makes "temporarily unplug a
channel" a one-line, reversible operation.

## What flows back

Inbound messages are captured to memory (message + thread + dispatch
evidence links), replies flow through the same hookable chat pipeline as
every other turn, and delivery outcomes are inspectable per connector in
the operator shell.
