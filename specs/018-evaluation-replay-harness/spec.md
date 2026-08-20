# Feature Specification: Evaluation And Replay Harness

**Feature Branch**: `[018-evaluation-replay-harness]`  
**Created**: 2026-04-24  
**Status**: Draft  
**Input**: User description: "结合 docs/specs/018-evaluation-and-replay-harness.md 完成 phase 33 的工作"

## Clarifications

### Session 2026-04-24

- Q: How should approval-gated or side-effecting steps behave in the default replay mode? → A: Default to non-live replay: preserve the original boundaries, do not execute real side effects, and treat approval-gated steps as blocked or simulated evidence unless the operator explicitly enables live validation.
- Q: How should regression fixtures be authored in phase 33? → A: Fixtures are curated by engineers through repo-managed definitions and captured evidence; operators consume them for replay/comparison but do not create or edit fixtures in-product.
- Q: What operator-facing surface must phase 33 include for replay and comparison? → A: The web operator shell must support launching replays and inspecting replay/comparison outcomes in phase 33.
- Q: Which completed work should become replay candidates by default? → A: Only curated representative work and engineer-managed fixtures are replay-candidate eligible in phase 33.
- Q: What level of comparison detail is required for phase 33? → A: Plane-level comparison: terminal status plus runtime, policy, integration, delivery, and evidence summary differences where available.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Replay Representative Agent Work (Priority: P1)

As an operator, I need to replay curated representative personal-agent work after changing
policies, integrations, or capabilities so I can see whether the system still behaves as
expected without rebuilding the scenario by hand.

**Why this priority**: Phase 33 exists to make behavior changes auditable before
knowledge-plane work becomes the main differentiator. If operators cannot replay important
workflows from captured truth, the system remains too fragile to evolve safely.

**Independent Test**: Select a curated representative prior run, workflow, or
engineer-managed fixture, start a replay from captured evidence through the web operator
shell, and inspect the replay outcome and any missing prerequisites without using raw
logs, direct datastore access, or ad hoc scripts to reconstruct the scenario.

**Acceptance Scenarios**:

1. **Given** a curated representative run, workflow, or engineer-managed fixture exists,
   **When** the operator starts a replay from the web operator shell, **Then** the system
   creates a new replay attempt linked to the source work and preserves the source
   provenance used for the replay.
2. **Given** a representative scenario depends on artifacts, policies, integrations, or
   approvals, **When** the operator inspects replay readiness before launch, **Then** the
   system explains whether the scenario is fully replayable, partially replayable, or
   blocked and identifies the missing prerequisite.
3. **Given** the source work included side-effect boundaries or approval gates,
   **When** the operator launches a replay, **Then** the replay respects those safety
   boundaries, defaults to non-live validation, and does not silently widen the original
   blast radius.
4. **Given** the replay finishes, **When** the operator inspects the result, **Then** the
   system shows the replay outcome, links it back to the source scenario, and retains the
   evidence needed for later audit.

---

### User Story 2 - Compare Before And After Outcomes (Priority: P2)

As an operator, I need to compare baseline and replay outcomes so I can tell whether a
change introduced runtime drift, policy drift, integration drift, delivery drift, or no
material difference.

**Why this priority**: Replaying work without comparison only proves that something ran.
Phase 33 must make changes explainable by showing what changed, where it changed, and
whether the difference is expected or problematic.

**Independent Test**: Run a baseline and replay comparison for a representative scenario,
then inspect an operator-visible web shell summary that classifies terminal status plus
runtime, policy, integration, delivery, and evidence summary differences where available,
without manual log diffing.

**Acceptance Scenarios**:

1. **Given** a replay has both source and replay outcomes available, **When** the
   operator opens the comparison view in the web operator shell, **Then** the system
   presents before-and-after terminal status, operational-plane summaries, and evidence
   summary differences where available.
2. **Given** the replay result differs because a policy, approval expectation, or safety
   boundary changed, **When** the operator inspects the comparison, **Then** the
   difference is attributed to policy drift rather than shown as an unexplained failure.
