# Research: Calendar Integration

## Decisions

### Decision: Introduce a dedicated daemon-owned calendar domain package instead of extending integration probes

- Rationale: Phase 27 integration probes prove shared readiness and provenance behavior,
  but they do not model calendar account projection, event identity, busy/free semantics,
  or auditable event mutation. Roadmap 29 needs a first-class calendar domain with its
  own resource and operation vocabulary.
- Alternatives considered:
  - Extend `daemon/internal/integrations` probe routes with calendar-specific verbs.
    - Rejected because it would blur shared readiness infrastructure with domain behavior
      and make phase 27's fake probe path the accidental long-term calendar API.
  - Drive calendar behavior only through generic workflow or tool-call payloads.
    - Rejected because operators need stable domain-owned routes and artifacts for event
      inspection and mutation truth.

### Decision: Resolve calendar account selection through explicit integration choice or canonical default, then project primary-calendar metadata in the calendar domain

- Rationale: The spec requires calendar account readiness and default selection to reuse
  phase 27 semantics, while phase 29 also needs primary calendar and primary timezone
  truth that do not belong on the generic integration resource. The calendar domain should
  therefore derive account selection from integrations and persist its own account
  projection.
- Alternatives considered:
  - Copy calendar-specific timezone and calendar-container fields onto integration
    resources.
    - Rejected because it would push domain-specific state back into the shared
      integrations plane.
  - Require every request to name an integration and calendar explicitly.
    - Rejected because phase 29 clarification fixed the default behavior to the canonical
      default integration and primary calendar.

### Decision: Model calendar work as explicit operation records with linked structured artifacts

- Rationale: Operators need to distinguish account selection, busy/free evaluation,
  event mutation, stale-state failure, and downstream delivery. Persisting one
  `calendar_operation` record per domain action plus structured event or availability
  artifacts preserves truthful domain history without re-reading live backend state later.
- Alternatives considered:
  - Derive audit truth only from runtime tool-call input and output blobs.
    - Rejected because tool-call payloads are too generic and make domain-specific
      inspection, filtering, and failure analysis harder.
  - Persist only event snapshots with no operation resource.
    - Rejected because busy/free lookups, failed mutations, and background workflow
      linkage also need first-class operator-visible truth.

### Decision: Constrain phase 29 mutation to timed single events on the primary calendar with no attendee semantics

- Rationale: Clarification established that recurring events, all-day events, attendee
  invitation, RSVP tracking, and alternate-calendar selection are out of scope. Encoding
  these constraints directly in the plan keeps the first calendar slice roadmap-closed
  and testable.
- Alternatives considered:
  - Include recurring or all-day mutation in the first slice.
    - Rejected because event-series identity, date-boundary handling, and broader test
      matrices would inflate scope beyond the roadmap.
  - Support attendee fields without invitation or RSVP truth.
    - Rejected because partial attendee semantics would appear successful without a
      truthful external-side-effect model.

### Decision: Run background calendar work through the existing schedule and workflow planes, then reuse delivery outcomes from phase 28

- Rationale: The roadmap explicitly requires schedule-triggered workflows and shared
  delivery reuse. Calendar operations should therefore attach to existing runtime and
  workflow truth with additive calendar-operation summaries instead of introducing a
  calendar-only background executor or notification plane.
- Alternatives considered:
  - Add a calendar-specific scheduler or notification channel.
    - Rejected because phase 25 and 28 already own those responsibilities and duplicating
      them would fracture background-result truth.
  - Limit calendar operations to foreground requests only.
    - Rejected because the roadmap definition of done includes schedule-triggered
      workflows and background delivery.

### Decision: Extend the repo-owned fake integration backend into a deterministic fake calendar backend for verification

- Rationale: The constitution requires `DOPE_ENV=test` by default and the spec allows a
  local or fake verification path. Extending the fake integration backend to supply one
  deterministic primary calendar and timed single-event lifecycle is enough to validate
  account projection, mutation truth, stale-state handling, schedule/workflow linkage,
  and delivery reuse without live external dependencies.
- Alternatives considered:
  - Require a real Google, Outlook, or CalDAV sandbox to close roadmap 29.
    - Rejected because roadmap closure would then depend on live credentials and external
      API stability.
  - Mock the calendar manager only in unit tests with no operator-facing HTTP path.
    - Rejected because the roadmap also requires operator-visible surfaces and one manual
      verification path.

## Implementation Notes

- Reuse immutable `integrationBindings` on tool calls and workflow steps, but attach
  calendar-operation summaries so operators can distinguish integration selection from
  calendar-domain truth.
- Treat the bound calendar account's primary timezone as the default interpretation and
  rendering timezone for timed-event writes unless a later phase adds explicit override
  behavior.
- Persist event snapshots and availability summaries at operation time; do not recalculate
  them from live backend state when operators inspect history later.
