# Tasks: Routine Builder

**Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md) | **Roadmap**: 66

- [X] T001 [Setup] Confirm scheduler Create/Pause/Resume/Cancel/Get + Target/Trigger shapes.
- [X] T002 [Foundational] routine types: Trigger/Workflow/Definition/Version/Routine/Preview + states.
- [X] T003 [Foundational] manager: Scheduler interface; compile(def)->scheduler.CreateInput (workflow target).
- [X] T004 [US1] Create (validate+compile+store v1); Preview (dry-run, no activation).
- [X] T005 [US2] Update (new version, recompile, cancel prior, preserve prior evidence); Pause/Resume/Cancel.
- [X] T006 [US3] Repair (recreate missing schedule, no version bump); Get/List.
- [X] T007 [P] tests: create/compile, update-preserves-evidence, lifecycle, repair, preview/validation.
- [X] T008 [API] app + server wiring; /v1/routines CRUD + preview/pause/resume/cancel/repair.
- [X] T009 [Polish] schemas (routine resource + create request) + contract test; verify build/vet/test.

## Notes
Persistence in-memory with Restore for this slice (compiled schedules + attempts persist in the
scheduler store, which holds the execution evidence). Webhook triggers are Roadmap 67.
