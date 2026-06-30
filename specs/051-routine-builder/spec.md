# Feature Specification: Routine Builder

**Feature Branch**: `main`
**Created**: 2026-06-30
**Status**: Draft
**Phase / Roadmap**: Phase 66 — Roadmap 66
**Upstream authority**: [docs/specs/051-routine-builder.md](../../docs/specs/051-routine-builder.md)

## Overview

Add a product routine builder: explicit, user-defined proactive routines composed from a
trigger, a workflow, an approval expectation, and a delivery preference. Routines compile to
the existing schedule + workflow + delivery planes — no new background execution and no
autonomous planning or memory. Users can create, preview, inspect, pause, resume, cancel, and
repair routines; edits create new versions and never rewrite prior execution evidence.

## User Scenarios & Testing *(mandatory)*

### US1 - Create a routine (P1)
1. A user composes an explicit routine (trigger + workflow + approval + delivery) and previews
   it before activation; the preview shows the compiled schedule kind and approval/delivery/
   retry expectations.
2. Activating compiles the routine to a schedule with a workflow target; the routine is active.

### US2 - Pause/edit safely (P2)
1. A user pauses a routine; the underlying schedule is paused and the routine state reflects it.
2. A user edits a routine; a new version is recorded, a new schedule compiled, the prior
   schedule cancelled, and the prior version's evidence (schedule id + attempts) preserved.
3. A cancelled routine rejects further lifecycle transitions.

### US3 - Inspect/repair (P3)
1. An operator inspects routine versions and the compiled schedule for execution evidence.
2. If the compiled schedule has gone missing, repair recreates it without bumping the version.

### Edge Cases
- Invalid definition (missing name/goal/trigger detail) is rejected before any schedule is made.
- Preview never activates a schedule.
- Editing a paused routine recompiles paused.

## Requirements *(mandatory)*

### Functional Requirements
- **FR-001**: The system MUST persist routine definitions and versions.
- **FR-002**: Routines MUST compile to existing schedule + workflow targets.
- **FR-003**: Routine edits MUST NOT rewrite prior execution evidence (prior versions keep their
  compiled schedule id; prior schedule attempts remain).
- **FR-004**: Approval, delivery, and quota/retry expectations MUST be previewable before
  activation.
- **FR-005**: Routine execution MUST be replay/evaluation eligible (it runs through the existing
  schedule/workflow planes, which are already replay-eligible).
- **FR-006**: No autonomous planning, memory, or hidden background execution.

### Key Entities
- Routine (definition + version history + state + compiled schedule), Routine Version (snapshot +
  schedule id), Trigger, Workflow, Preview.

## Compatibility & Operational Impact *(mandatory)*
- **Compatibility**: Additive new subsystem compiling to the existing scheduler/workflow planes;
  no schedule/workflow shape changes.
- **Migration / Rollback**: No migration; routines are opt-in. Rollback = cancel routines (their
  schedules cancel).
- **Verification**: lifecycle + versioning + compilation + preview + repair tests; existing
  scheduler suite unaffected.
- **Observability**: routine state + versions; execution evidence is the ordinary schedule
  attempt history.

## Success Criteria *(mandatory)*
- **SC-001**: Create compiles to a workflow schedule; preview reports expectations without
  activating.
- **SC-002**: Edit creates a new version, recompiles, and preserves prior version evidence.
- **SC-003**: Pause/resume/cancel drive the compiled schedule + routine state; cancel is terminal.
- **SC-004**: Repair recreates a missing schedule without bumping the version.

## Assumptions
- Persistence is in-memory with Restore for this slice; full SQLite persistence is a follow-on.
  Natural-language prefill (if any) only sets fields on a structured, user-approved definition.
  Webhook triggers are Roadmap 67.
