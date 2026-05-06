# Implementation Plan: Roadmap Authority And Release Truth Reconciliation

**Branch**: `029-roadmap-release-truth` | **Date**: 2026-05-06 | **Spec**: [`spec.md`](./spec.md)
**Input**: Feature specification from `specs/029-roadmap-release-truth/spec.md`

## Summary

Close Roadmap 44 by reconciling planning truth across roadmap summaries, upstream specs,
branch-local speckit artifacts, quickstarts, harness guidance, and release-readiness
notes. The implementation is documentation-only: define a consistent status vocabulary,
classify Roadmap 42 and Roadmap 43 evidence states, add a standalone reusable
release-truth checklist, link implemented roadmap evidence after Roadmap 39, and make
remaining stable-host, full hosted soak, and real-account smoke gaps explicit without
reopening completed implementation scope.

## Technical Context

**Language/Version**: Markdown planning and operator documentation only. No Go,
TypeScript, schema, or runtime behavior changes are planned.  
**Primary Dependencies**: `docs/runtime/daemon-roadmaps.md`,
`docs/harness/harness-architecture.md`, `docs/specs/README.md`,
`docs/runtime/release-readiness.md`,
`docs/specs/029-roadmap-authority-and-release-truth-reconciliation.md`,
`specs/027-integration-diagnostics/quickstart.md`, and
`specs/028-hosted-operational-profile/quickstart.md`.  
**Storage**: Repository documentation files. No daemon storage, persistence, config, or
generated runtime artifact storage changes.  
**Testing**: Text searches for stale or contradictory Roadmap 42/43 status wording,
manual link review from roadmap summaries to quickstarts/runbooks/evidence, checklist
application to Roadmaps 42 and 43, and Markdown review. Run code tests only if a script,
validator, schema, or generated artifact changes during implementation.  
**Target Platform**: Repository documentation consumed by release owners, engineers,
planners, and reviewers. Default validation is local repository inspection; no production
environment, live connector, or credential access is required.  
**Project Type**: Documentation and release-governance reconciliation across roadmap,
harness, upstream spec, and branch-local speckit artifacts.  
**Performance Goals**: Release owners can determine Roadmap 42 and 43 implementation
state, local verification state, and remaining release-evidence gaps in <=10 minutes.
Release reviewers can apply the standalone release-truth checklist to Roadmaps 42 and 43
in <=30 minutes without chat history.  
**Constraints**: Do not change runtime behavior, API/schema/event/config/storage
surfaces, provider behavior, connector behavior, context engineering, memory, or
knowledge-plane design. Do not invent new release evidence or rewrite historical evidence
records; classify and link existing evidence only. Standard branch-local specs in this
program should remain below 50 tasks or be split before implementation planning.  
**Scale/Scope**: Roadmap 44 plus the evidence states for Roadmaps 42 and 43, implemented
roadmaps after Roadmap 39 that are referenced by this reconciliation, and future planning
guidance for Roadmap 45 onward.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Roadmap closure** - PASS. The plan closes one whole roadmap: Roadmap 44 planning
  authority and release-truth reconciliation. It does not attempt to implement Roadmap 45
  product scope or public release-gate work.
- **Production-grade, minimal, reversible change** - PASS. The change is additive and
  reversible documentation work. Rollback reverts the updated docs and standalone
  checklist while preserving historical evidence artifacts.
- **Contracts and auditability** - PASS. Contracts define status vocabulary,
  roadmap-evidence link requirements, and release-truth checklist criteria. No API,
  schema, event, config, persistence, or execution boundary changes are planned.
- **Verification and observability** - PASS. The plan names text-search checks, manual
  link review, checklist application, and residual evidence-gap visibility. Runtime
  observability is unchanged; planning observability improves through explicit labels.
- **Environment and secrets** - PASS. Work uses repository-local documentation only.
  Production data, live connectors, real-account credentials, and privileged access are
  not required and must not be introduced.

No violations require justification. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/029-roadmap-release-truth/
|-- plan.md
|-- research.md
|-- data-model.md
|-- quickstart.md
|-- contracts/
|   |-- status-vocabulary.md
|   |-- roadmap-evidence-linkage.md
|   `-- release-truth-checklist.md
|-- checklists/
|   `-- requirements.md
`-- tasks.md                # /speckit.tasks output, not created by /speckit.plan
```

### Source Code (repository root)

