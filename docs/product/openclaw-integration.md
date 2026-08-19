# OpenClaw Comparison And Integration Options

## Position

OpenClaw is the current feature benchmark for Kura, but it is not a required architecture or implementation base.

We may choose one of three paths:

- treat OpenClaw only as a product reference and build a new daemon from scratch
- reuse selected OpenClaw ideas and interfaces, but not its runtime internals
- integrate with or fork parts of OpenClaw if that becomes the fastest low-risk path later

This decision should remain open until the product scope, module map, and technical stack are stable.

## What We Are Actually Benchmarking

The benchmark target is capability coverage, not internal implementation details.

The product areas currently used for comparison are:

- daemon and gateway model
- Web UI, TUI, and chat entry points
- sessions, routing, and multi-agent isolation
- tools, nodes, and automation hooks
- plugins and skills
- persona and workspace shaping
- context engineering
- memory engineering

## If We Reuse OpenClaw, What Might Be Worth Keeping

### Keep

- gateway and channel ingress
- multi-surface host integration
- workspace and tool hosting primitives
- notification and wake-up surfaces
- identity and session entry points

## What We Already Expect To Redesign

Whether or not OpenClaw is reused, these areas are expected to be replaced or substantially redesigned:

- prompt-centric context assembly
- memory file conventions as the primary truth source
- session compaction logic
- agent handoff format
- planning loop
- policy engine
- long-running execution state model

## Specific Concern: Prompt Files

Files such as `AGENTS.md`, `SOUL.md`, and `TOOLS.md` can remain useful as human-editable overlays, but should not remain the primary runtime truth source.

Recommended future role:

- bootstrap hints
- persona overlays
- user-authored preferences
- tool or workspace documentation

They should not determine:

- system memory state
- long-lived execution state
- policy decisions
- inter-agent handoff contracts

## If OpenClaw Is Used As A Host Shell

OpenClaw should call into a new core through explicit contracts:

- `start_run`
- `resume_run`
- `append_input`
- `request_recall`
- `execute_tool`
- `record_artifact`
- `save_checkpoint`

The new core should return structured objects rather than free-form strings wherever possible.

## If OpenClaw Is Not Used

The same boundary still matters, but the new daemon will own both shell and core:

- gateway protocol
- operator UI API
- channel adapter contracts
- node capability contracts
- runtime API
- storage and policy boundaries

## Migration Strategy If OpenClaw Is Reused

### Phase 1

Keep OpenClaw surfaces and tool hosting. Introduce a new event log and step runtime under the existing shell.

### Phase 2

Replace memory and compaction with policy-controlled memory and a new context pipeline.

### Phase 3

Replace agent handoff and planning with structured protocols and domain-specialized capability agents.

### Phase 4

Move prompt files into overlays and human configuration rather than runtime state carriers.

## Integration Risks

- hidden assumptions inside OpenClaw prompt construction
- state spread across markdown files and implicit workspace behavior
- partial duplication between OpenClaw session state and new runtime state
- unclear ownership of permissions and escalation

These risks should be handled with an explicit boundary contract before implementation starts.
