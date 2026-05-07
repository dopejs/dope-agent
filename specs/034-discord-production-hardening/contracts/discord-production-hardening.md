# Contract: Discord Production Hardening

This contract is the planning handoff for Roadmap 49. It specializes the shared phase 48
channel connector conformance contract for the existing Discord connector.

## Hosted Readiness Gates

| Gate | Required Result | Failure State | Verification |
|------|-----------------|---------------|--------------|
| Credential authentication | Bot credential authenticates without exposing token material | `failed` with `auth_missing` or equivalent credential diagnostic | Fake transport auth tests; redaction tests |
| Bot account binding | Tenant-owned Discord bot account is bound and inspectable through redacted metadata | `degraded` or `failed` with repair evidence | Account binding tests; tenant isolation tests |
| Explicit destinations | Hosted setup has explicit selected guild/channel destinations | `degraded_needs_repair` | Setup validation tests |
| Destination validation | Every selected guild/channel/DM behavior validates | `degraded_needs_repair` until repaired | Destination validation tests |
| Conformance core invariants | Every phase 48 core invariant passes | Not hosted-ready | Discord conformance regression |
| Optional surfaces | Every optional Discord surface is supported, limited, or unsupported explicitly | Not hosted-ready if silent | Capability profile tests |
| Live hosted smoke | Passed with safe credentials, or structured skip exists | Release risk remains visible | Live smoke or skip tests |

## Setup State Contract

| Input Condition | Persisted State | Hosted-Ready? | Required Evidence |
|-----------------|-----------------|---------------|-------------------|
| Missing, malformed, invalid, or revoked credential | `failed` | No | Redacted credential diagnostic |
| Credential authenticates; no explicit hosted guild/channel destinations | `degraded_needs_repair` | No | Missing destination repair evidence |
| Credential authenticates; some selected destinations fail validation | `degraded_needs_repair` | No | Per-destination validation results |
| Credential authenticates; all selected destinations and behaviors validate | `hosted_ready` | Yes, if conformance gates also pass | Setup validation timestamp and account binding summary |
| Existing local config projects into hosted model but lacks hosted evidence | Local-compatible plus hosted degraded/needs-repair if required | No until hosted gates pass | Compatibility evidence |

## Destination Validation Contract

Discord destination validation must produce redacted tenant-scoped results for:

- direct-message behavior
- mention-required behavior
- allowed guild IDs
- allowed channel IDs
- message content access where content is needed
- bot membership and send/read permissions
- stale or inaccessible destinations

Destination evidence must include enough safe metadata for repair without exposing token
material, authorization headers, raw provider payloads, cross-tenant state, or inaccessible
message content.

## Diagnostic Mapping

| Discord Condition | Shared Reason Code | Lifecycle State | Remediation Owner |
|-------------------|--------------------|-----------------|-------------------|
| Missing, invalid, revoked, or unusable credential | `auth_missing` | `failed` | `product_user` |
| Missing guild/channel send/read permission | `permission_missing` | `permission_blocked` | `tenant_admin` |
| Message Content Intent unavailable where required | `permission_missing` | `permission_blocked` | `tenant_admin` |
| Selected guild/channel blocked or not selected | `blocked_route` | `degraded` | `tenant_admin` |
| Direct messages disabled or restricted | `blocked_route` | `degraded` | `tenant_admin` |
| Discord rate limit on sends, edits, or gateway operation | `rate_limited` | `rate_limited` | `provider` |
| Discord provider unavailable | `provider_unavailable` | `degraded` | `provider` |
| Gateway disconnected, reconnect failed, or network unavailable | `network_failed` | `degraded` | `operator` |
| Duplicate inbound gateway event or replay | `duplicate_inbound` | `degraded` | `none_required` |
| Reply send or edit failed after assistant work | `reply_failed` | `failed` | `operator` |
| Unsupported optional surface such as voice or broad rich media | `unsupported_capability` | `unsupported_capability` | `none_required` |
| Unclassified Discord connector failure | `unknown_connector_failure` | `failed` | `operator` |

## Freshness, Retention, And Redaction

- Cached Discord diagnostics may be shown, but diagnostics older than 15 minutes must be
  marked `stale`.
- Discord actions that fail must produce current diagnostic truth before remediation is
  shown.
