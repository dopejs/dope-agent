# Docs Specs Index

> **Retired flow (2026-08-17, operator decision):** the numbered-spec
> authoring flow ends here. Specs 001-062 are frozen historical record; new
> features are planned directly in design docs (e.g.
> [`docs/harness/plugin-architecture.md`](../harness/plugin-architecture.md))
> and implemented.

## Purpose

`docs/specs/` stores upstream product specs for roadmap slices, authored directly as
plain markdown (Spec Kit tooling was removed; the documents remain authoritative).

These documents are intentionally more detailed than a roadmap section and more stable than
an in-progress feature-branch spec. They are the authoritative scope for their roadmap
slice, so implementation planning starts from one source instead of reconstructing scope
from scattered roadmap notes.

## Usage Rule

When a roadmap slice already has a document in `docs/specs/`, use that document directly
as the authoritative scope, alongside
[`docs/runtime/daemon-roadmaps.md`](../runtime/daemon-roadmaps.md). Do not rely on the
short roadmap heading alone when a `docs/specs` document exists.

## Numbering

- `docs/specs/` numbering is sequential and independent from the `specs/` directory
  numbering.
- These numbers identify upstream product-spec documents only.
- The sequence continues at `058` for the Roadmap 77+ context/knowledge/memory program
  (see `docs/runtime/daemon-roadmaps.md`).

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
- `019` -> Roadmap 34: Tenant Identity And Access Foundation
- `020` -> Roadmap 35: Tenant-Scoped Data Migration
- `021` -> Roadmap 36: Tenant-Aware Operator Shell And SDK
- `022` -> Roadmap 37: Hosted Secrets, Integrations, And Connector Isolation
- `023` -> Roadmap 38: Billing, Quotas, And Usage Accounting
- `024` -> Roadmap 39: Production Install, Upgrade, Backup, And Soak
- `025` -> Roadmap 40: Live Validation And Side-Effect Replay
- `026` -> Roadmap 41: Evaluation Product Expansion
- `027` -> Roadmap 42: Integration Health And Permission Diagnostics
- `028` -> Roadmap 43: Hosted Operational Profile And Recovery
- `029` -> Roadmap 44: Roadmap Authority And Release Truth Reconciliation
  (`docs/runtime/release-truth-checklist.md` is the reusable release-closure checklist)
- `030` -> Roadmap 45: Hosted Signup And Tenant Activation
- `031` -> Roadmap 46: Hosted Credential And OAuth Setup Wizard
- `032` -> Roadmap 47: Public Quota Abuse And Billing UX
- `033` -> Roadmap 48: Channel Connector Conformance Contract
- `034` -> Roadmap 49: Discord Production Channel Hardening
- `035` -> Roadmap 50: Telegram Channel Connector
- `036` -> Roadmap 51: Slack Channel Connector
- `037` -> Roadmap 52: WhatsApp Or Matrix Channel Connector
- `038` -> Roadmap 53: Channel Management And Repair UX
- `039` -> Roadmap 54: Daemon-Owned Thread And Session Lifecycle
- `040` -> Roadmap 55: Non-Knowledge Multi-Turn Continuity
- `041` -> Roadmap 56: Group Room Reset And Handoff Semantics
- `042` -> Roadmap 57: Agent Profile And Persona Configuration
- `043` -> Roadmap 58: Workspace And Capability Binding
- `044` -> Roadmap 59: External Integration Adapter Plane
- `045` -> Roadmap 60: Real Calendar Provider Closure
- `046` -> Roadmap 61: Calendar Attendee And RSVP Workflows
- `047` -> Roadmap 62: Calendar Recurrence And All-Day Depth
- `048` -> Roadmap 63: Real Mail Provider Closure
- `049` -> Roadmap 64: Mail Attachment Transfer
- `050` -> Roadmap 65: Inbox Triage MVP Without Memory
- `051` -> Roadmap 66: Routine Builder
- `052` -> Roadmap 67: Webhook And External Trigger Plane
- `053` -> Roadmap 68: Operator-Managed Skill And Capability Catalog
- `054` -> Roadmap 69: Execution Backend And Sandbox Profile UX
- `055` -> Roadmap 70: Operator Shell Productization
- `056` -> Roadmap 71: Support Diagnostics And Evidence Bundle
- `057` -> Roadmap 72: Public Release Soak And Launch Gate
- `058` -> Roadmap 78: Memory Plane Foundation (implementation gated on the
  Roadmap 77 ship decision)
- `059` -> Roadmap 79: Context Engineering Foundation (gated)
- `060` -> Roadmap 80: Knowledge Retrieval (gated)
- `061` -> Roadmap 81: Agent-Managed Skills (gated)
- `062` -> Roadmap 82: Audited Self-Improvement (gated)

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

Release-closure claims should use
[`docs/runtime/release-truth-checklist.md`](../runtime/release-truth-checklist.md) when
implementation state and release evidence state could diverge.

## Relationship To Other Planning Artifacts

- `docs/runtime/daemon-roadmaps.md` defines roadmap order and closure criteria.
- `docs/harness/harness-architecture.md` defines architectural sequencing and rationale.
- `docs/specs/` captures the authoritative upstream spec for a slice.
- `specs/<NNN>-.../` holds the branch-local working area (spec/plan/tasks/contracts) once
  implementation planning begins.
