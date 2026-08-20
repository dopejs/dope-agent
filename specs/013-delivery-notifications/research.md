# Research: Delivery And Notifications

## Decisions

### Decision: Introduce a dedicated daemon-owned delivery plane instead of treating connector reply persistence as the delivery model

- Rationale: Existing `connector_messages` records capture transport-specific inbound or
  outbound message durability for channel loops, but roadmap 28 needs environment-scoped
  delivery targets, user-default and integration-specific preference resolution, digest
  windows, suppression semantics, and explicit separation between execution outcome and
  delivery outcome. A dedicated delivery plane keeps those concepts inspectable without
  overloading connector reply records or run status.
- Alternatives considered:
  - Extend `connector_messages` into the primary delivery ledger.
    - Rejected because connector message records do not own target selection,
      suppression, digest grouping, or source-linkage semantics for schedules and
      workflows.
  - Store delivery status only as extra fields on runs or workflows.
    - Rejected because operators need a separate delivery ledger with per-attempt history
      and target-specific truth, not a collapsed execution status flag.

### Decision: Resolve each result to exactly one preferred target using environment-scoped user defaults with optional integration-specific overrides

- Rationale: Clarification fixed phase 28 to single-target routing, user-default
  preference ownership, and optional integration overrides. That keeps the first delivery
  slice explainable, auditable, and compatible with the single-operator environment model
  while still allowing integration-sensitive routing in later domain phases.
- Alternatives considered:
  - Fan out every result to all active targets.
    - Rejected because broadcast semantics would complicate duplicate handling, partial
      success, and user-visible noise before the shared delivery plane is stable.
  - Make all preferences integration-specific only.
    - Rejected because schedules and generic workflows need reusable user-level routing
      defaults.

### Decision: Preserve per-attempt delivery history and keep retries bound to the chosen target with no automatic failover

- Rationale: The spec and clarifications require explicit retry, suppression, and failure
  truth. Preserving every attempt under one outcome record gives operators a defensible
  audit trail and avoids hiding transport churn behind a final state. Keeping retries
  bound to the chosen target prevents silent duplicate notifications and keeps target
  selection deterministic.
- Alternatives considered:
  - Store only a final delivery state or aggregate retry count.
    - Rejected because it weakens debuggability and makes transport failure analysis
      harder.
  - Automatically fail over to a secondary target after retry exhaustion.
    - Rejected because it breaks the clarified single-target routing rule and risks
      double delivery or confusing partial-success semantics.

### Decision: Emit delivery requests only from terminal background result boundaries, not from intermediate step or tool-call events

- Rationale: Roadmap 28 is about returning outcomes, alerts, and summaries to the user.
  Terminal run, workflow, and integration-backed result boundaries are the right place to
  construct a user-facing result message without turning every internal step into a
  notification source. This also keeps delivery truth additive to execution truth instead
  of interleaving the two ledgers.
- Alternatives considered:
  - Emit delivery attempts directly from tool-call completion.
    - Rejected because it would spam users with low-level internal progress and confuse
      execution detail with user-facing results.
  - Let every source subsystem perform its own transport send logic.
    - Rejected because it would recreate the fragmented, source-specific delivery behavior
      roadmap 28 is meant to close.

### Decision: Limit digest windows to routine-success outcomes while failures and urgent results bypass the digest path

- Rationale: Clarification established that routine successes may be summarized, but
  failures and urgent results must deliver immediately. That preserves an ambient summary
  experience without burying important failures inside a time bucket.
- Alternatives considered:
  - Allow all result classes to be digest-only when a summary window exists.
    - Rejected because important failures would become easier to miss.
  - Disable digest behavior entirely in phase 28.
    - Rejected because summary scaffolding is explicitly in scope and later reminder or
      memory-driven features depend on it.

### Decision: Reuse existing connector outbound persistence as transport evidence and add a repo-owned `test_sink` adapter for deterministic local verification

- Rationale: A connector-backed adapter keeps the delivery plane grounded in a real
  outward transport path, while a repo-owned `test_sink` target lets `KURA_ENV=test`
  verify routing, payload preview, retry, suppression, and digest behavior without live
  connector credentials. Both adapters converge on the same target, attempt, and outcome
  resources.
- Alternatives considered:
  - Require a live Discord or other real connector to verify roadmap 28.
    - Rejected because local planning and verification should stay safe by default in the
      test environment.
  - Use only a fake sink and postpone connector-backed delivery entirely.
    - Rejected because the roadmap should still prove the shared delivery plane can drive
      at least one operator-realistic transport path.

## Implementation Notes

- `daemon/internal/delivery` should own target registration, preference lookup, summary
  window membership, attempt lifecycle, and transport adapter coordination.
- `daemon/internal/app` should restore persisted delivery state and restart any eligible
  retry or digest timers after the store, event bus, runtime, scheduler, and connectors
  are ready.
- Source execution owners such as scheduler, runtime, orchestration, and integrations
  should publish additive delivery-eligible completion facts instead of directly sending
  connector messages.
- Connector-backed delivery should link transport attempt receipts back to existing
  `connector_messages` rows rather than duplicating outbound transport state in a second
  connector-specific table.
