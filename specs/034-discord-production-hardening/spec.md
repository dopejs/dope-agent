# Feature Specification: Discord Production Hardening

**Feature Branch**: `034-discord-production-hardening`  
**Created**: 2026-05-07  
**Status**: Draft  
**Input**: User description: "$speckit-specify 结合 docs/specs/034-discord-production-channel-hardening.md 完成 phase 49 的工作"

**Upstream authority**: `docs/specs/034-discord-production-channel-hardening.md` is the authoritative design document for this work (Roadmap 49). This specification translates that design into testable scenarios, requirements, and success criteria. Where the upstream document and this spec disagree, the upstream document wins and this spec must be updated.

**Related contract**: `docs/specs/033-channel-connector-conformance-contract.md` defines the shared hosted channel connector conformance contract that Discord must satisfy or explicitly declare unsupported for optional capabilities.

## Clarifications

### Session 2026-05-07

- Q: How should Discord setup behave when some selected destinations fail validation? → A: Save setup as degraded/needs repair when some selected destinations fail; block hosted-ready status until repaired.
- Q: How should hosted Discord setup behave when guild/channel allowlists are unset? → A: Save setup as degraded/needs repair and block hosted-ready status until explicit destinations validate.
- Q: What freshness and retention rules should Discord diagnostic and conformance evidence use? → A: Inherit phase 48 rules: diagnostics stale after 15 minutes; evidence retained 90 days by default.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Connect Discord With Tenant-Owned Credentials (Priority: P1)

As a user or tenant operator, I can connect a Discord bot using tenant-owned credentials, choose whether the bot responds in direct messages, mentions, guilds, and allowed channels, and immediately understand whether the bot can see and reply in the selected places.

**Why this priority**: Discord cannot be offered as a hosted production connector unless setup is tenant-owned, understandable, permission-aware, and safe when credentials are missing, invalid, or insufficient.

**Independent Test**: Can be tested by completing Discord setup with valid test credentials and by attempting setup with invalid, revoked, missing-permission, and blocked-channel credentials, then confirming the setup result is accurate and redacted.

**Acceptance Scenarios**:

1. **Given** a tenant has valid Discord bot credentials and selects direct message, mention, guild, and channel behavior, **When** setup validation runs, **Then** the tenant sees a connected state with the selected behavior recorded as tenant-scoped configuration.
2. **Given** a tenant provides an invalid, revoked, or missing Discord bot credential, **When** setup validation runs, **Then** the setup fails with a stable credential diagnostic and no token material is shown, logged, or retained in user-visible evidence.
3. **Given** a tenant selects guilds or channels where the bot cannot read or send messages, **When** setup validation runs, **Then** the setup is saved as degraded/needs repair, the tenant sees which selected destinations are not usable and what action is required to repair them, and Discord is not marked hosted-ready until the selected destinations validate.

---

### User Story 2 - Repair Discord Readiness Failures (Priority: P1)

As an operator, I can diagnose Discord token, permission, rate-limit, gateway, and provider availability failures using stable reason codes, timestamps, freshness, and redacted evidence so failed setups and degraded connectors are supportable.

**Why this priority**: Hosted Discord failures cross tenant credentials, provider permissions, gateway sessions, and rate limits. Operators need durable evidence that distinguishes causes without exposing secrets or another tenant's state.

**Independent Test**: Can be tested by replaying or simulating each required failure family and confirming authorized operators receive the correct redacted diagnostic while unauthorized users learn nothing about inaccessible tenants or connector accounts.

**Acceptance Scenarios**:

1. **Given** Discord authentication is missing, invalid, or revoked, **When** an authorized operator inspects connector health, **Then** the diagnostic identifies a credential failure, remediation owner, timestamp, freshness state, and safe evidence status.
2. **Given** Discord permissions, message content access, guild membership, channel access, rate limits, gateway connection, or provider availability blocks normal operation, **When** connector health is inspected, **Then** the diagnostic distinguishes the cause with a stable reason and repair guidance.
3. **Given** an operator is not authorized for a tenant, **When** they request Discord setup or diagnostic evidence for that tenant, **Then** access is denied without revealing tenant, guild, channel, account, credential, or message details.

---

### User Story 3 - Receive Predictable Discord Replies (Priority: P1)

As a Discord user, I receive assistant replies only where the tenant has allowed them, with direct message, mention, guild, channel, duplicate, and reply-failure behavior matching the shared channel conformance contract.

**Why this priority**: Public hosted use depends on Discord messages being accepted, ignored, blocked, deduplicated, and answered predictably rather than depending on connector-specific surprises.

**Independent Test**: Can be tested by sending representative direct messages, guild mentions, non-mentions, disallowed guild messages, disallowed channel messages, duplicate inbound events, and reply-failure cases through a Discord test connector.