- Discord diagnostic evidence, conformance evidence, smoke evidence, and redaction-failure
  outcomes expire from normal inspection after 90 days unless an authorized longer
  retention policy applies.
- If evidence cannot be confidently redacted, detailed evidence is suppressed and a safe
  generic classification plus redaction-failure marker is recorded for authorized
  operators.

## Safe Live Credential Rule

Safe live Discord credentials must satisfy all of these conditions before live smoke can
run:

- credentials belong to a non-production Discord application or bot
- credentials are scoped to a test tenant and test destinations only
- an operator explicitly approves use for the validation path
- evidence is redacted and retained under the same 90-day default retention rule
- production tenants, production guilds, and normal live connector state are not touched

## Routing And Reply Contract

| Scenario | Required Outcome |
|----------|------------------|
| Direct message and `respondInDM=true` | Accepted and replied in the direct message conversation |
| Direct message and `respondInDM=false` | Blocked or ignored with no assistant reply |
| Guild/channel outside allowlist | Blocked with no assistant reply and repair evidence |
| Guild message requires mention and mention is absent | Ignored with no assistant reply |
| Guild message mentions bot | Mention artifact stripped before assistant handling |
| Duplicate provider message after retry/reconnect/restart | At most one assistant reply; duplicate outcome inspectable |
| Reply progression unsafe or rate-limited | Degrade to safer mode, preferably final-only if final delivery remains possible |
| Assistant completes but Discord reply fails | Assistant execution and Discord delivery outcomes remain separate |

## Capability Declaration

Discord must declare:

- Core invariants: all pass before hosted-ready.
- Direct messages: supported only when enabled and validated.
- Group/guild channels: supported only for explicit validated destinations.
- Mention gating: supported when configured and tested.
- Rooms, voice, broad rich media, marketplace listing, memory-based thread recall:
  unsupported for phase 49.
- Thread reply: supported, limited, or unsupported based on validated runtime behavior.
- Thinking and incremental visible updates: supported or limited only with rate-limit and
  degradation evidence.
- Final-only foreground replies: required minimum for accepted messages.
- Connector-backed background delivery: must remain separate from foreground reply truth.

## API, Schema, Event, And Documentation Impact

Any exposed public shape must update schemas and fixtures with the implementation:

- `GET /v1/config` includes `connectors.discord.hostedReadiness`, which preserves local
  compatibility while projecting local-only credentials as `degraded_needs_repair`
  until tenant-scoped destination validation evidence promotes the connector to
  `hosted_ready`
- `GET /v1/connectors/{connectorId}/discord-setup` returns
  `DiscordHostedSetupResource` with tenant-scoped `DiscordDestinationValidationResource`
  entries for authorized operators
- `discord_hosted_setups`, `discord_destination_validations`, and
  `discord_smoke_evidence` retain redacted setup, destination, and smoke records by
  tenant and connector
- connector account binding summaries for Discord
- connector capability profile and conformance results
- connector diagnostic state with Discord reason mapping
- destination validation and repair evidence if exposed
- live smoke or structured skip evidence
- `connector.discord_setup_validated`, `connector.diagnostic_state_changed`,
  `connector.route_outcome_recorded`, `connector.reply_failed`, and foreground/background
  delivery events expose only redacted evidence
- `connector.reply_failed` carries separate `assistantExecutionOutcome` and
  `discordDeliveryOutcome` fields
- `docs/channels/discord-channel-loop.md` and related operator docs

## Verification Gates

Implementation is incomplete until these cases are covered:

- valid setup becomes hosted-ready only after selected destinations validate
- invalid credential fails without leaking token material
- missing explicit hosted destinations save degraded/needs repair
- partially invalid destinations save degraded/needs repair
- direct message, mention, allowed guild, allowed channel, blocked guild/channel, and
  mention-required routing behavior
- duplicate inbound suppression after replay/reconnect
- gateway disconnect/reconnect evidence
- rate-limit evidence and reply progression degradation
- reply failure evidence separated from assistant execution
- foreground reply outcome remains separate from connector-backed background delivery
  outcome
- diagnostic freshness after 15 minutes and current truth on failed actions
- 90-day retention expiry
- redaction suppression on unreliable evidence
- existing local Discord config compatibility
- live hosted smoke pass or structured skip
- schema/contract fixtures and docs updated with public shape changes
