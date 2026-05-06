# Data Model: Roadmap Authority And Release Truth Reconciliation

This feature models documentation concepts rather than persisted daemon records. The
entities below define what must be represented consistently in roadmap, harness, upstream
spec, branch-local, and release-readiness materials.

## Roadmap Closure Record

Represents the current closure state for one roadmap.

Fields:

- `roadmapNumber`: numeric roadmap identifier.
- `title`: canonical roadmap title.
- `statusLabel`: one value from Status Vocabulary.
- `implementationState`: `proposed`, `not_started`, `in_progress`, `complete`, or
  `not_applicable`.
- `localVerificationState`: `not_started`, `complete`, `blocked`, `not_applicable`, or
  `unknown`.
- `stableHostState`: `not_started`, `dry_run_complete`, `full_run_complete`, `pending`,
  or `not_applicable`.
- `hostedSoakState`: `not_started`, `pending`, `complete`, `not_required`, or
  `not_applicable`.
- `realAccountSmokeState`: `not_started`, `pending`, `complete`, `blocked`, `skipped`,
  or `not_applicable`.
- `evidenceLinks`: one or more Release Evidence Links when evidence exists.
- `residualGaps`: zero or more Residual Evidence Gaps.
- `publicReadiness`: `eligible`, `not_eligible`, or `not_claimed`.

Validation rules:

- Public readiness is `eligible` only when required release evidence links exist and the
  standalone release-truth checklist passes.
- Roadmap 42 must record implementation and local verification complete with remaining
  stable-host or real-account release evidence pending unless implementation discovers
  stronger evidence.
- Roadmap 43 must record local implementation evidence and stable-host dry-run evidence
  separately from the pending full-duration hosted daemon release soak.

## Status Vocabulary

Represents allowed wording for roadmap and release evidence states.

Fields:

- `label`: canonical status phrase.
- `meaning`: human-readable interpretation.
- `allowedUse`: documents or contexts where the label may appear.
- `publicReadinessAllowed`: boolean.
- `requiredEvidence`: evidence links needed before the label may be used.

Validation rules:

- Binary complete/pending markers may be retained only when accompanied by precise
  evidence wording.
- No status may imply public readiness without linked release evidence.

## Release Evidence Link

Represents a reference from planning materials to evidence.

Fields:

- `sourceDocument`: document that contains the link.
- `targetArtifact`: quickstart, runbook, release evidence index, smoke report, soak
  report, or upstream spec.
- `roadmapNumber`: roadmap the evidence supports.
- `evidenceKind`: `quickstart`, `runbook`, `local_verification`, `stable_host_dry_run`,
  `full_hosted_soak`, `real_account_smoke`, `release_index`, or `skip_reason`.
- `freshnessScope`: commit, profile, run identity, or documented historical evidence.
- `limitations`: known constraints such as dry-run only, skipped credentials, or pending
  full soak.

Validation rules:

- Links must not point to historical evidence as if it were current public-readiness
  evidence unless the checklist accepts that scope.
- Missing safe credentials must be represented as explicit skip or blocked evidence, not
  as absent evidence.

## Residual Evidence Gap

Represents remaining release work that does not necessarily imply missing implementation.

Fields:

- `gapId`: stable short identifier for the residual gap.
- `roadmapNumber`: related roadmap.
- `blockerClass`: `implementation_missing`, `verification_missing`,
  `stable_host_dry_run_pending`, `hosted_soak_pending`,
  `real_account_credentials_unavailable`, `tenant_approval_unavailable`,
  `operator_deferred`, or `evidence_stale`.
- `description`: reviewer-facing explanation.
- `requiredForPublicReadiness`: boolean.
- `ownerRole`: release owner, engineer, operator, tenant administrator, or provider.

Validation rules:

- Stable-host dry-run pending, hosted soak pending, and real-account credentials
  unavailable are release evidence gaps unless implementation is explicitly missing.
- Required public-readiness gaps must produce residual-work or no-ship classification.

## Release-Truth Checklist

Represents the standalone reusable reviewer artifact.

Fields:

- `location`: `docs/runtime/release-truth-checklist.md`.
- `linkedFrom`: roadmap, release-readiness, harness, upstream spec, or branch-local
  planning documents.
- `sections`: status vocabulary, evidence links, residual gaps, no-ship rules,
  public-readiness eligibility, and verification notes.
- `classificationOutcome`: `pass`, `residual_work`, or `no_ship`.

Validation rules:

- The checklist must be reachable from roadmap and spec materials.
- Applying the checklist to Roadmaps 42 and 43 must identify their remaining release
  evidence gaps without reopening completed implementation scope.

## Planning Boundary

Represents the standard scope-control rule for future specs.

Fields:

- `taskLimit`: target fewer than 50 tasks for standard branch-local specs.
- `splitRule`: split oversized upstream specs before implementation planning.
- `appliesTo`: non-knowledge parity roadmap family after Roadmap 44.

Validation rules:

- Future planning guidance must not encourage oversized standard specs.
- Splitting scope must happen before implementation planning, not after tasks reveal an
  oversized roadmap.