**Acceptance Scenarios**:

1. **Given** direct messages are enabled for a tenant, **When** a Discord user sends the bot a direct message, **Then** the message is accepted and the assistant reply is delivered in that direct message conversation.
2. **Given** a guild or channel is not allowed, or a mention is required but absent, **When** a Discord message arrives there, **Then** the message is ignored or blocked with no assistant reply and with operator-visible reason evidence.
3. **Given** Discord or the gateway delivers the same inbound message more than once, **When** the connector handles the duplicate, **Then** at most one assistant reply is sent and the duplicate outcome is inspectable.
4. **Given** the assistant work completes but Discord reply delivery fails, **When** operators inspect the outcome, **Then** execution success and Discord reply failure are visible as separate facts.

---

### User Story 4 - Prove Hosted Production Readiness (Priority: P2)

As a release reviewer, I can see conformance, reconnect, rate-limit, repair, and smoke evidence proving Discord is ready to offer as a hosted production connector, or see a structured reason why live validation was safely skipped.

**Why this priority**: Phase 49 is a production hardening slice. Completion requires reviewable evidence, not only a working happy path.

**Independent Test**: Can be tested by running the Discord conformance evidence set, fake transport failure cases, reconnect and rate-limit scenarios, and a live hosted smoke path when safe credentials are available.

**Acceptance Scenarios**:

1. **Given** the Discord connector is evaluated against the shared channel conformance contract, **When** results are reviewed, **Then** every required core invariant passes and every optional unsupported or limited capability is declared explicitly.
2. **Given** Discord gateway reconnects, provider rate limits, duplicate inbound events, and reply failures occur, **When** the hardening evidence is reviewed, **Then** each case has durable, redacted, tenant-scoped diagnostic evidence.
3. **Given** safe live Discord credentials are unavailable, **When** release validation runs, **Then** the live smoke path records a structured skip with owner, reason, and remaining risk rather than silently passing.

### Edge Cases

