# Plugins

**Everything outside the trust-boundary kernel is a plugin.** The kernel
(store, event bus, identity, auth, policy, secrets, audit) cannot be
disabled; everything else — LLM dispatch, chat, memory, context, session
strategies, channels, billing, scheduler, and 20+ more — assembles as a
named plugin with declared dependencies.

## The profile: `<data_dir>/plugins.json`

Boot-time input. A missing file means "everything enabled"; a malformed
file fails the boot loudly.

```json
{
  "disabled": ["channel-discord"],
  "entries": {
    "session-strategy": {
      "config": { "personalBudgetChars": 64000, "keepRecent": 4 }
    },
    "context": {
      "config": { "memoryBudgetChars": 4000, "retrievalBudgetChars": 2000 }
    },
    "self-improve": {
      "config": { "maxPerTargetPerDay": 3 }
    }
  }
}
```

- Disabling a plugin leaves its manager unwired (APIs answer "not
  configured") and **transitively disables dependents** — e.g. disabling
  `billing` disables `webhooks` (its quota gate is billing-backed and
  fail-closed; it can never run half-wired).
- Channel plugins gate their connector runtimes: profile wins over the
  config flag, and no network or credential is touched when disabled.

## Introspection

```bash
curl -s http://127.0.0.1:19192/v1/plugins           # assembly report
curl -s http://127.0.0.1:19192/v1/plugins/profile    # on-disk profile
```

`GET /v1/plugins` is the daemon's dump-config for the plugin plane: every
plugin in build order with enablement, disable reasons, provided seams,
requires edges, hook registrations, and profile warnings. `PUT
/v1/plugins/profile` validates and atomically replaces the profile
(`restartRequired: true` — the profile is boot-time input). The web
operator shell has a Plugins page with enable/disable toggles.

## Hook points (the waterfall)

Plugins attach behavior at named hook points; handlers run in
registration order, may **mutate the payload**, or **halt** (veto):

| Point | Payload | Powers |
|-------|---------|--------|
| `chat/turn-start` | `tenantId, threadId, query, sourceKind` | rewrite the query, veto the turn |
| `chat/pre-dispatch` | `+ agentProfileId, provider, model, messages[]` | rewrite context/provider/model, veto |
| `chat/turn-end` | `+ dispatchId, output, status, turn ids` | observe the settled turn |

`chat/pre-dispatch` runs **before the dispatch record is persisted**, so
the persisted messages are byte-identical to what the provider receives —
the *model-visible = logged* invariant. A veto returns HTTP 403 and is
recorded as a `chat.hook.vetoed` event.

## Default plugin lineup (selected)

| Plugin | Role |
|--------|------|
| `context` | memory bootstrap + query-time recall (first in the waterfall) |
| `session-strategy` | window shaping: personal 48k / IM-thread 16k budgets |
| `memory` | L0–L3 memory plane + capture hooks + 60s consolidation tick |
| `self-improve` | audited config-tuning proposals |
| `channel-*` | Discord / Telegram / Slack / Matrix runtimes |
