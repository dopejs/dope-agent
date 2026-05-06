# Research: Hosted Signup And Tenant Activation

## Decision: Add a daemon-owned activation state machine

**Rationale**: Activation must be durable, tenant-scoped, restart-safe, diagnosable, and
consistent across SDK and web shell clients. A daemon-owned activation service can reuse
existing identity, billing, audit, and chat packages while keeping product behavior out of
client-only code and operator runbooks.

**Alternatives considered**:

- Web-only activation flow. Rejected because it cannot provide durable restart behavior,
  authoritative audit, or concurrency-safe personal tenant resolution.
- Extending only `/v1/operator/onboarding`. Rejected because onboarding is a broad shell
  projection and should consume activation state rather than own persistence and
  idempotency.
- Embedding activation logic in existing tenant handlers. Rejected because it would mix
  product first-run state with lower-level tenant identity primitives.

## Decision: Resolve personal tenant idempotently per authenticated hosted user

**Rationale**: The clarified eligibility rule says any authenticated hosted user is
eligible unless disabled or denied. Activation should create or resolve exactly one
personal tenant for that principal, and concurrent attempts should converge through a
unique principal/personal-tenant activation identity and transactional creation/update
rules.

**Alternatives considered**:

- Create a new tenant on every signup attempt. Rejected because returning users and
  repeated invites would create duplicate personal tenants.
- Require invitation-only activation. Rejected by clarification.
- Require operator approval before personal activation. Rejected because it blocks
  self-serve hosted readiness and contradicts the upstream phase.

## Decision: Add explicit activation API and schema contracts

**Rationale**: API contract stability is the main cross-surface risk. New protected
activation endpoints let existing tenant, billing, onboarding, and chat contracts remain
compatible while exposing a focused activation state and action contract to SDK/web
clients.

**Alternatives considered**:

- Overload `/v1/auth/me` with activation state. Rejected because authentication identity
  and product activation lifecycle have different state transitions and failure modes.
- Overload `/v1/chat/query` to mark activation complete. Rejected because normal chat
  responses include user/query/reply content while activation audit must be metadata-only.
- Add web-only client conventions. Rejected because SDK users also need activation state.

## Decision: Block activation completion until quota baseline is available

**Rationale**: The spec requires quota baseline visibility and clarified that missing
quota baseline blocks activation completion. Existing billing behavior already
distinguishes quota-state unavailable for hosted operation, so activation can reuse that
reason as a readiness blocker rather than showing misleading capacity.

**Alternatives considered**:

- Allow activation with unknown quota. Rejected by clarification and because it weakens
  first-run trust.
- Allow only test chat with conservative default quota. Rejected because it creates a
  hidden capacity rule outside the billing baseline.
- Ignore quota for test activation. Rejected because the spec requires default quota and
  plan projection for new users.

## Decision: Require `test_chat` as the v1 safe first action

**Rationale**: Test chat is the smallest product-shaped action that proves the hosted
tenant can perform useful work without live connectors, production secrets, payment
checkout, or organization setup. It also keeps reminder/provider setup as optional
follow-ups rather than roadmap completion blockers.

**Alternatives considered**:

- Reminder creation. Rejected because reminders introduce scheduling and delivery
  semantics that are outside the minimal activation proof.
- Guided provider setup. Rejected because it can require secrets or external provider
  credentials.
- Any available safe action. Rejected by clarification and because tests would be less
  deterministic.

## Decision: Persist activation audit and diagnostics as metadata only

**Rationale**: Operators need to know whether test chat completed, which stage failed,
and what remediation applies. They do not need user message content. Metadata-only records
reduce privacy risk and make redaction validation concrete.

**Alternatives considered**:

- Store full test chat transcript. Rejected by clarification and privacy risk.
- Store transcript only on failure. Rejected because failures are often the most
  sensitive path and diagnostics can use reason codes and metadata.
- Let tenant policy decide in v1. Rejected because it expands scope into retention and
  policy behavior not required for Roadmap 45.

## Decision: Make activation failure reasons stable and bounded

**Rationale**: Operators must diagnose activation failures without direct storage
inspection, and SDK/web clients must present blocked states without parsing raw error
text. Stable reason codes should cover tenant resolution, eligibility, quota readiness,
authorization, test chat execution, persistence/audit failure, and unexpected failures.

**Alternatives considered**:

- Reuse raw error strings. Rejected because raw text is unstable and not contract-safe.
- Return only HTTP status codes. Rejected because status alone cannot identify the
  failing activation stage or remediation owner.
- Expose internal exception names. Rejected because they leak implementation details and
  create brittle contracts.

## Decision: Keep organization onboarding additive

**Rationale**: The upstream fixed decision says organization onboarding must not block
personal activation. Activation state belongs to the personal tenant by default; org
invitations can appear as follow-up state only after personal activation remains
available.

**Alternatives considered**:

- Require accepting organization invite first. Rejected because it blocks personal
  activation.
- Fold organization administration into activation. Rejected as explicitly out of scope.
- Hide organization invitations entirely. Rejected because invite acceptance is an
  allowed hosted entry, but it remains additive.
