# Contract: Release-Truth Checklist

## Goal

Provide a standalone reusable checklist that release reviewers can apply to Roadmap 44
and later roadmap closures before public-readiness claims are accepted.

## Required Location

The checklist must be implemented as:

```text
docs/runtime/release-truth-checklist.md
```

It must be linked from roadmap and spec materials that discuss Roadmap 44 release-truth
reconciliation or future public-readiness decisions.

## Required Sections

The checklist must include:

- scope and when to use the checklist,
- status vocabulary reference,
- evidence link review,
- implementation and local verification classification,
- stable-host dry-run classification,
- full hosted soak classification,
- real-account smoke classification,
- residual blocker classes,
- public-readiness eligibility,
- no-ship conditions,
- verification commands and manual review steps,
- reviewer outcome: `pass`, `residual_work`, or `no_ship`.

## No-Ship Conditions

The checklist must classify a release claim as `no_ship` when:

- public readiness is claimed without linked release evidence,
- required evidence is missing, stale, mismatched, or secret-exposing,
- stable-host dry-run evidence is presented as full hosted soak evidence,
- real-account smoke is absent without explicit skip, blocked, or pending reason,
- implementation status and release evidence status contradict each other,
- the reviewed evidence cannot be reached from roadmap/spec materials.

## Residual-Work Conditions

The checklist must classify a roadmap as `residual_work` when implementation is complete
but required public-readiness evidence remains pending, skipped, blocked, stale, or
deferred.

## Pass Conditions

The checklist may classify a roadmap as `pass` only when:

- implementation and local verification status are clear,
- all required evidence links are current and reachable,
- residual gaps are either closed or not required for the reviewed readiness claim,
- no no-ship condition applies.

## Verification

Required checks:

- Apply the checklist to Roadmap 42 and confirm stable-host or real-account release
  evidence remains distinct from implementation/local verification completion.
- Apply the checklist to Roadmap 43 and confirm stable-host dry-run evidence remains
  distinct from the pending full-duration hosted daemon release soak.
- Confirm the checklist is linked from roadmap/spec materials.
- Confirm reviewers can reach a pass, residual-work, or no-ship classification without
  chat history.
