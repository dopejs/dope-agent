# Channel Connector Conformance

Phase 48 defines the shared hosted channel connector contract. The contract applies to
fake connector conformance tests and the Discord regression baseline; Telegram, Slack,
and future connectors consume it in their provider-specific phases.

Hosted-ready connectors must prove these core invariants:

- tenant ownership and permission-gated inspection
- active-tenant account binding for ingress, foreground replies, diagnostics, and
  connector-backed delivery
- redacted APIs, events, logs, fixtures, support output, and conformance evidence
- durable inbound identity using tenant, connector account, channel or conversation, and
  provider message ID, or a documented equivalent durable rule
- stable routing decisions: accepted, ignored, blocked, duplicate, unsupported, or failed
- at least final-only foreground replies for accepted messages
- required diagnostic classifications with freshness, remediation, redaction, and
  retention evidence
- separation between foreground reply outcomes and background delivery outcomes

Provider-specific surfaces such as rooms, threads, rich media, thinking visibility, and
incremental updates may be supported, limited, or unsupported only when explicit. An
unsupported surface cannot weaken a core invariant.

## Discord Phase 49 Handoff

Discord now specializes this contract through hosted setup, diagnostic, route/reply, and
live-smoke evidence:

- hosted-ready requires a tenant-owned Discord bot credential and explicit selected
  guild/channel or DM behavior with redacted validation evidence
- local gateway configuration remains compatible, but projects as
  `degraded_needs_repair` for hosted readiness when it lacks explicit hosted evidence
- direct messages are supported when enabled and validated; group channels are supported
  only for explicit validated guild/channel destinations; thread replies and incremental
  updates are limited; voice, rooms, broad rich media, marketplace listing, and
  memory-based recall are unsupported for this phase
- Discord maps auth, permission, blocked route, duplicate inbound, reply failure,
  rate-limit, gateway/network, provider, unsupported, and unknown failures into the
  shared connector diagnostic vocabulary
- reply events keep assistant execution outcome separate from Discord foreground reply
  delivery outcome, and connector-backed background delivery remains a separate truth
- release review accepts either redacted safe-live smoke evidence or a structured skip
  that names the owner, reason, validation date, remaining risk, retention expiry, and
  redaction status

Default verification uses fake connectors and fake credentials in `~/.dope-test`.
Live connector credentials and production tenants are out of scope unless an operator
chooses a separate live validation path.

## Telegram Phase 50 Handoff

Telegram specializes this contract through bot-token hosted setup, explicit allowment,
route/reply diagnostics, delivery separation, and smoke evidence:

- hosted-ready requires a valid Telegram bot credential, redacted bot account binding,
  explicit user/chat/group allowment, and passing connector conformance gates
- valid credentials without explicit allowment remain `action-required` and cannot
  accept ingress, send replies, or become background-delivery eligible
- direct messages are supported only for explicitly allowed users or direct chats; group
  messages are supported only for explicitly allowed groups with bot mention or command
  gating
- text and commands are the only supported ingress payloads; attachments, voice,
  payments, mini apps, media transfer, thinking visibility, and incremental visible
  updates are unsupported for phase 50
- durable duplicate suppression uses tenant, connector account, Telegram chat ID, and
  Telegram message ID; Telegram update ID is retained as redacted delivery evidence
- foreground replies are final-only, and connector-backed background delivery remains
  separate from foreground reply and assistant execution truth
- release review accepts fake safe-live pass evidence plus either safe live smoke
  evidence or a structured skip that names owner, reason, date, remaining risk, and
  redaction status

## Provider-Specific Handoff

Future provider specs must reference this document for shared connector behavior and
limit their own acceptance criteria to provider mechanics. A provider spec may add
surface-specific tests for rooms, threads, media, cards, edits, stop controls, or rate
limits, but it must not redefine tenant ownership, active-tenant account binding,
durable inbound identity, dedupe, routing outcome meanings, redaction, diagnostics, or
foreground/background delivery separation.

Each provider handoff must include:

- a capability profile declaring every core invariant as pass or fail, and every
  provider-specific surface as supported, limited, or unsupported
- any equivalent durable identity rule, with the exact provider fields that replace the
  standard tenant/account/channel/provider-message identity
- routing fixtures for direct, group, mention-gated, blocked, unsupported, duplicate,
  and failed inputs that use the shared outcome vocabulary
- diagnostic mappings from provider failures into the shared connector reason codes
- explicit rollback notes for disabling the provider without deleting retained
  conformance or diagnostic evidence
