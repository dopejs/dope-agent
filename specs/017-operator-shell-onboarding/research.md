# Research: Operator Shell And Onboarding

## Decisions

### Decision: Ship a single web-first primary operator shell in phase 32

- Rationale: The repository already has a `web` client surface and only a minimal chat
  console there. The clarified spec explicitly allows one primary operator shell and does
  not require TUI parity. Closing roadmap 32 with one web-first shell keeps the change
  reversible and avoids splitting verification across two clients.
- Alternatives considered:
  - Deliver equivalent `web` and `tui` shells in the same roadmap.
    - Rejected because it doubles UI scope, state synchronization work, and acceptance
      coverage without being required by the feature spec.
  - Make TUI the primary shell.
    - Rejected because the repository already has a browser client surface and phase 32 is
      specifically about coherent onboarding and operator inspection for non-developer
      users.

### Decision: Add daemon-owned operator projection routes instead of deriving onboarding and activity summaries in the client

- Rationale: The spec requires the shell to consume daemon truth rather than invent
  client-only state. Existing daemon routes already expose authoritative approvals,
  integrations, connectors, capabilities, schedules, workflows, deliveries, and events,
  but onboarding progress, cross-domain activity, and diagnostic findings are still
  operator-facing projections. Those summaries should be assembled server-side so the web
  client renders daemon-owned truth rather than reimplementing control-plane logic.
- Alternatives considered:
  - Reuse only existing raw routes and compute onboarding/activity/diagnostics entirely in
    React state.
    - Rejected because it would create client-derived summaries that can drift from
      persisted daemon truth and violate the roadmap's operator-trust goal.
  - Persist new operator-shell-only tables.
    - Rejected because the required shell views can be projected from existing daemon
      resources and persisted events without introducing a second source of truth.

### Decision: Keep approval handling on the existing policy routes and use the shell as the action surface

- Rationale: The daemon already supports `GET /v1/policy/approvals`,
  `GET /v1/policy/approvals/{approvalId}`, and
  `POST /v1/policy/approvals/{approvalId}/resolve`. Phase 32 needs the operator shell to
  expose and act on those approvals, not to replace the policy plane with a second
  approval subsystem.
- Alternatives considered:
  - Add an operator-shell-only approval route family.
    - Rejected because it would duplicate approval resolution semantics that already exist
      in the policy plane.
  - Make the inbox inspect-only.
    - Rejected because the clarified spec requires direct approve/reject handling inside
      the shell.

### Decision: Reuse existing detail routes for drill-down and add only three new operator projections

- Rationale: The daemon already exposes inspectable routes for integrations, connectors,
  capabilities, schedules, workflows, deliveries, runs, and events. The minimum additive
  projection set required to close roadmap 32 is:
  - `GET /v1/operator/onboarding`
  - `GET /v1/operator/activity`
  - `GET /v1/operator/diagnostics`
  These routes can summarize operator state while linking back to existing detail routes
  for full inspection and actions.
- Alternatives considered:
  - Create a large operator route surface that mirrors every existing domain resource.
    - Rejected because it would duplicate contracts already owned by those domains.
  - Collapse everything into one monolithic `/v1/operator/shell` payload.
    - Rejected because onboarding, recent activity, and diagnostics evolve at different
      cadences and need independent verification and filtering.

### Decision: Treat the first useful action as a shell-embedded bounded test action that reuses existing daemon execution routes

- Rationale: Clarification narrowed the first useful action to a bounded test query or
  test run that returns immediate result and status feedback in the shell. The web shell
  should therefore embed an operator action panel that reuses existing execution routes
  (`/v1/chat/query` and, when needed, `/v1/runs`) instead of inventing a shell-only
  execution API.
- Alternatives considered:
  - Require background schedule or workflow creation as the first useful action.
    - Rejected because that broadens onboarding into background automation setup and
      exceeds the minimal shell scope.
  - Treat readiness completion alone as the first useful outcome.
    - Rejected because the clarified spec requires an actual bounded action with visible
      outcome.

### Decision: Keep environment selection outside the shell and make environment scope explicit in every operator projection

- Rationale: The clarified spec does not require in-shell switching between test and live
  environments. Phase 32 should display the active environment prominently and enforce
  strict environment scoping for onboarding, approvals, activity, and diagnostics while
  leaving environment selection to the entry point or a later roadmap.
- Alternatives considered:
  - Add test/live switching inside the shell.
    - Rejected because it expands the roadmap into environment switching UX, auth refresh,
      state invalidation, and safety prompts.
  - Limit the shell to test-only behavior.
    - Rejected because the shell must remain truthful about whichever environment the
      daemon is currently serving.

### Decision: Drive shell freshness from existing event streaming plus targeted refetch, not long-lived client-owned shadow models

- Rationale: The daemon already exposes `/v1/events/stream` and persisted event history.
  The shell can load projection routes and detail routes for initial state, then use event
  streaming to trigger bounded refetch of affected views. This keeps the browser reactive
  without turning the client into a second event-processing authority.
- Alternatives considered:
  - Poll every domain route on an interval.
    - Rejected because it increases load and weakens timely operator feedback under
      approval and health changes.
  - Maintain a full browser-side event replay model.
    - Rejected because it recreates daemon-owned truth projection logic in the client.

### Decision: Build diagnostics from explicit reason-bearing resource state and event linkage, not free-form UI heuristics

- Rationale: Existing domain resources already carry reason and status fields such as
  integration readiness reason, connector failure reason, capability status, schedule
  status, workflow status, delivery status, and approval state. Diagnostics should be a
  server-side projection over those explicit signals so the shell can explain blockers and
  failures without relying on brittle client-side interpretation.
- Alternatives considered:
  - Render generic UI errors based on whichever fetch failed.
    - Rejected because it would not explain why background work is blocked or failed.
  - Add a separate persistent diagnostics ledger.
    - Rejected because the shell can derive operator findings from existing resources and
      persisted events.

## Implementation Notes

- Keep operator projections additive in `daemon/internal/api`; do not create a second
-source subsystem outside the existing daemon route layer.
- Prefer derived operator projection types in `daemon/internal/api/types.go` and projection
  helpers in `daemon/internal/api/*_projection.go` style files so the design matches the
  repository's existing pattern.
- Extend `sdk/ts/src/index.ts` to cover new operator projections and reused approval or
  detail routes needed by the web shell; keep `web` consuming the SDK rather than calling
  `fetch` directly.
- Replace the current chat-only web app with a web operator shell that still embeds the
  existing chat-query path as one bounded first-action panel when appropriate.
