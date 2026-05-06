# Contract: Activation Audit And Diagnostics

## Audit Events

Activation state changes must be tenant-scoped and audit-visible through the existing
tenant audit path or a versioned event/schema if implementation promotes the event.

### Event Kinds

- `tenant.activation_started`
- `tenant.activation_blocked`
- `tenant.activation_completed`
- `tenant.activation_test_chat_completed`
- `tenant.activation_failed`
- `tenant.activation_denied`

### Required Metadata

- `activationId`
- `tenantId` when resolved
- `principalId`
- `tokenId` when available
- `stage`
- `fromStatus`
- `toStatus`
- `reasonCode`
- `retryable`
- `remediationOwner`
- `testChat.dispatchId` when applicable
- `testChat.status` when applicable
- `testChat.provider` when applicable
- `testChat.model` when applicable
- `testChat.finishReason` when applicable

## Forbidden Data

Audit, diagnostics, logs, fixtures, and activation persistence must not contain:

- raw secrets
- access tokens
- authorization headers
- app secrets
- refresh credentials
- credential-bearing environment values
- inaccessible tenant details
- test chat query text
- test chat reply text
- test chat transcript
- streamed deltas
- prompts
- raw provider payloads

## Diagnostics Projection

Operator-facing activation diagnostics must identify:

- activation id
- tenant id when accessible
- principal id or safe principal label
- activation status
- failing stage
- stable reason code
- retryability
- remediation owner
- last state transition time
- relevant readiness item ids
- quota baseline status when relevant
- test chat metadata when relevant

Diagnostics must not require direct storage inspection for representative failures.

## Required Tests

- Every activation state transition writes audit-visible metadata.
- Audit write failure is fail-closed for security-relevant activation completion.
- Test chat success records dispatch/status metadata only.
- Test chat failure records stable reason metadata only.
- Redaction validation rejects transcripts, prompts, raw query/reply content, and
  credential-bearing values.
- Diagnostics identify tenant resolution, eligibility, quota baseline, authorization,
  test chat, audit, persistence, and unexpected failure classes.
