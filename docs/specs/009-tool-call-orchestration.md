# Spec 009: Tool-Call Orchestration

## Status

Planned

## Authority

This document is the authoritative upstream spec for Roadmap 24 before a branch-local
speckit feature spec is created.

Primary source documents:

- `docs/runtime/daemon-roadmaps.md` Roadmap 24
- `docs/harness/harness-architecture.md` Phase 4 orchestration direction

## Background

The daemon now has one execution boundary for:

- local tools
- executable skills
- MCP tools

What it does not yet have is a first-class orchestration layer that can plan and run
multi-step workflows across those consumer families while preserving policy, sandbox, and
audit truth.

This phase is the point where the harness moves from “can execute one controlled tool
call” to “can execute a controlled tool workflow.”

## Goal

Build a policy-aware orchestration layer on top of the unified runtime tool-call plane so
skills, local tools, and MCP tools can participate in ordered or graph-shaped workflows.

## Fixed Decisions

- Orchestration must run on top of the existing runtime tool-call plane, not replace it.
- Existing policy, approval, sandbox, and provenance guarantees remain the baseline for
  every concrete workflow step.
- This phase is not a memory-driven planner or self-improvement system.
- Mixed workflows must not create bypass paths around MCP, skill, or local-tool controls.

## Dependencies

- Roadmap 19: Skill And Local Tool Sandbox Execution
- Roadmap 21: Complete MCP Runtime And Catalog

## Scope

### In Scope

- planning and selection model for multi-step tool workflows
- ordered or graph-shaped workflow execution
- retries, cancellation, and partial-failure semantics
- cross-consumer coordination between MCP tools, local tools, and executable skills
- operator-visible orchestration state, rationale, and audit trail
- verification of at least one mixed workflow

### Out Of Scope

- autonomous self-improvement loops
- memory-driven planning systems
- marketplace or ecosystem discovery surfaces
- broad context engineering or memory work beyond orchestration needs

## Operator Problems To Solve

1. A single controlled tool call is not enough for real workflows that need planning,
   sequencing, fallback, or retries.
2. Mixed workflows must not bypass existing sandbox, approval, or runtime truth.
3. Partial workflow failure must remain visible without forcing operators to reconstruct
   state from raw events.

## User Stories

### Story 1: Inspect Why A Workflow Was Planned

As an operator, I want orchestration decisions to be explicit so I can understand why the
daemon selected a particular tool sequence or graph.

Acceptance expectations:

- workflow planning decisions are inspectable and replayable
- tool choice and ordering can be explained without reading raw logs
- policy and approval expectations remain attached to concrete execution steps

### Story 2: Run A Multi-Step Workflow Safely

As an operator, I want the daemon to run a controlled multi-step workflow through the
existing runtime plane so sequencing does not create an unmanaged side path.

Acceptance expectations:

- every execution step is still a normal runtime tool call or equivalent tracked action
- retries and cancellation remain explicit
- already-visible side effects are not hidden by later failure or retry

### Story 3: Coordinate Mixed Consumer Families

As an operator, I want one workflow to combine MCP tools, local tools, and executable
skills without losing policy or provenance truth.

Acceptance expectations:

- workflows can combine at least two consumer families
- cross-tool data flow and handoff remain operator-visible
- mixed workflows do not bypass sandbox or approval controls

## Functional Requirements

- The daemon must support a first-class orchestration model for ordered or graph-shaped
  tool workflows.
- Orchestration decisions must be explicit, inspectable, and auditable.
- Orchestrated execution must remain on the existing runtime plane rather than introducing
  a second execution boundary.
- Each concrete workflow step must preserve the same policy, approval, provenance, and
  sandbox guarantees it would have outside orchestration.
- The daemon must represent partial workflow failure explicitly.
- Retry, cancellation, and partial-failure semantics must be visible in runtime history and
  operator-facing records.
- The orchestration layer must support mixed workflows spanning at least two of:
  - MCP tools
  - local tools
  - executable skills

## Compatibility And Operational Notes

- This phase should extend the current runtime plane, not replace it.
- Existing single-step tool-call behavior must remain valid and auditable.
- Workflow planning should remain subordinate to explicit policy and execution truth rather
  than inventing new hidden policy semantics.

## Verification

- targeted daemon tests for planning state, ordered execution, retries, cancellation, and
  partial failure
- contract coverage for orchestration-visible runtime and event surfaces
- manual verification of at least one mixed workflow using the daemon-owned runtime plane

## Definition Of Done

- the harness can run controlled multi-step tool workflows on one execution boundary
- orchestration decisions are auditable and policy-aware
- mixed MCP, skill, and local-tool workflows do not introduce bypass paths

## Recommended Speckit Input

```text
$speckit-specify 结合 docs/specs/009-tool-call-orchestration.md 完成 phase 24 的工作
```
