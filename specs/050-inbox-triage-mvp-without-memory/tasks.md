# Tasks: Inbox Triage MVP Without Memory

**Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md) | **Roadmap**: 65

Stories: US1 rules+run, US2 proposed outcomes, US3 audit+replay.

## Phase 1: Setup
- [X] T001 [Setup] New internal/triage package scaffolding; confirm mail message snapshot fields.

## Phase 2: Foundational (engine)
- [X] T002 [Foundational] types.go: Classification/Outcome (fixed sets), Condition/Rule/Policy,
  Message, Decision (matched rule + evidence + replayCandidate), Run.
- [X] T003 [Foundational] manager.go: CreatePolicy (validate), Get/List, Run (first-match,
  default fyi/no_action), matcher (contains/equals/not_contains, AND), determinism.

## Phase 3: US1/US2/US3 — classify, propose, audit
- [X] T004 [US1] rule matching for all classifications + default; first-match-wins.
- [X] T005 [US2] outcomes are proposals (draft_reply/reminder/delivery_digest/no_action); no
  silent auto-send (triage returns proposals only).
- [X] T006 [US3] every decision is a replay candidate; deterministic re-run; default flagged.
- [X] T007 [P] tests: classify+default+first-match; determinism; validation; proposals.

## Phase 4: Wiring + API
- [X] T008 [API] app.go: construct triage manager + register; api.Dependencies/Server field.
- [X] T009 [API] api/triage.go: policy create/list/get + run endpoints; routes registered.

## Phase 5: Contracts + polish
- [X] T010 [Polish] schemas: triage policy/run resources + create request; contract test.
- [X] T011 [Polish] verify: build/vet/test triage, api, contracts; no memory inputs.

## Notes
- Persistence is in-memory with Restore for this MVP (policies reload via Restore); full SQLite
  persistence + tenancy accessors are a documented follow-on. Triage outcomes are proposals;
  reminder/delivery execution reuses the existing gated managers in a later slice.

## Dependencies
T002 blocks all. T003 before T004-T007. T008/T009 after engine. T010 after API.
</content>
