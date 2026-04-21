# Docs Specs Index

## Purpose

`docs/specs/` stores upstream product specs for roadmap slices that should be expanded into
full speckit feature specs later.

These documents are intentionally more detailed than a roadmap section and more stable than
an in-progress feature branch spec. They exist so `/speckit-specify` can start from a
single authoritative source instead of reconstructing scope from scattered roadmap notes.

## Usage Rule

When a future roadmap slice already has a document in `docs/specs/`, prefer using that
document directly as the main input to `/speckit-specify`.

Recommended pattern:

```text
$speckit-specify 结合 docs/specs/<NNN>-<name>.md 完成 phase <roadmap number> 的工作
```

Do not rely on the short roadmap heading alone when a `docs/specs` document exists.

## Numbering

- `docs/specs/` numbering is sequential and independent from speckit feature branch or
  `specs/` directory numbering.
- These numbers identify upstream product-spec documents only.

## Current Mapping

- `007` -> Roadmap 22: MCP Catalog Management And Distribution
- `008` -> Roadmap 23: Additional MCP Transports
- `009` -> Roadmap 24: Tool-Call Orchestration
- `010` -> Roadmap 25: Scheduled Tasks And Wakeups
- `011` -> Roadmap 26: Use-Computer Capability Plane
- `012` -> Roadmap 27: Personal Integrations Platform
- `013` -> Roadmap 28: Delivery And Notifications
- `014` -> Roadmap 29: Calendar Integration
- `015` -> Roadmap 30: Mail Integration
- `016` -> Roadmap 31: Tasks And Reminders
- `017` -> Roadmap 32: Operator Shell And Onboarding
- `018` -> Roadmap 33: Evaluation And Replay Harness

## Authoring Standard

Each `docs/specs` document should include:

- roadmap or architecture context
- goal
- in-scope and out-of-scope boundaries
- fixed decisions that should not be reopened by default
- dependencies on already completed phases
- user or operator stories
- functional requirements
- compatibility and operational notes
- verification expectations
- definition of done
- a recommended `/speckit-specify` prompt

## Relationship To Other Planning Artifacts

- `docs/runtime/daemon-roadmaps.md` defines roadmap order and closure criteria.
- `docs/harness/harness-architecture.md` defines architectural sequencing and rationale.
- `docs/specs/` captures the authoritative upstream spec for a future slice before a
  branch-local speckit feature is created.
- `specs/<NNN>-.../` remains the branch-local speckit working area once implementation
  planning begins.
