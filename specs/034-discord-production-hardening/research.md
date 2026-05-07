# Phase 0 Research: Discord Production Hardening

## Decision: Harden Discord in place as the phase 48 conformance consumer

**Rationale**: Discord already has the gateway runtime, fake transport tests, IM loop
integration, route outcomes, capability profile scaffolding, and shared connector
diagnostic/conformance vocabulary. Hardening the existing provider keeps the change
minimal and lets phase 49 prove the phase 48 contract against the first real channel.

**Alternatives considered**:

- Build a separate hosted Discord connector service: rejected because it duplicates
  existing runtime, routing, store, and event boundaries and increases rollback risk.
- Rework all connector setup around a new abstraction first: rejected because phase 49 is
  provider-specific hardening, not a multi-channel abstraction roadmap.

## Decision: Persist setup progress but gate hosted readiness for partial failures

**Rationale**: Tenants should not lose setup work when a selected guild/channel is missing
permissions or unavailable, but the connector must not be advertised as hosted-ready until
every selected destination and behavior validates. This creates a clear degraded/needs
repair state with operator-visible evidence.

**Alternatives considered**:

- Reject setup saves unless every selected destination validates: rejected because it
  makes repair harder and loses useful diagnostic state.
- Mark the connector healthy for working destinations: rejected because it can mislead
  operators about public hosted readiness and leave selected destinations broken.

## Decision: Missing explicit hosted destinations are degraded/needs repair

**Rationale**: Hosted Discord should fail closed. Treating empty allowlists as "all
guilds/channels where the bot is installed" is too broad for tenant-owned hosted setup and
weakens operator review. Local legacy config can still project compatibly, but hosted-ready
state requires explicit destination validation.

**Alternatives considered**:

- Empty allowlists mean all accessible destinations: rejected for hosted use because it can
  expand blast radius silently.
- Block saving setup entirely: rejected because a repairable persisted setup state is more
  useful for diagnostics and guided completion.

## Decision: Reuse phase 48 diagnostic freshness and retention rules

**Rationale**: Discord should not create provider-specific policy for stale diagnostics or
evidence retention. Cached diagnostics become stale after 15 minutes, failed actions must
produce current diagnostic truth before remediation is shown, and diagnostic/conformance/
smoke/redaction-failure evidence expires from normal inspection after 90 days by default.

**Alternatives considered**:

- Define Discord-specific freshness/retention in implementation: rejected because it would
  diverge from the shared channel conformance contract.
- Require current diagnostics only with no cache display: rejected because cached
  diagnostic history is useful when clearly marked stale.

## Decision: Extend diagnostics from coarse transport classes to stable support reasons

**Rationale**: Current Discord failure classes are too coarse for hosted support. Phase 49
needs stable reasons for credential failure, permission/message-content problems, blocked
guild/channel, direct-message restriction, rate limit, provider unavailable, gateway
disconnect, network failure, duplicate inbound, reply failure, unsupported capability, and
unknown connector failure.

**Alternatives considered**:

- Keep `auth_error` and `transport_error` only: rejected because operators cannot repair
  setups or distinguish provider, permission, rate-limit, and network causes.
- Expose raw Discord errors: rejected because raw provider text can carry secrets or
  tenant-specific data and is not stable enough for support workflows.

## Decision: Live hosted smoke is optional but must produce explicit evidence

**Rationale**: Automated acceptance must not require real Discord credentials. When safe
credentials exist, a live hosted/test smoke can prove end-to-end hosted readiness. When
they do not, the result must be a structured skip with owner, reason, date, and remaining
risk so release review does not mistake missing live validation for success.

**Alternatives considered**:

- Require live Discord smoke for all acceptance: rejected because it would force real
  secrets into normal local verification.
- Omit live smoke entirely: rejected because public hosted readiness benefits from
  explicit live validation or a reviewable skip.

## Decision: Keep reply progression declaration conservative and degradation-tested

**Rationale**: Discord supports typing and message edits today, but provider rate limits
and edit failures can make incremental progression unsafe. The hosted connector must
declare support precisely and degrade to final-only behavior when progression is unsafe
while keeping reply failure evidence separate from assistant execution.

**Alternatives considered**:

- Treat current streaming behavior as unconditionally supported: rejected because rate
  limits and edit failures can make visible updates unsafe.
- Disable all progression for Discord: rejected because typing/final and edit progression
  are useful when proven safe and already exist in the runtime.