- A token is missing, malformed, invalid, revoked, or belongs to a bot that cannot be used by the tenant.
- Discord message content access is disabled where content is needed for the selected behavior.
- The bot is not a member of the selected guild, has been removed from a guild, or loses required channel permissions after setup succeeds.
- Direct messages are disabled by tenant configuration or by Discord-side user or bot restrictions.
- Hosted setup has no explicit guild or channel destination selection; setup must save as degraded/needs repair and fail closed until explicit destinations validate.
- Guild or channel allowlists are empty, malformed, stale, or include destinations the bot cannot access.
- A message mentions the bot with provider-specific syntax that should not be sent to the assistant as user intent.
- Discord delivers duplicate gateway events or replays an event after reconnect or restart.
- Discord rate limits message sends or edits while a reply is in progress.
- Gateway connection drops, reconnects, or repeatedly flaps while messages are arriving.
- Visible reply progression is unavailable, unsafe, or rate-limited and must fall back to a safer reply mode.
- The assistant finishes successfully but the Discord reply cannot be sent or edited.
- Diagnostic evidence includes provider text, identifiers, or request context that is useful only if safely redacted.
- Redaction confidence is insufficient; detailed evidence must be suppressed while retaining a safe reason class.
- Cached Discord diagnostics are older than 15 minutes; inspection may show them only as stale, and new failed actions must produce current diagnostic truth.
- Discord diagnostic or conformance evidence reaches 90 days old; normal inspection must expire it unless an authorized longer retention policy applies.
- Existing local Discord configuration must continue working while hosted setup projects into the production connector model.
- Live Discord smoke validation cannot run because safe credentials are not available.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Discord MUST satisfy all required core invariants from the shared hosted channel connector conformance contract before it is considered hosted-production-ready.
- **FR-002**: Discord MUST explicitly declare every optional or provider-specific capability as supported, unsupported, or limited, including reply progression behavior, direct messages, guild messages, channel allowlists, mentions, and any unsupported rich media or voice surfaces.
- **FR-003**: Discord setup MUST use tenant-owned credentials and MUST prevent token values, credential-bearing payloads, authorization headers, and secret-derived material from appearing in setup results, diagnostics, events, logs, support output, or validation evidence.
- **FR-004**: Discord setup MUST validate whether the credential can authenticate, whether the bot account is usable, and whether selected guild, channel, direct message, and mention behavior can work as configured.
- **FR-004a**: If Discord credentials authenticate but some selected destinations or behaviors fail validation, the system MUST save the setup as degraded/needs repair and MUST block hosted-ready status until the selected destinations and behaviors validate.
- **FR-004b**: Hosted Discord setup with no explicit guild or channel destination selection MUST save as degraded/needs repair and MUST block hosted-ready status until explicit destinations validate.
- **FR-005**: Users MUST be able to configure Discord direct message handling, mention-required handling, allowed guilds, and allowed channels as tenant-scoped behavior.
- **FR-006**: Guild and channel allowlist state MUST be inspectable by authorized users using redacted tenant-scoped metadata, including enough destination identity to repair configuration without exposing inaccessible tenant data.
- **FR-007**: Discord inbound messages MUST resolve to the correct tenant-owned account binding and MUST fail closed when no authorized tenant binding exists.
- **FR-008**: Discord inbound routing MUST produce stable accepted, ignored, blocked, duplicate, unsupported, or failed outcomes for direct messages, guild messages, mention-required messages, and allowlisted or blocked destinations.
- **FR-009**: Discord mention or addressing artifacts MUST be removed from accepted user intent before the assistant receives the message.
- **FR-010**: Discord MUST deduplicate repeated inbound deliveries so provider retries, gateway reconnects, and restart replays produce at most one assistant reply per original user-visible Discord message.
- **FR-011**: Discord reply progression MUST declare the supported level, use visible progress only when safe, and degrade to a safer reply mode when message edits, typing signals, or rate limits make progression unsafe.
- **FR-012**: Discord foreground reply outcomes MUST remain separate from background delivery outcomes, even when both use the same Discord connector account.
- **FR-013**: Discord reply failures MUST distinguish assistant execution outcome from Discord delivery outcome so operators can see when the assistant completed but the channel reply failed.
- **FR-014**: Discord diagnostics MUST classify credential failures, missing permissions, missing message content access, blocked guilds or channels, direct-message restrictions, rate limits, provider unavailable, gateway disconnected, network failed, duplicate inbound, reply failed, unsupported capability, and unknown connector failure.
- **FR-015**: Every Discord diagnostic classification MUST include severity, remediation owner, timestamp, freshness, retry safety where relevant, and redacted evidence status for authorized inspection.
- **FR-015a**: Cached Discord diagnostics MAY be shown for inspection, but they MUST be marked stale after 15 minutes, and Discord actions that fail MUST produce current diagnostic truth before remediation is shown.
- **FR-015b**: Discord diagnostic evidence, conformance evidence, smoke evidence, and redaction-failure outcomes MUST use a 90-day default retention period unless an authorized longer retention policy applies.
- **FR-016**: Discord gateway reconnects, repeated disconnects, rate limits, duplicate inbound events, setup failures, and reply failures MUST produce durable diagnostic evidence that operators can inspect without provider secrets.
- **FR-017**: If Discord diagnostic or conformance evidence cannot be confidently redacted, detailed evidence MUST be suppressed and replaced with a safe generic classification plus a redaction-failure indication for authorized operators.
- **FR-018**: Existing local Discord test usage MUST remain compatible while existing configuration is migrated, projected, or interpreted into the hosted connector model.
- **FR-019**: Discord production hardening MUST include conformance evidence, fake transport evidence, and live hosted smoke evidence when safe credentials are available.
- **FR-020**: When safe live Discord credentials are unavailable, validation MUST record a structured skip with owner, reason, date, and remaining risk.
- **FR-021**: Phase 49 MUST NOT add Discord voice, broad rich media support, a Discord app marketplace listing, memory-based thread recall, or multi-channel abstractions beyond what is required for hosted channel conformance.
- **FR-022**: Phase 49 MUST NOT treat a happy-path Discord message loop as complete unless setup, repair, diagnostics, failure handling, conformance, and verification boundaries are also satisfied.

### Key Entities

