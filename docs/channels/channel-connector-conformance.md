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
- daemon-owned thread source linkage for accepted, duplicate, blocked, and replayed
  inbound messages
- stable routing decisions: accepted, ignored, blocked, duplicate, unsupported, or failed
- at least final-only foreground replies for accepted messages
- required diagnostic classifications with freshness, remediation, redaction, and
  retention evidence
- separation between foreground reply outcomes and background delivery outcomes
- source-to-runtime trace evidence that remains metadata-only and explicitly separate
  from memory recall or context packing

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

## Slack Phase 51 Handoff

Slack specializes this contract through hosted Slack OAuth, workspace binding, explicit
route policy, thread-rooted channel replies, delivery separation, diagnostics, and smoke
evidence:

- hosted-ready requires a valid Slack OAuth installation, exactly one tenant-owned
  workspace binding, required scopes, and at least one selected channel or explicit DM
  user/user-group allowment
- submitted bot tokens, signing secrets, local-only credentials, marketplace
  publication, enterprise grid administration, memory-based team context, files, voice
  clips, huddles, canvases, workflow buttons, interactive blocks, rich media, thinking
  visibility, and incremental visible updates are unsupported for phase 51
- direct messages are supported only for explicitly allowed Slack users or user-group
  members; selected channel messages are accepted only when they mention the agent or
  use an explicitly supported invocation signal
- durable duplicate suppression uses tenant, connector, Slack workspace, conversation,
  and Slack message ID; Slack event ID is retained only as redacted delivery evidence
- foreground replies are final-only; channel replies are rooted at the triggering Slack
  message thread; connector-backed background delivery remains separate from assistant
  execution and foreground reply truth
- Slack maps auth, permission, blocked route, duplicate inbound, reply failure,
  rate-limit, provider, network/event-delivery, unsupported, and unknown failures into
  the shared connector diagnostic vocabulary
- release review accepts redacted fake or safe-live smoke evidence, or a structured skip
  that names owner, reason, validation date, remaining risk, retention expiry, and
  redaction status

## Matrix Phase 52 Handoff

Matrix specializes this contract through tenant-provided bot setup, homeserver binding,
explicit direct or selected-room route policy, final-only replies, delivery separation,
diagnostics, and smoke or structured skip evidence:

- phase 52 chooses Matrix and explicitly rejects WhatsApp fallback, hosted homeserver
  operation, Matrix account provisioning, bridge automation, and E2EE key/session
  management
- hosted-ready requires a valid tenant-provided Matrix bot credential, reachable and
  supported homeserver, passing conformance profile, and at least one allowed direct user
  or selected room route
- direct messages are supported only for explicitly allowed Matrix users; room messages
  are supported only in selected rooms and only when a bot mention or configured command
  invokes the agent
- unencrypted text is the only supported message payload; encrypted rooms,
  undecryptable events, media, calls, voice, reactions, bridge metadata, thinking
  visibility, and incremental visible updates are unsupported for phase 52
- durable duplicate suppression uses tenant, connector, homeserver, conversation, and
  Matrix event id; sync batch and transaction ids are retained as redacted delivery
  evidence only
- foreground replies are final-only Matrix replies, and connector-backed background
  delivery remains separate from assistant execution and foreground reply truth
- release review accepts safe-live Matrix smoke evidence, or a structured skip that names
  owner, reason, validation date, remaining risk, retention expiry, and redaction status

## Provider-Specific Handoff

Future provider specs must reference this document for shared connector behavior and
limit their own acceptance criteria to provider mechanics. A provider spec may add
surface-specific tests for rooms, threads, media, cards, edits, stop controls, or rate
limits, but it must not redefine tenant ownership, active-tenant account binding,
durable inbound identity, dedupe, routing outcome meanings, redaction, diagnostics, or
foreground/background delivery separation.

Roadmap 54 adds one more shared rule: connector-local conversation state is never the
source of truth for continuation after restart. Providers must attach accepted inbound
messages to the daemon-owned current thread for `(tenant, connector, source account,
source conversation)`, preserve duplicate evidence on replay, and respect archived
thread blocking unless the thread is explicitly reopened.

Roadmap 55 adds bounded continuity on top of that daemon-owned thread. Providers must
pass accepted messages through the daemon thread identity so only current-session
eligible turns can be assembled. Duplicate, replayed, unsupported, missing-source, and
archived-thread inputs must not infer continuity; they may only record redacted
operator-visible exclusion evidence. Continuity previews must expose inclusion limits,
reset-boundary exclusions, source identity decisions, and redaction status without raw
prompt or memory fields.

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
