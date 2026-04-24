# Research: Evaluation And Replay Harness

## Decision: Add A Focused Evaluation Domain Package

Create `daemon/internal/evaluation` for replay candidates, attempts, comparison results,
drift findings, fixture manifests, and plane-level comparison logic.

**Rationale**: Phase 33 introduces durable evaluation records and lifecycle rules that are
not simple operator projections. Keeping those rules in a focused package avoids bloating
`daemon/internal/api`, while keeping normal run, workflow, schedule, integration,
delivery, policy, and computer-use execution truth in their existing packages.

**Alternatives considered**:

- Put all replay behavior in `daemon/internal/api`: rejected because handlers would own
  cross-domain lifecycle and comparison behavior.
- Extend `daemon/internal/runtime`: rejected because replay/evaluation is control-plane
  evidence over runtime truth, not a replacement for normal execution.
- Implement in the web client: rejected because comparison truth must be daemon-owned and
  restart-safe.

## Decision: Curated Candidate Eligibility Only

Phase 33 replay candidates are limited to curated representative work and
engineer-managed fixtures. Ordinary completed work is not automatically eligible.

**Rationale**: Curated eligibility closes the roadmap with a defensible evidence model,
bounded storage pressure, and predictable regression inputs. Broad automatic eligibility
would create compatibility and retention commitments before the replay model is proven.

**Alternatives considered**:

- Make every completed run/workflow eligible if evidence exists: rejected because it
  expands data retention and replay-readiness promises across all historical work.
- Use short-window automatic eligibility: rejected for phase 33 because it still requires
  retention policy and UX around expiring candidates that the roadmap does not need.

## Decision: Default Replay Is Non-Live And Evidence-Preserving

Replay launch defaults to non-live mode. Real side effects are not executed, and
approval-gated steps are blocked or evaluated from prior evidence unless the operator
explicitly selects live validation.

**Rationale**: This preserves operator trust, protects live external state, and matches the
spec clarification that phase 33 is about deterministic replay and comparison where
possible. Live validation remains available as an explicit scoped choice when needed.

**Alternatives considered**:

- Reuse original approvals automatically: rejected because prior approvals should not
  silently authorize new side effects.
- Require fresh approvals and execute live side effects by default: rejected because it
  makes the default path higher blast-radius than normal regression work.
- Fully simulate all approval and side-effect outcomes: rejected because it can hide
  blocked or missing evidence that operators need to see.

## Decision: Plane-Level Comparison

Comparison results include terminal status plus runtime, policy, integration, delivery,
and evidence summary differences where available. Full artifact equality is not the
default phase 33 requirement.

**Rationale**: Plane-level comparison is useful to operators and testable against current
daemon truth without making every artifact and event payload part of a brittle equality
contract. It also aligns with the upstream requirement to distinguish runtime, policy,
integration, and delivery drift where observable.

**Alternatives considered**:

- Terminal status only: rejected because it would not explain where drift occurred.
- Full event, payload, artifact, and output diffing by default: rejected because it adds
  broad retention and normalization requirements beyond the first reliable replay slice.

## Decision: Web Operator Shell Is Required For Phase 33

The web operator shell must support replay launch and replay/comparison inspection.

**Rationale**: Roadmap 32 established the web shell as the primary operator surface. Phase
33 is operator-facing reliability work; API and docs alone would leave operators back at
raw route reconstruction.

**Alternatives considered**:

- API/contracts/docs only: rejected because it does not satisfy operator-visible replay
  outcomes.
- Read-only web shell history with launch via API: rejected because phase 33 requires
  operators to initiate replay from the shell.

## Decision: Add Schema-Backed Evaluation Routes And Events

Expose `/v1/evaluation/*` routes for candidates, attempts, comparisons, and fixtures.
Publish additive events for replay launch, replay terminal state, and comparison
completion.

**Rationale**: A dedicated evaluation route family keeps replay contracts discoverable and
versionable while allowing the web shell and SDK to consume typed surfaces. Events provide
bounded freshness hints without making the browser reconstruct state from event history.

**Alternatives considered**:

- Nest all routes under `/v1/operator`: rejected because evaluation records are durable
  domain resources, not only operator projections.
- Only emit events and derive state from event history: rejected because restart-safe
  inspection should read durable resources directly.

## Decision: Store Evaluation History Durably

Persist candidates, attempts, comparisons, drift findings, fixture metadata, environment
scope, source references, safety scope, and readiness limitations in SQLite.

**Rationale**: The spec requires replay and comparison artifacts to remain visible after
restart. Durable records also let the web shell show prior evaluation work without
re-running comparisons.

**Alternatives considered**:

- Derive everything from existing events on demand: rejected because comparison summaries
  and fixture metadata need stable identifiers and restart-safe inspection.
- Keep only repo fixture files: rejected because replay attempts and comparison results
  are runtime evidence produced by operators.

## Decision: Use Repo-Managed Fixtures For Regression Coverage

Engineer-owned fixture definitions and captured evidence should live in repo-managed test
or fixture paths, with manifest metadata loaded by the daemon/evaluation tests.

**Rationale**: This supports repeatable schedule, integration, and computer-use coverage
without building in-product fixture authoring. It also keeps fixture review and updates in
normal code review.

**Alternatives considered**:

- Operator-authored in-product fixtures: rejected by clarification and scope.
- Hidden test-only fixtures with no operator metadata: rejected because operators must be
  able to inspect fixture provenance and limitations.
