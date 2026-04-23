# Research: Mail Integration

## Decisions

### Decision: Introduce a dedicated daemon-owned mail domain package instead of extending integration probes

- Rationale: Phase 27 integration probes prove shared readiness and provenance behavior,
  but they do not model mailbox identity, thread inspection, draft lifecycle,
  send-path truth, or auditable attachment failures. Roadmap 30 needs a first-class mail
  domain with its own resource and operation vocabulary.
- Alternatives considered:
  - Extend `daemon/internal/integrations` probe routes with mail-specific verbs.
    - Rejected because it would blur shared readiness infrastructure with domain
      behavior and make phase 27's fake probe path the accidental long-term mail API.
  - Drive mail behavior only through generic workflow or tool-call payloads.
    - Rejected because operators need stable domain-owned routes and artifacts for
      mailbox inspection, draft state, and outbound-side-effect truth.

### Decision: Resolve mailbox selection through explicit integration choice or canonical default, then project mailbox identity in the mail domain

- Rationale: The spec requires mailbox readiness and default selection to reuse phase 27
  semantics, while phase 30 also needs mailbox-specific identity and capability truth
  that do not belong on the generic integration resource. The mail domain should
  therefore derive mailbox selection from integrations and persist its own account
  projection.
- Alternatives considered:
  - Copy mailbox-specific fields directly onto integration resources.
    - Rejected because it would push domain-specific state back into the shared
      integrations plane.
  - Require every request to name an integration explicitly.
    - Rejected because phase 30 clarification fixed default behavior to canonical
      default mailbox selection when no explicit `integrationId` is provided.

### Decision: Model mail work as explicit operation records with structured thread, message, draft, and attachment artifacts

- Rationale: Operators need to distinguish mailbox selection, inspection, draft-only
  work, direct send, send-existing-draft, reply, forward, attachment failure, and
  downstream delivery. Persisting one `mail_operation` record per domain action plus
  structured artifacts preserves truthful domain history without re-reading live backend
  state later.
- Alternatives considered:
  - Derive audit truth only from runtime tool-call input and output blobs.
    - Rejected because tool-call payloads are too generic and make domain-specific
      inspection, filtering, and failure analysis harder.
  - Persist only sent-message snapshots with no explicit operation resource.
    - Rejected because thread inspection, draft updates, blocked sends, and background
      workflow linkage also need first-class operator-visible truth.

### Decision: Keep outbound semantics explicit by separating direct send, send-existing-draft, reply, and forward truth

- Rationale: Clarification established that roadmap 30 supports both direct send and
  send-existing-draft, but operator-visible history must distinguish them. Reply and
  forward are also separate operation classes, not aliases for generic send. Modeling
  explicit operation classes and send-path metadata keeps audit truth honest and keeps
  downstream tests precise.
- Alternatives considered:
  - Collapse all outbound actions into one generic `send` operation.
    - Rejected because it would hide whether the action used a draft, direct content, or
      an existing conversation.
  - Require every outbound action to create a draft first.
    - Rejected because the clarified spec explicitly allows direct send in addition to
      send-existing-draft.

### Decision: Restrict attachment handling to metadata and failure truth, and block final send when required attachment references are unresolved

- Rationale: The clarified spec intentionally narrows phase 30 attachment scope to
  metadata and failure truth, not generalized file transfer. If a send request depends
  on unresolved attachment references, the safest truthful behavior is to block final
  send and preserve explicit failure or draft-only truth rather than sending incomplete
  mail.
- Alternatives considered:
  - Allow degraded send while merely flagging missing attachments.
    - Rejected because it would create a high-risk mismatch between what the user asked
      to send and what was actually sent.
  - Include full attachment upload/download support in phase 30.
    - Rejected because it widens scope into binary transfer, storage, and verification
      work that the roadmap does not require.

### Decision: Require explicit recipients for new outbound mail and explicit workflow send permission for background final send

- Rationale: Clarification established that new outbound mail must use only recipients
  explicitly provided in the current request, and background workflows may finalize send
  only when they explicitly declare send-side-effect permission. These constraints reduce
  the highest-risk accidental-send cases without weakening mailbox inspection or draft
  flows.
- Alternatives considered:
  - Infer recipients for new outbound mail from mailbox context or prior drafts.
    - Rejected because the resulting accidental-send risk is too high for the first
      production mail slice.
  - Let any background workflow finalize send if it references a mail action.
    - Rejected because the spec explicitly requires a separate send-side-effect
      declaration for background execution.

### Decision: Extend the repo-owned fake integration backend into a deterministic fake mail backend for verification

- Rationale: The constitution requires `DOPE_ENV=test` by default and the spec allows a
  local or fake verification path. Extending the fake integration backend to supply one
  deterministic mailbox projection, seeded threads and messages, draft lifecycle, direct
  send, reply, and forward behavior is enough to validate mailbox projection, send-path
  truth, attachment-failure blocking, workflow gating, and delivery reuse without live
  external dependencies.
- Alternatives considered:
  - Require a real Gmail, Outlook, or IMAP sandbox to close roadmap 30.
    - Rejected because roadmap closure would then depend on live credentials and
      third-party API stability.
  - Mock the mail manager only in unit tests with no operator-facing HTTP path.
    - Rejected because the roadmap also requires operator-visible surfaces and one manual
      verification path.

## Implementation Notes

- Reuse immutable `integrationBindings` on tool calls and workflow steps, but attach
  `mailOperationSummaries` so operators can distinguish integration selection from
  mail-domain truth.
- Preserve truthful distinction between outbound result mode (`draft_only` versus `sent`)
  and outbound send path (`direct` versus `draft`) in mail-operation records and
  summaries.
- Keep fake mail verification intentionally deterministic: one healthy mailbox
  projection, one seeded inbound thread, one seeded draft, explicit-recipient direct
  send, send-existing-draft, reply, forward, one blocked background send, and one
  unresolved-attachment failure are sufficient to close roadmap 30.
