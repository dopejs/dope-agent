# Research: Computer-Use Capability Plane

## Decisions

### Decision: Introduce a daemon-owned computer-use control plane with browser session and action resources scoped to a run

- Rationale: Phase 26 needs first-class operator-visible truth for browser work without
  creating an execution path outside the current runtime plane. A daemon-owned
  `computer-use` module can manage browser sessions, action history, target matching, and
  evidence while keeping each session scoped to the run or workflow that owns it.
- Alternatives considered:
  - Keep browser execution as an opaque `browser` capability tool call.
    - Rejected because the current implementation is only a policy placeholder and cannot
      expose inspectable session, action, or evidence truth.
  - Create browser sessions as top-level resources independent of runs.
    - Rejected because phase 26 clarified that sessions may be reused inside a run or
      workflow but must not survive across separate runs or schedule dispatches.

### Decision: Execute computer-use actions through normal runtime steps and tool calls with additive session/action linkage

- Rationale: The spec requires computer-use work to remain on the existing runtime plane.
  Each browser action therefore maps to a normal runtime step and tool call, while
  additive session and action IDs on the tool-call truth make computer-use inspection
  explicit.
- Alternatives considered:
  - Build a parallel computer-use execution ledger outside runtime step/tool-call truth.
    - Rejected because it would weaken auditability and force operators to correlate two
      execution systems.
  - Record only session-level summaries and omit per-action runtime linkage.
    - Rejected because approval outcomes, target mismatch, and evidence need concrete
      action-level attribution.

### Decision: Use risk-based approvals on concrete computer-use actions rather than capability-wide approvals

- Rationale: Phase 26 clarified that only high-risk actions need approval. Approval must
  bind to the specific action, target context, and page evidence that the operator is
  reviewing, while lower-risk read-only or in-page inspection actions remain unblocked
  unless policy elevates them.
- Alternatives considered:
  - Require approval for every browser action.
    - Rejected because it makes multi-step browser work operationally unusable and is not
      required by the clarified spec.
  - Keep approval at the coarse capability instance level only.
    - Rejected because a capability-wide approval cannot preserve action-specific trust,
      target context, or audit truth.

### Decision: Persist target-match context per action and fail immediately on mismatch

- Rationale: The clarified target-mismatch behavior is “fail immediately, preserve
  evidence, require renewed inspection.” Each action therefore needs stored target-match
  context describing the expected page or element and the observed state at execution
  time.
- Alternatives considered:
  - Auto-relocate a similar target and continue.
    - Rejected because it would let the daemon make unreviewed UI decisions after page
      drift.
  - Retry navigation or DOM lookup automatically on mismatch.
    - Rejected because mismatch is a trust boundary, not just a transient failure.

### Decision: Keep phase 26 browser sessions single-page and reusable only inside one run or workflow

- Rationale: The clarified phase 26 boundary is one active page per session, no multi-tab
  or multi-window behavior, and no cross-run reuse. This keeps session lifecycle,
  evidence capture, and restart handling small enough to close the roadmap safely.
- Alternatives considered:
  - Support multi-tab workflows in the first slice.
    - Rejected because it would expand target tracking, approval context, and state
      restoration complexity.
  - Force a fresh browser session for every action.
    - Rejected because it would make realistic multi-step workflows impossible.

### Decision: Capture screenshots, page snapshots, and downloads as first-class computer-use artifacts with metadata and file-backed content

- Rationale: The upstream spec requires evidence to become first-class artifacts. The
  daemon should therefore persist artifact metadata in store-backed resources and use the
  existing `artifacts` package as the owner for file-backed evidence blobs referenced from
  computer-use actions.
- Alternatives considered:
  - Store only file paths in action output.
    - Rejected because operators need stable, environment-scoped artifact truth and
      additive schema coverage.
  - Emit evidence only as transient events.
    - Rejected because screenshots and downloads must remain inspectable after restart.

### Decision: Keep the computer-use control plane daemon-owned while isolating browser-consumer implementation behind a replaceable driver boundary

- Rationale: Current docs and repo architecture expect heavier browser stacks to be
  isolatable later. Phase 26 should keep resource ownership, approval, persistence, and
  audit truth in the daemon while hiding the underlying browser driver behind a small
  internal interface that can later target a dedicated `capabilities/browser` worker or a
  stronger sandbox backend.
- Alternatives considered:
  - Embed browser stack specifics directly into API handlers.
    - Rejected because it would couple policy and persistence to a fragile runtime.
  - Block phase 26 until an isolated browser worker exists.
    - Rejected because the roadmap explicitly allows stronger isolation reuse later without
      making it a prerequisite for the first capability-plane slice.

## Implementation Notes

- Add a new `daemon/internal/computeruse` package to own session lifecycle, action
  dispatch, target-match validation, and driver abstraction.
- Extend `daemon/internal/runtime` tool-call truth with additive computer-use linkage
  fields instead of creating a new execution ledger.
- Use `daemon/internal/artifacts` for evidence metadata and persisted content rather than
  inventing ad hoc browser-output handling inside API handlers.
- Keep session creation and action execution scoped to a run so schedule-launched and
  workflow-launched browser work automatically inherits existing environment, policy, and
  audit boundaries.
