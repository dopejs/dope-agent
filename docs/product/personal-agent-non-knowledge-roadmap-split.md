# Personal Agent Non-Knowledge Roadmap Split

Status: proposed

Authority: This document is the roadmap-splitting authority for the remaining personal-agent work that should land before knowledge-plane investment becomes the main differentiator.

Primary source documents:
- `docs/harness/harness-architecture.md`
- `docs/runtime/daemon-roadmaps.md`
- `docs/product/feature-phasing.md`
- `docs/product/openclaw-architecture-gaps.md`

## Background

DopeAgent now has the core execution substrate needed for a serious personal agent:
runtime truth, policy and approval gating, sandboxed execution, MCP lifecycle, and tool-call orchestration. What remains is not another execution primitive. What remains is the product surface that makes the system behave like an always-on personal agent instead of a strong daemon demo.

## Goal

Define the non-knowledge-plane roadmap split that should be completed before context engineering and memory become the main investment focus.

## Fixed Decisions

- This work is not demo planning.
- The remaining product surface must be split into multiple roadmap-closed specs rather than one broad umbrella implementation.
- Knowledge-plane work stays later by default.
- The target shape is an ambient personal agent that can wake itself, act through external systems, deliver results, and remain operator-visible.
- Domain implementations should build on one runtime, workflow, policy, and audit substrate.

## Dependencies On Completed Phases

- Roadmap 16: Sandbox Execution Plane
- Roadmap 17: Sandbox Requirement Declarations And Consumer Convergence
- Roadmap 18: MCP Execution Plane
- Roadmap 22: MCP Catalog Management And Distribution
- Roadmap 23: Additional MCP Transports
- Roadmap 24: Tool-Call Orchestration

## Roadmap Split

The post-orchestration pre-knowledge program should be split into:

1. `010-scheduled-tasks-and-wakeups.md`
2. `011-use-computer-capability-plane.md`
3. `012-personal-integrations-platform.md`
4. `013-delivery-and-notifications.md`
5. `014-calendar-integration.md`
6. `015-mail-integration.md`
7. `016-tasks-and-reminders.md`
8. `017-operator-shell-and-onboarding.md`
9. `018-evaluation-and-replay-harness.md`

## Why Separate Specs Are Required

- scheduled wakeups are trigger-plane work, not domain logic
- use-computer is a capability-plane and safety problem
- personal integrations need one shared platform contract before domain-specific slices
- calendar, mail, and reminders all have distinct object models and failure modes
- operator shell and evaluation are not optional polish for a production personal agent

## Out Of Scope

- context engineering
- memory retrieval or write policy
- agent-managed skills
- dreaming or self-improvement
- broad mobile-node or device-fleet work

## Verification Expectations

Each child roadmap must end with:

- operator-visible API and event truth
- durable persistence and restart semantics where applicable
- targeted regression coverage
- one clear manual acceptance path in `DOPE_ENV=test` when real external behavior matters

## Definition Of Done

This split is done when the child specs exist, their roadmap ordering is aligned in runtime and harness docs, and future implementation can start from the child docs without reopening the program structure.

## Recommended Spec Input

Do not implement this umbrella document directly. Use one of the child specs as the authoritative input for a future roadmap slice.
