# Contract: Status Vocabulary

## Goal

Use one vocabulary for roadmap closure and release evidence so implementation state is
not confused with local verification, stable-host dry-run evidence, full hosted soak, or
real-account smoke.

## Canonical Labels

Allowed labels:

- `proposed`: scope is planned but implementation has not started.
- `implementation complete`: required implementation work is complete, but release
  evidence may still be pending.
- `local verification complete`: targeted local checks, contract checks, or quickstart
  verification passed for the implemented scope.
- `stable-host dry-run complete`: a stable always-on host or VPS ran a dry-run or
  controlled-host smoke; this is not a full hosted release soak.
- `full hosted soak pending`: full-duration hosted daemon soak evidence is still
  required before public readiness.
- `real-account smoke pending`: safe real-account smoke evidence is still required, or
  credentials/approvals must be explicitly skipped or blocked.
- `release evidence gap`: remaining release evidence work that does not imply missing
  implementation unless `implementation_missing` is also recorded.
- `public readiness`: release claim allowed only when current linked release evidence
  satisfies the standalone release-truth checklist.

## Required Roadmap 42 Classification

Roadmap 42 must be classified as:

- implementation complete,
- local verification complete,
- stable-host or real-account release evidence pending unless stronger evidence is
  linked,
- not publicly ready unless the standalone checklist passes.

## Required Roadmap 43 Classification

Roadmap 43 must be classified as:

- implementation complete,
- local verification complete,
- stable-host dry-run complete where linked evidence exists,
- full-duration hosted daemon release soak pending,
- not publicly ready unless the standalone checklist passes.

## Prohibited Wording

Do not use wording that:

- treats stable-host dry-run evidence as a full hosted release soak,
- treats missing real-account credentials as missing implementation,
- claims public readiness without linked release evidence,
- rewrites historical evidence instead of linking and classifying it,
- uses `done`, `pending`, or checkbox-only labels without evidence-state wording.

## Verification

Required checks:

- Search roadmap, harness, upstream spec, and branch-local docs for Roadmap 42 and
  Roadmap 43 status contradictions.
- Confirm every public-readiness phrase is paired with linked release evidence or
  downgraded to residual release work.
- Confirm Roadmap 44 remains the reconciliation slice before Roadmap 45 starts.
