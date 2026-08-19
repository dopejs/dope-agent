# Inbox Triage MVP Without Memory

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 65, the first
inbox triage workflow slice without memory or knowledge-plane dependency.

Primary source documents:
- `docs/specs/015-mail-integration.md`
- `docs/specs/048-real-mail-provider-closure.md`
- `docs/specs/016-tasks-and-reminders.md`
- `docs/specs/013-delivery-and-notifications.md`

## Background

Hermes-style parity includes handling message overload. Kura should first support
explicit-rule and operator-configured inbox triage before memory-driven personalization or
learned prioritization exists.

## Goal

Add a transparent inbox triage workflow that classifies mail using explicit rules,
operator-approved actions, delivery, and reminder/task linkage.

## Fixed Decisions

- Triage decisions are rule/config-driven in this roadmap.
- No learned preferences, memory recall, semantic user model, or knowledge plane.
- Destructive or externally visible actions require explicit permission.
- Triage evidence must be replayable.

## Dependencies On Completed Phases

- Roadmap 63: Real Mail Provider Closure
- Roadmap 31: Tasks And Reminders
- Roadmap 28: Delivery And Notifications

## In Scope

- triage policy resource with explicit rules
- unread or selected-mail triage run
- classifications such as urgent, needs reply, FYI, newsletter, blocked, unsupported
- draft reply, reminder, delivery digest, or no-action outcomes
- audit and replay candidate evidence

## Out Of Scope

- memory-based prioritization
- automatic relationship inference
- campaign automation
- silent auto-send replies

## Operator Or User Problems To Solve

- Users need help turning inbox items into visible actions without trusting hidden memory.
- Operators need to inspect why a message was classified a certain way.

## User Stories

- As a user, I can define triage rules and run them on selected inbox items.
- As a user, I can receive a digest of urgent or needs-reply messages.
- As an operator, I can replay a triage decision.

## Functional Requirements

- The system MUST persist triage policy, run, item, classification, and action records.
- Triage classifications MUST link to source mail message IDs and explicit rule IDs.
- Externally visible actions MUST require approval or configured permission.
- Triage results MUST support replay/evaluation fixtures.

## Compatibility And Operational Notes

This roadmap should reuse workflow, mail, reminder, delivery, and evaluation surfaces
rather than creating a parallel triage executor.

## Verification Expectations

- Rule engine tests for deterministic classifications.
- Workflow tests for draft, reminder, digest, and no-action outcomes.
- Replay fixture tests for triage decisions.
- Web tests for policy creation and run inspection.

## Definition Of Done

- Inbox triage provides useful non-memory automation with inspectable decisions and safe
  action boundaries.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/050-inbox-triage-mvp-without-memory.md 完成 phase 65 的工作`
