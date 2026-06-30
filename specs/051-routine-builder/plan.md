# Implementation Plan: Routine Builder

**Branch**: `main` | **Spec**: [spec.md](./spec.md) | **Upstream**: [docs/specs/051-routine-builder.md](../../docs/specs/051-routine-builder.md)
**Phase / Roadmap**: Phase 66 — Roadmap 66

## Summary
New `internal/routine` subsystem: explicit routine definitions (trigger + workflow + approval +
delivery) that compile to the existing scheduler workflow-target plane. Lifecycle (create/preview/
update/pause/resume/cancel/repair) with versioning that preserves prior execution evidence. No
autonomous planning, no memory, no hidden background execution.

## Technical Context
- Go 1.24 daemon. Compiles to `internal/scheduler` (Create/Pause/Resume/Cancel/Get) via a small
  Scheduler interface. In-memory routines with Restore (MVP); compiled schedules persist in the
  scheduler store (where the execution evidence lives).
- API + schemas additive. No scheduler/workflow shape changes.

## Constitution Check
- Roadmap closure: users build proactive routines from product surfaces while engineers inspect
  ordinary schedule/workflow truth.
- Production-grade: explicit validation, preview-before-activate, evidence-preserving edits,
  repair.
- Contracts first: routine resource + create request schemas; contract test.
- Verification: lifecycle/versioning/compilation/preview/repair unit tests.
- Environment: opt-in; test env default.

## Project Structure
```
specs/051-routine-builder/  spec.md plan.md tasks.md checklists/
daemon/internal/routine/types.go,manager.go,manager_test.go
daemon/internal/api/routine.go ; server.go + app.go wiring
schemas/api/routine-resource.schema.json, create-routine.request.schema.json
daemon/internal/contracts/routine_contracts_test.go
```

## Complexity Tracking
No violations. Additive subsystem compiling to existing planes; outcomes run through the
scheduler/workflow planes (already replay-eligible).
