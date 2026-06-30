# Feature Specification: Inbox Triage MVP Without Memory

**Feature Branch**: `main`
**Created**: 2026-06-30
**Status**: Draft
**Phase / Roadmap**: Phase 65 — Roadmap 65
**Upstream authority**: [docs/specs/050-inbox-triage-mvp-without-memory.md](../../docs/specs/050-inbox-triage-mvp-without-memory.md)

## Overview

Add a transparent inbox triage workflow that classifies mail using **explicit, operator-defined
rules** and proposes visible actions — with no memory, no learned preferences, no semantic user
model, and no silent auto-send. A triage policy holds ordered rules; a triage run evaluates a set
of (unread or explicitly selected) messages against the rules and records, per message, a
classification (urgent / needs_reply / fyi / newsletter / blocked / unsupported), the matched
rule, the matched evidence, and a proposed outcome (draft reply / reminder / delivery digest /
no-action). Destructive or externally-visible outcomes require explicit permission (reusing the
existing approval/live-validation gates); triage itself only classifies and proposes. Every
triage decision is auditable and a replay candidate.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Define triage rules and run them (Priority: P1)
**Acceptance Scenarios**:
1. An operator defines a triage policy with ordered rules (match conditions -> classification +
   proposed action) and runs it on selected inbox items.
2. Each message receives exactly one classification from the first matching rule; unmatched
   messages get a default classification (fyi) and no-action.
3. The decision records the matched rule id and the matched evidence (which condition matched),
   so an operator can inspect why a message was classified a certain way.

### User Story 2 - Proposed outcomes, not silent actions (Priority: P2)
**Acceptance Scenarios**:
1. A rule classifying needs_reply proposes a draft-reply outcome (a proposal, not a sent reply).
2. A rule classifying urgent proposes a reminder outcome (proposal) linked to the message.
3. A rule classifying newsletter proposes a delivery-digest outcome (proposal).
4. No outcome performs a destructive or externally-visible side effect without explicit
   permission; triage never silently auto-sends.

### User Story 3 - Audit and replay (Priority: P3)
**Acceptance Scenarios**:
1. A triage run records an auditable decision per message (classification, rule, evidence,
   proposed outcome, timestamp).
2. Each decision is a replay candidate so the run can be re-evaluated deterministically.
3. An "unsupported" classification is recorded explicitly when no rule applies and the default
   cannot classify (rather than a silent drop).

### Edge Cases
- Empty policy: all messages classified default (fyi), no-action.
- Conflicting rules: first match wins (deterministic ordering).
- Blocked sender rule: classification blocked, no-action proposed (no reply).
- Message missing fields: rule conditions that reference missing fields do not match.

## Requirements *(mandatory)*

### Functional Requirements
- **FR-001**: A triage policy resource MUST hold ordered, explicit rules (match conditions ->
  classification + proposed action); rules are operator-defined, not learned.
- **FR-002**: A triage run MUST evaluate a set of selected/unread messages and produce exactly
  one decision per message (first matching rule; default fyi/no-action when none match).
- **FR-003**: Classifications MUST be from the fixed set: urgent, needs_reply, fyi, newsletter,
  blocked, unsupported.
- **FR-004**: Each decision MUST record the matched rule id and matched evidence so the
  classification is inspectable/transparent.
- **FR-005**: Outcomes (draft_reply, reminder, delivery_digest, no_action) MUST be proposals;
  destructive/externally-visible outcomes MUST require explicit permission; no silent auto-send.
- **FR-006**: Triage decisions MUST be auditable and replay candidates (deterministic re-run).
- **FR-007**: The workflow MUST NOT use memory, learned preferences, a semantic user model, or a
  knowledge plane.
- **FR-008**: No credential/secret material MUST be exposed in any triage decision/audit record.

### Key Entities
- Triage Policy (ordered rules), Triage Rule (conditions + classification + proposed action),
  Triage Run (over a message set), Triage Decision (classification + matched rule + evidence +
  proposed outcome, replayable).

## Compatibility & Operational Impact *(mandatory)*
- **Compatibility**: Additive new triage subsystem; consumes existing mail message snapshots and
  proposes reminder/delivery outcomes via existing managers; no changes to mail/reminder/delivery
  shapes.
- **Migration / Rollback**: No migration; triage is opt-in (no policy = no triage). Rollback =
  remove/disable policies.
- **Verification**: rule-matching unit tests (each classification + default + first-match),
  proposed-outcome tests, replay determinism test, no-silent-send test.
- **Observability**: triage run + decision events; decisions inspectable; no new memory plane.

## Success Criteria *(mandatory)*
- **SC-001**: A run produces exactly one transparent decision per message with matched rule +
  evidence.
- **SC-002**: Every classification comes from the fixed set; default applies when no rule matches.
- **SC-003**: Outcomes are proposals; zero silent auto-sends.
- **SC-004**: Re-running the same policy on the same messages yields identical decisions
  (deterministic / replayable).
- **SC-005**: No memory/learned-preference inputs influence classification.

## Assumptions
- Rules match on message fields available in the mail message snapshot (sender, subject,
  newsletter markers, recipients). Reminder/delivery outcomes are proposals in this MVP; their
  execution reuses existing gated managers. Fake mail backend is the dev/test input source.