3. **Given** the replay result differs because an integration, external dependency, or
   delivery path changed, **When** the operator inspects the comparison, **Then** the
   system distinguishes that change from runtime execution drift where evidence allows.
4. **Given** the replay cannot produce a fair comparison because required evidence is
   missing or the scenario is no longer replayable, **When** the operator inspects the
   comparison, **Then** the system reports the limitation explicitly instead of implying a
   false regression or false success.

---

### User Story 3 - Maintain Regression Fixtures For Real Agent Flows (Priority: P3)

As an engineer, I need reusable regression fixtures for schedules, integrations, and
computer-use paths so I can validate representative personal-agent behavior after changes
without depending on improvised one-off checks.

**Why this priority**: Roadmap 33 is not complete with only one manual replay screen.
Engineers need durable regression inputs that cover the real product surface, otherwise
evaluation remains fragile and non-repeatable.

**Independent Test**: Add or refresh representative fixtures for schedule-driven work, an
integration-backed workflow, and a computer-use path, then run replay and comparison
against those fixtures in normal regression flows.

**Acceptance Scenarios**:

1. **Given** an engineer curates a representative scenario into a regression fixture,
   **When** that fixture is inspected later, **Then** it retains provenance back to the
   original scenario, its assumptions, and any declared replay limitations.
2. **Given** an operator uses a curated regression fixture, **When** they launch or inspect
   replay and comparison for that fixture, **Then** they can consume the fixture outcome
   without needing in-product fixture authoring or editing controls.
3. **Given** a supported fixture exists, **When** automated regression runs execute,
   **Then** the fixture can be replayed and compared without manual reconstruction of the
   scenario inputs or expected evidence.
4. **Given** a scenario involves schedules, integrations, or computer use, **When** the
   engineer adds it to the regression set, **Then** the fixture preserves the relevant
   truth needed to evaluate that domain rather than flattening it into a generic summary.
5. **Given** an operator or engineer needs higher-confidence validation after a change,
   **When** they follow the documented verification path, **Then** at least one
   representative before-and-after flow can be exercised as a manual acceptance check in a
   controlled environment.

### Edge Cases

- If a source scenario references artifacts, approvals, or external state that no longer
  exist, the replay remains inspectable as partially replayable or blocked rather than
  disappearing from evaluation history.
- If the original scenario depended on live external behavior that cannot be reproduced
  safely, the system makes that limitation explicit and avoids treating the replay as a
  deterministic regression source.
- If a replay includes approval-gated or side-effecting steps and the operator does not
  explicitly opt into live validation, the system leaves those steps blocked or uses prior
  evidence only, rather than silently reusing production-impacting behavior.
- If the baseline completed with an interrupted, denied, or failed outcome, the replay and
  comparison preserve that terminal truth instead of normalizing it into a generic
  success/failure pair.
- If the replay target environment differs from the source environment, the system makes
  that scope difference explicit so operators do not treat cross-environment comparison as
  a like-for-like result by accident.
- If the same representative scenario is replayed multiple times, operators can tell which
  replay belongs to which change window and which baseline it was compared against.
- If completed work exists but has not been curated as representative work or included in
  an engineer-managed fixture, the system does not present it as replay-candidate eligible
  for phase 33.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide an evaluation and replay harness for representative
  personal-agent work that operators can inspect and run without reconstructing the source
  scenario manually.
- **FR-001a**: Phase 33 MUST expose replay launch and replay/comparison inspection in the
  web operator shell; raw daemon routes, contracts, or docs alone are not sufficient to
  claim operator-facing completion.
- **FR-002**: Operators MUST be able to select curated representative completed work as
  replay candidates, including run or workflow envelopes and related schedule,
  integration, computer-use, artifact, approval, and delivery context when present.
- **FR-002a**: Phase 33 replay-candidate eligibility MUST be limited to curated
  representative work and engineer-managed fixtures; ordinary completed work is not
  automatically replay-candidate eligible.