```text
docs/
|-- runtime/
|   |-- daemon-roadmaps.md              # roadmap status and evidence links
|   |-- release-readiness.md            # release gate linkage
|   `-- release-truth-checklist.md      # standalone reusable checklist
|-- harness/
|   `-- harness-architecture.md         # sequencing and evidence-state wording
`-- specs/
    |-- README.md                       # upstream spec mapping and planning rule
    |-- 027-integration-health-and-permission-diagnostics.md
    |-- 028-hosted-operational-profile-and-recovery.md
    `-- 029-roadmap-authority-and-release-truth-reconciliation.md

specs/
|-- 027-integration-diagnostics/
|   `-- quickstart.md                   # Roadmap 42 evidence source
|-- 028-hosted-operational-profile/
|   `-- quickstart.md                   # Roadmap 43 evidence source
`-- 029-roadmap-release-truth/
    `-- ...                             # branch-local planning artifacts
```

**Structure Decision**: Keep Roadmap 44 as a documentation and release-governance
reconciliation. Add the reusable release-truth checklist under `docs/runtime/` next to
the existing release readiness gate, and link it from roadmap, harness, upstream spec,
and branch-local planning materials. Do not add daemon packages, scripts, schemas, API
routes, SDK methods, web views, or TUI surfaces unless planning discovers an existing
validator or generated artifact is broken and required for documentation verification.

## Roadmap 44 Planning Contracts

The implementation plan MUST keep these artifacts complete before `/speckit.tasks`:

- [`contracts/status-vocabulary.md`](./contracts/status-vocabulary.md) - canonical
  status labels, allowed meanings, and public-readiness restrictions.
- [`contracts/roadmap-evidence-linkage.md`](./contracts/roadmap-evidence-linkage.md) -
  required links from roadmap, harness, upstream spec, and branch-local artifacts to
  evidence sources and residual gap labels.
- [`contracts/release-truth-checklist.md`](./contracts/release-truth-checklist.md) -
  standalone checklist placement, sections, classification outcomes, no-ship rules, and
  reviewer application requirements.

These artifacts are planning gates. Implementation is incomplete if a roadmap can claim
public readiness without linked evidence, if Roadmap 42 local completion is confused with
real-account or stable-host release evidence, if Roadmap 43 stable-host dry-run evidence
is confused with a full-duration hosted daemon release soak, or if the standalone
checklist is not reachable from roadmap/spec materials.

## Migration And Rollback Plan

1. Add status vocabulary and release-truth checklist contracts in branch-local planning
   artifacts.
2. Add or update the standalone `docs/runtime/release-truth-checklist.md`.
3. Reconcile Roadmap 42, 43, and 44 status wording in roadmap, harness, upstream spec,
   and branch-local quickstart references.
4. Link evidence sources without moving or rewriting historical evidence artifacts.
5. Update future planning guidance so standard branch-local specs remain below 50 tasks
   or are split before planning.
6. Verify with targeted `rg` checks and manual link/checklist review.

Rollback reverts the documentation edits and removes the standalone checklist if needed.
Historical quickstarts, runbooks, evidence indexes, and release evidence remain untouched.
No daemon data, production state, secrets, or runtime behavior are affected.

## Post-Design Constitution Check

- **Roadmap closure** - PASS. `research.md`, `data-model.md`, `quickstart.md`, and the
  three contracts cover the full Roadmap 44 documentation reconciliation and explicitly
  defer product behavior, public launch-gate execution, live connector work, context,
  memory, and knowledge-plane design.
- **Production-grade, minimal, reversible change** - PASS. Design is limited to
  auditable docs and planning artifacts. Rollback is a documentation revert.
- **Contracts and auditability** - PASS. Contracts define status labels, link
  requirements, residual blocker classes, checklist placement, and no-ship rules. No
  compatibility-impacting runtime contracts change.
- **Verification and observability** - PASS. Quickstart requires text checks for
  contradictions, manual link review, checklist application to Roadmaps 42 and 43, and
  explicit recording of any unverified release evidence gaps.
- **Environment and secrets** - PASS. Design requires no production state, live
  connectors, credentials, or privileged access. Real-account smoke references must stay
  at the evidence/skip-classification level and never expose secret material.

No post-design violations require justification.

## Complexity Tracking

> Filled only when Constitution Check has unjustified violations. None for this plan.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| (none)    |            |                                     |
