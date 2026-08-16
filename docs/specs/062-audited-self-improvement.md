# 062 — Audited Self-Improvement

## Roadmap Context

Roadmap 82 (context/knowledge/memory program, final design slice). Depends
on 058-061; every mechanism here composes the planes those slices built.

## Goal

A closed improvement loop the operator can audit and veto: the system
observes its own outcomes (evaluation plane), proposes bounded changes
(memory write-policy tuning, context budget adjustments, skill revisions
via 061), and applies them only through the approval plane with recorded
before/after evidence.

## In Scope

- An improvement-proposal resource typed by target (memory-policy,
  context-budget, skill, routine): each carries the motivating evaluation
  evidence, the concrete diff, the predicted effect, and a rollback plan.
- Evaluation-plane integration (Roadmaps 33/41): proposals reference
  campaign/fixture results; applied changes schedule a follow-up
  evaluation whose result is attached to the proposal record.
- Approval-plane application with automatic rollback when the follow-up
  evaluation regresses beyond a recorded threshold.
- Full audit chain: proposal → decision → application → follow-up →
  keep/rollback, all as events and inspectable resources.

## Out Of Scope

- Any self-directed behavior outside the proposal/approval loop (product
  non-goal); model/prompt fine-tuning; changes to the trust boundary
  itself.

## Fixed Decisions

- No change applies without a recorded rollback path.
- Improvement proposals are rate-bounded per target per window; the bound
  is operator configuration, not agent-adjustable.

## Verification / Definition Of Done

- Behavioral tests for the full loop including regression-triggered
  rollback; contract tests; a quickstart demonstrating one audited
  improvement cycle end-to-end.
