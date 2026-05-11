# Research: Group Room Reset Handoff

## Conversation Shape As Explicit Product State

Decision: Represent direct-message, group, room, and web-originated conversation shape as
explicit thread/source metadata with conservative `unknown` or `unsupported` outcomes for
legacy or incomplete evidence.

Rationale: Roadmap 56 depends on predictable group/room isolation. Explicit shape avoids
inferring public/private behavior from message text, display names, participant overlap,
or connector-local state. Conservative unknown handling preserves backward compatibility
without granting implicit group behavior.

Alternatives considered:
- Infer shape from connector names or participant count. Rejected because it is fragile
  across renamed rooms, shared channels, and connector replay.
- Treat all channel-origin conversations as equivalent. Rejected because direct-message,
  group, and room reset and participation semantics differ materially.

## Default Group And Room Participation Policy

Decision: Default group/room participation requires both allowlist eligibility and a
qualifying mention.

Rationale: Shared spaces need fail-closed routing. Mention-only can invite participation
in rooms the tenant has not approved, while allowlist-only can make the assistant respond
unexpectedly to ordinary room traffic. Requiring both conditions is the smallest safe
default and allows broader room-level behavior to be added later by explicit policy.

Alternatives considered:
- Allowlist-only participation. Rejected because it can produce surprising responses in
  busy rooms.
- Mention-only participation. Rejected because it bypasses room-level tenant approval.
- Disabled-by-default only. Rejected because the spec requires mention and allowlist
  policy linkage as an implementable default.

## Handoff Identity Model

Decision: Handoff creates or selects a separate destination thread and links it to the
source thread by a handoff record.

Rationale: Separate destination identity preserves source isolation, keeps reset scope
clear, makes permission checks explicit on both sides, and matches the upstream
requirement for source and destination references. A handoff record provides traceability
without merging unrelated surface state.

Alternatives considered:
- Reuse one daemon thread identity across source and destination. Rejected because it
  blurs source-specific reset, destination participation policy, and inspection
  permission boundaries.
- Start an unrelated destination thread with only a UI link. Rejected because it fails
  the user goal of continuing a thread with traceable handoff.

## Handoff Context Bridge

Decision: Handoff may reference eligible current-segment source turns for the first
destination response only, subject to source permission, destination permission,
redaction, retention, and Roadmap 55 continuity eligibility. Source turns are never copied
into destination history.

Rationale: A one-response bridge lets the user continue meaningfully across surfaces
while avoiding persistent cross-surface hidden context. References keep evidence
auditable and avoid rewriting history. After the first destination response, normal
destination-thread continuity takes over unless another authorized handoff occurs.

Alternatives considered:
- No source context. Rejected because it makes handoff mostly a link rather than a
  continuation.
- Copy source turns into destination history. Rejected because it creates duplicate and
  misleading destination turns.
- Keep source references for the whole destination segment. Rejected because it creates a
  long-lived cross-surface dependency that resembles memory.

## Reset And Handoff Permission Boundary

Decision: Reset and handoff creation both require `connectors.manage`; inspection uses
the existing thread inspection boundary.

Rationale: Reset and handoff creation mutate lifecycle or routing state. Roadmap 54
already uses `connectors.manage` for lifecycle mutations, so reusing it avoids a new
tenant permission and preserves existing operator expectations.

Alternatives considered:
- Use `credentials.inspect`. Rejected because inspection permission should not mutate
  lifecycle or routing state.
- Require only participant access for handoff. Rejected because handoff can expose and
  route cross-surface context.
- Add a new handoff permission. Rejected for this phase because existing lifecycle
  mutation permission is sufficient and lower risk.

## Connector Conformance Additions

Decision: Connector conformance must declare and test support for conversation shape,
room identity, mention detection, allowlist evaluation, duplicate detection, reset
support, handoff source/destination support, and safe inspection evidence.

Rationale: Connectors differ in room identifiers, mention semantics, shared channel
behavior, and replay/edit/delete events. Explicit conformance prevents unsupported
connectors from silently receiving group or handoff semantics.

Alternatives considered:
- Assume all connectors can supply room and mention evidence. Rejected because existing
  connector coverage is uneven and some sources cannot prove the required identity.
- Implement per-connector ad hoc behavior without shared contract. Rejected because
  operator evidence and tests would drift across Discord, Slack, Telegram, Matrix, and
  future channels.

## Retention, Redaction, And Restart Recovery

Decision: Group/room/reset/handoff evidence follows Roadmap 54 retention and redaction
rules and must survive daemon restart with enough metadata to explain final status.

Rationale: This feature changes routing and context behavior. Operators need product
evidence after restarts without exposing unsafe provider data. Reusing Roadmap 54
retention and Roadmap 55 redaction/continuity eligibility keeps the blast radius small.

Alternatives considered:
- Store only transient logs. Rejected because logs are not sufficient product evidence
  and may be unavailable after restart.
- Persist raw connector payloads for debugging. Rejected because it violates secret and
  message-body safety constraints.