- **FR-003**: Every replay candidate MUST preserve explicit provenance to the source work,
  including the evidence and contextual truth used to define what is being replayed.
- **FR-004**: Before a replay starts, the system MUST state whether the candidate is fully
  replayable, partially replayable, or blocked, and it MUST explain any missing
  prerequisites or limitations.
- **FR-005**: Replay execution MUST remain within the existing control-plane and
  runtime-truth model so replay outcomes are governed, inspectable, and auditable through
  the same operator-visible system truth as normal work.
- **FR-006**: Replay execution MUST preserve the original safety boundaries, approval
  expectations, and side-effect constraints unless the operator explicitly selects a more
  permissive validation scope.
- **FR-006a**: If a replay is allowed to exercise real side effects for validation, the
  system MUST make that scope explicit before launch and preserve it in replay provenance.
- **FR-006b**: The default replay mode MUST be non-live: it MUST NOT execute real side
  effects, and it MUST treat approval-gated steps as blocked or evidence-only unless the
  operator explicitly enables live validation.
- **FR-007**: Replay outcomes MUST remain operator-visible and durably linked to the
  source scenario, the replay attempt, and the evidence produced during replay execution.
- **FR-008**: The comparison surface MUST present baseline and replay outcomes together in
  a form that operators can inspect without manual log diffing.
- **FR-008a**: Phase 33 comparison detail MUST include terminal status plus runtime,
  policy, integration, delivery, and evidence summary differences where available.
- **FR-009**: Comparison results MUST distinguish runtime drift, policy drift,
  integration drift, and delivery drift where evidence allows, and MUST report unknown or
  mixed drift explicitly when a more precise classification is not defensible.
- **FR-010**: Comparison results MUST distinguish matched outcomes, observed drift,
  blocked replays, and unreplayable scenarios so operators are not forced to infer status
  from raw evidence fragments.
- **FR-011**: Evaluation for this phase MUST consume explicit run, workflow, policy,
  artifact, and delivery truth rather than prompt-only summaries or undocumented operator
  memory.
- **FR-012**: The system MUST support curated regression fixtures for representative
  schedule-driven work, integration-backed work, and computer-use paths.
- **FR-012a**: Phase 33 regression fixtures MUST be authored through engineer-owned,
  repo-managed definitions and captured evidence; in-product fixture creation or editing
  by operators is out of scope.
- **FR-013**: Regression fixtures MUST retain provenance to the representative scenario
  they cover, along with assumptions, expected boundaries, and any known replay
  limitations.
- **FR-014**: Supported regression fixtures MUST be runnable in repeatable regression
  flows without manual reconstruction of source inputs or expected evidence.
- **FR-015**: The feature MUST provide at least one documented manual before-and-after
  verification flow for a real or local-fixture scenario so operators can validate replay
  behavior outside automated regressions.
- **FR-016**: Replay, comparison, and regression outcomes MUST remain environment-scoped
  so evidence from one environment is not confused with another environment's behavior.
- **FR-017**: When source evidence is incomplete, expired, redacted beyond reuse, or no
  longer available, the system MUST preserve the scenario and explain the limitation
  instead of silently omitting it from evaluation surfaces.
- **FR-018**: Replay and comparison artifacts MUST remain operator-visible after restart
  so evaluation history can be audited over time.
- **FR-019**: The phase 33 scope MUST stay focused on deterministic replay and comparison
  where possible, and MUST NOT require automatic self-improvement loops, model-training
  infrastructure, or autonomous metric optimization to claim completion.
- **FR-020**: Operator-facing evaluation surfaces MUST make clear which baseline,
  comparison target, and change window a replay belongs to so repeated validation work
  remains traceable.

### Key Entities *(include if feature involves data)*

- **Replay Candidate**: A curated representative piece of completed personal-agent work or
  engineer-managed fixture selected for replay, including its source provenance, replay
  readiness, and declared limitations.