- **Discord Connector**: The tenant-owned Discord channel integration that receives inbound Discord messages, sends foreground replies, and reports hosted readiness.
- **Discord Bot Credential**: A tenant-owned secret used to authenticate the Discord bot. Secret values are never exposed in user-visible output or diagnostic evidence.
- **Discord Account Binding**: The tenant-scoped relationship between a tenant and the Discord bot account, including redacted identity, lifecycle state, permissions, and selected behavior.
- **Discord Setup Configuration**: The tenant choices for direct messages, mention requirements, allowed guilds, allowed channels, connector display identity, and degraded/needs-repair state for partially invalid selected destinations.
- **Guild Or Channel Allowlist**: The tenant-scoped set of Discord destinations where the bot may respond, with redacted metadata suitable for inspection and repair.
- **Discord Inbound Message**: A normalized Discord user message with tenant binding, source conversation, sender context, provider message identity, routing decision, and redacted source metadata.
- **Discord Reply Progression Declaration**: The stated support level for visible thinking, incremental updates, final-only replies, or unsupported progression, including safe degradation behavior.
- **Discord Diagnostic State**: Redacted operator-visible evidence for setup, health, permission, rate-limit, gateway, duplicate, reply, and provider failure conditions.
- **Discord Conformance Evidence**: Reviewable results showing whether Discord satisfies required hosted channel connector invariants and which optional provider surfaces are supported, unsupported, or limited.
- **Discord Smoke Evidence**: Live hosted validation result when safe credentials exist, or a structured skip record when they do not.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: This phase adds hosted-readiness expectations for Discord setup, account binding, capability declaration, diagnostics, conformance evidence, documentation, and validation. Existing Discord local test configuration must continue to work and must be projected into the hosted connector model without breaking current test usage.
- **Migration / Rollback**: Existing Discord configuration may need to be interpreted as tenant-owned setup state for hosted readiness. Rollback should preserve the existing local Discord message loop and disable new hosted-readiness gating, repair surfaces, and conformance projections if they block operation.
- **Verification Strategy**: Required validation includes Discord conformance tests, fake transport tests for authentication, permissions, message content access, allowlists, rate limits, gateway disconnects, reconnects, duplicate inbound events, reply failures, redaction behavior, existing local configuration compatibility, and a live hosted smoke path with safe credentials or structured skip.
- **Observability Impact**: Operators must gain stable Discord setup, health, repair, rate-limit, gateway, duplicate, blocked-route, unsupported-capability, reply-failure, redaction, conformance, and smoke evidence without relying on raw provider logs or secret-bearing payloads.
- **Environment & Secrets**: Automated work must default to the repository test environment and fake credentials. Production tenants, live connectors, and real Discord credentials must not be touched unless an operator explicitly chooses a separate live validation path with safe credentials. Safe credentials mean non-production Discord credentials scoped to a test tenant, explicitly approved by an operator for validation, redacted in all evidence, and isolated from normal production tenants.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of required hosted channel connector core invariants pass for Discord before it is marked hosted-production-ready.
- **SC-002**: 100% of Discord optional or provider-specific surfaces under review produce an explicit supported, unsupported, or limited declaration with no silent skips.
- **SC-003**: 100% of credential, permission, message content access, guild access, channel access, direct-message restriction, rate-limit, gateway disconnect, provider unavailable, duplicate inbound, unsupported capability, and reply-failure fixture cases produce the expected stable diagnostic classification.
- **SC-003a**: 100% of cached Discord diagnostic inspection cases mark diagnostics older than 15 minutes as stale, and 100% of failed Discord action cases produce current diagnostic truth before remediation is shown.
- **SC-003b**: 100% of Discord diagnostic evidence, conformance evidence, smoke evidence, and redaction-failure outcomes expire from normal inspection after 90 days unless covered by an authorized longer retention policy.
- **SC-004**: 100% of redaction tests confirm Discord setup results, diagnostics, events, logs, support output, conformance evidence, and smoke evidence exclude raw tokens, authorization headers, credential-bearing payloads, and cross-tenant data.
- **SC-005**: 100% of duplicate inbound and reconnect replay fixture cases result in at most one assistant reply per original user-visible Discord message.
- **SC-006**: 100% of allowed direct message, allowed mention, allowed guild, and allowed channel fixture cases produce an assistant reply, and 100% of blocked or disallowed fixture cases produce no assistant reply.
- **SC-006a**: 100% of setup validations with authentic credentials and partially invalid or missing explicit selected destinations save as degraded/needs repair and do not mark Discord hosted-ready until those destinations validate.
- **SC-007**: 100% of reply progression degradation cases preserve a final user-visible reply when Discord progression is unsafe but final delivery remains possible.
- **SC-008**: 100% of reply failure cases distinguish assistant completion from Discord delivery failure in operator-visible evidence.
- **SC-009**: Existing local Discord test configuration remains usable after hosted-readiness projection in 100% of compatibility cases.
- **SC-010**: Live hosted smoke validation either completes successfully with safe credentials or records a structured skip with owner, reason, date, and remaining risk in 100% of release validation runs.

## Assumptions

- Roadmap 48 channel connector conformance, Roadmap 46 hosted credential and OAuth setup wizard, and existing integration health and permission diagnostics are available as prerequisite product truths.
- Discord remains the first production hardening proof channel, but this phase does not make Discord the only strategic channel.
- Tenant-owned bot credentials are the required hosted setup model for this phase.
- Current Discord reply progression support is expected to be declared and verified, but safe degradation is required when provider limits or failures make progression unsafe.
- Fake transport validation is sufficient for automated failure coverage; live Discord validation depends on explicit availability of safe credentials.
- Safe live Discord credentials are non-production, test-tenant scoped, explicitly operator-approved, redacted in all evidence, and used only for the validation path.
- Existing local Discord connector usage is operator-owned development behavior and must not be broken by hosted production hardening.
- Rich media, voice, marketplace listing, memory-based thread recall, and broad multi-channel abstractions are future work.