- **Replay Attempt**: A new evaluation run created from a replay candidate, linked to the
  source scenario, safety scope, environment, produced evidence, and terminal outcome.
- **Comparison Result**: The operator-visible summary of baseline and replay outcomes,
  including terminal status, runtime differences, policy differences, integration
  differences, delivery differences, evidence summary differences, confidence limits, and
  supporting evidence references.
- **Drift Finding**: A specific observed difference attributed to runtime, policy,
  integration, delivery, unknown, or mixed causes based on available evidence.
- **Regression Fixture**: An engineer-curated, repo-managed representative scenario for
  automated or manual replay validation, including provenance to the original work and any
  replay assumptions.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Additive compatibility-impacting surface changes are expected
  for replay candidates, replay attempts, comparison summaries, regression fixture
  metadata, and operator-visible evaluation history. Existing run, workflow, approval,
  artifact, and delivery truth should remain backward compatible.
- **Migration / Rollback**: Rollout should be additive by introducing replay and
  comparison records without replacing existing execution history. Rollback is a revert of
  new evaluation surfaces while preserving any already-recorded replay provenance and
  comparison evidence for audit where feasible.
- **Verification Strategy**: Required validation includes targeted automated regression
  coverage for replay readiness, replay execution, comparison classification, fixture
  curation, environment scoping, and restart persistence, plus one manual before-and-after
  verification flow in `KURA_ENV=test` using either a real or local-fixture scenario.
- **Observability Impact**: Operators need durable replay and comparison status, drift
  classification, readiness limitations, source-to-replay linkage, and restart-safe audit
  history surfaced through the web operator shell. Operator docs and evidence inspection
  guidance must be updated so evaluation outcomes are explainable without raw log
  reconstruction.
- **Environment & Secrets**: Default validation should run in the test environment.
  Replay must respect existing secret-redaction and approval boundaries, and live
  side-effect validation should occur only when the operator explicitly opts into that
  scope.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can select a representative scenario, launch a replay, and reach
  an operator-visible terminal status through the web operator shell in under 10 minutes
  without reconstructing the scenario from raw logs or source code.
- **SC-002**: At least 95% of replay-and-comparison validations against supported
  deterministic fixtures end in a clear classification of matched, drifted, blocked, or
  unreplayable rather than an ambiguous outcome.
- **SC-003**: The product supports at least one reusable representative regression fixture
  for each required domain class in phase 33: schedules, integrations, and computer use.
- **SC-004**: For the documented manual before-and-after flow, an operator can determine
  within 5 minutes of replay completion whether the observed difference is primarily
  runtime, policy, integration, delivery, or not a material drift.
- **SC-004a**: For 100% of supported comparison scenarios exercised during release
  verification, the web operator shell shows terminal status and at least one
  plane-specific or evidence-summary explanation for every observed material difference.
- **SC-005**: Replay and comparison records for completed evaluation work remain
  inspectable after restart for 100% of validation scenarios exercised during release
  verification.

## Assumptions

- Existing daemon-owned run, workflow, approval, artifact, and delivery truth remains the
  source of record for evaluation rather than a parallel replay-only ledger.
- The first phase 33 slice prioritizes deterministic or bounded scenarios and may mark
  some real-world flows as partially replayable or unreplayable when trustworthy replay is
  not possible.
- Default replay behavior is non-live and evidence-preserving; live side-effect validation
  is an explicit operator choice rather than the baseline mode.
- Fixture authoring remains engineer-owned in phase 33; operator-facing product work
  consumes fixture outcomes rather than providing fixture editing workflows.
- Replay candidate eligibility is intentionally curated in phase 33; broad automatic
  replay eligibility for all completed work can be considered after the evidence model
  proves stable.
- Operators primarily validate replay behavior in the test environment and use local
  fixtures when live external behavior is not required for acceptance.
- Knowledge-plane work, autonomous adaptation, and optimization loops remain outside the
  scope of this feature even if replay evidence later informs those areas.
