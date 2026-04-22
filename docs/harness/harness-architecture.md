# Harness Architecture

## Purpose

DopeAgent is no longer only a daemon plus connectors. Once skill loading became a first-class daemon contract, the system started moving into **harness engineering**.

The harness is the substrate that makes agent execution:

- inspectable
- controllable
- replayable
- auditable
- safe to extend

This document defines the current harness scope and the recommended implementation order.

## Harness Scope

The harness should include at least:

1. skill usage
2. sandbox environment
3. context engineering
4. memory system
5. MCP integration
6. tool-call orchestration
7. agent-managed skills
8. dreaming / self-improve

These are not eight unrelated features. They are eight parts of the same execution substrate.

## Core Design Principle

The harness should be designed as:

- a **control plane**
- over a **multi-backend execution substrate**

This is the right level of abstraction because Dope needs to reason about:

- what is allowed
- what is loaded
- what is executed
- where it is executed
- what state it reads and writes
- what evidence is left behind

without baking all of that into any single tool, connector, or provider.

## Harness Layers

### 1. Control Plane

The control plane owns:

- declarative profiles and policies
- skill discovery and inspection
- sandbox inspection and effective policy explanation
- provider identity and readiness
- MCP server inventory and lifecycle
- tool/capability manifests
- memory and context configuration
- operator approval and audit interfaces

The control plane should be explicit and inspectable.

### 2. Runtime Plane

The runtime plane owns:

- run, step, and event lifecycle
- dispatch decisions
- checkpoint and recovery behavior
- execution coordination across tools, providers, and channels
- orchestration state

The runtime plane should consume control-plane truth, not invent policy ad hoc.

### 3. Execution Plane

The execution plane owns:

- sandbox backends
- provider bridges
- MCP transport execution
- tool subprocesses
- future remote or container backends

This plane should be pluggable, but not ungoverned.

### 4. Knowledge Plane

The knowledge plane owns:

- context assembly
- memory retrieval
- memory write policy
- agent-managed skill evolution

This is the least mature part today and should be built on top of the control/runtime substrate, not before it.

## Recommended Build Order

### Phase 1: Skill Usage

Closed:

- daemon skill registry
- explicit chat skill selection
- `~/.agents` and `<dataDir>` roots

This phase establishes that execution hints and operator-authored instructions are part of daemon truth.

### Phase 2: Sandbox Environment

Next:

- sandbox contract
- first backend
- filesystem and env controls
- network controls
- audit trail

This phase makes execution boundaries explicit.

### Phase 3: MCP

After sandbox:

- requirement declarations for filesystem, network, secrets, and execution mode
- secret scope and redaction policy
- execution provenance and remaining local-consumer convergence
- MCP server registry
- MCP transport lifecycle
- MCP tool exposure policy
- MCP credential isolation

MCP should ride on the sandbox and control plane, not bypass them.

Current daemon status:

- MCP servers are first-class daemon resources with explicit profile and declaration binding
- MCP stdio transport lifecycle runs through sandbox-backed subprocess execution
- MCP `streamable-http` is available as the first remote transport without creating a second
  unmanaged execution plane
- MCP `websocket` is now available as the first additional long-lived session transport,
  with secret-ref-backed header auth and no anonymous fallback path
- tool exposure is explicit per tool and runtime surface instead of server-wide implicit enablement
- MCP tool invocation now reuses the existing `/v1/runs/.../tool-calls` runtime plane with
  approval, provenance, and audit continuity
- bundled MCP catalog entries can be installed through daemon API or the repo helper script,
  and both paths converge on the same installed server resource model
- catalog install, server inspection, and tool-call history all preserve origin, install
  method, transport identity, and redacted operator-visible output
- restart recovery restores enabled MCP servers through persisted daemon state
- MCP session bootstrap now times out explicitly so daemon start and server restore do not
  hang indefinitely on an unresponsive stdio or remote server
- websocket disconnect recovery is daemon-managed, bounded, and projected back through MCP
  server state and event history instead of a transport-specific background loop

Follow-on work should now split rather than extending the completed MCP slice:

- Roadmap 22: MCP catalog management and distribution
- Roadmap 23: additional MCP transports
- Roadmap 24: tool-call orchestration

### Phase 4: Tool-Call Orchestration

After MCP catalog management and transport expansion:

- policy-aware tool planning
- execution graph or ordered orchestration
- retries, cancellation, and partial-failure semantics
- audit of orchestration decisions

This is where Dope moves from “can call a tool” to “can run a controlled tool workflow.”

Current daemon status:

- workflow resources are now first-class daemon-owned records nested under runs
- `POST/GET /v1/runs/{runId}/workflows` plus `start` and `cancel` routes expose frozen
  plan inspection before execution
- workflow steps execute through the existing runtime `Step` and `ToolCall` plane rather
  than a parallel executor
- runtime `Run`, `Step`, and `ToolCall` resources now carry additive workflow linkage for
  reverse inspection
- mixed MCP plus executable-skill workflows now preserve dependency edges, handoff
  summaries, approval expectation preview, and runtime linkage in one operator-visible
  workflow resource
- daemon restart preserves persisted workflow audit truth and marks unfinished workflows
  `interrupted` instead of auto-resuming them

### Post-Phase 4: Personal-Agent Product Surface Before Knowledge Plane

Before context engineering becomes the main differentiator, the daemon should close the
remaining non-knowledge personal-agent product surface:

- Roadmap 25: scheduled tasks and wakeups
- Roadmap 26: use-computer capability plane
- Roadmap 27: personal integrations platform
- Roadmap 28: delivery and notifications
- Roadmap 29: calendar integration
- Roadmap 30: mail integration
- Roadmap 31: tasks and reminders
- Roadmap 32: operator shell and onboarding
- Roadmap 33: evaluation and replay harness

These are not memory work. They are the missing product and reliability layers that turn
the current daemon substrate into a serious personal agent.

Current daemon status after Roadmap 26:

- top-level schedule resources are now first-class daemon-owned records with create, list,
  inspect, pause, resume, and cancel routes
- one-time schedules dispatch normal runs or workflows with additive `scheduleId` and
  `scheduleAttemptId` linkage on runtime and workflow truth
- recurring schedules preserve timezone-aware next due time, paused history,
  overlap-skipped history, and bounded retry visibility
- daemon restart performs bounded catch-up and records missed intervals rather than
  replaying an unbounded backlog
- browser-first computer-use sessions, actions, and evidence artifacts are now first-class
  daemon-owned resources
- computer-use steps execute through the existing runtime and workflow plane with additive
  session and action linkage on tool-call and workflow-step truth
- phase 26 stays single-page and browser-first: extra tabs, new windows, and generalized
  desktop automation requests fail explicitly instead of widening the execution surface
- high-risk browser actions remain approval-gated with inspect-before-act context, and
  restart recovery leaves in-flight browser work `interrupted`
- personal integrations are now first-class daemon-owned resources with explicit
  readiness, canonical-default, and provenance truth
- the harness and runtime plane reuse additive `integrationBindings` snapshots on tool
  calls, workflow steps, and approvals instead of a parallel integration execution ledger
- the repo-owned fake integration backend closes roadmap-27 verification without touching
  live calendar or mail systems

### Phase 5: Context Engineering

After the non-knowledge personal-agent product surface:

- context sources
- packing and truncation rules
- explicit inclusion/exclusion policy
- context observability

Context engineering should compile structured truth, not replace it.

### Phase 6: Memory

After context:

- episodic memory
- semantic memory
- project or environment memory
- write promotion policy
- retrieval policy

Memory should be built on explicit provenance and execution evidence.

### Phase 7: Agent-Managed Skills

After memory:

- skill drafts generated by the agent
- managed revisions
- promotion, approval, and rollback
- attachment to memory and execution evidence

This is not just “memory.” It is a managed artifact system for reusable operational behavior.

### Phase 8: Dreaming / Self-Improve

After memory, managed skills, and replay or evaluation support:

- offline reflection over runs, failures, approvals, and tool traces
- candidate generation for memory, skills, policy, and orchestration improvements
- promotion gates for self-generated changes
- evaluation-backed rollout or rejection

Dreaming should not directly rewrite production behavior. It should produce managed candidates that move through explicit review and promotion paths.

## Agent-Managed Skills

Agent-managed skills deserve to be treated as a distinct harness capability.

They are related to memory, but they are not identical to memory.

### Why they are distinct

Memory answers:

- what happened
- what was learned
- what is likely useful later

Agent-managed skills answer:

- what reusable operational procedure should exist now
- which version is active
- what evidence justified promoting it
- who approved it
- how it can be rolled back

That means agent-managed skills need:

- draft state
- provenance
- diff history
- approval or promotion gates
- activation and deactivation controls
- rollback semantics

This is much closer to managed operational artifacts than plain memory recall.

## Dreaming / Self-Improve

Dreaming is a high-order harness capability, not just another memory feature.

It should be treated as an improvement pipeline that consumes execution evidence and produces managed candidates.

### Inputs

- run and step history
- tool-call traces
- provider and channel failures
- approvals and operator corrections
- memory reads and writes
- managed skill usage outcomes

### Outputs

- memory candidates
- managed skill drafts
- policy or orchestration candidates
- context assembly candidates
- evaluation or replay fixtures

### Why it is separate from memory

Memory answers:

- what happened
- what was learned
- what might matter later

Dreaming answers:

- what should change
- which reusable behavior should be promoted
- what should be evaluated before promotion
- what should be rolled back or rejected

That means dreaming depends on:

- provenance
- sandboxed execution and validation
- approval and promotion gates
- managed artifact lifecycle
- replay or evaluation support

Without those, dreaming becomes uncontrolled self-modification rather than managed improvement.

## Additional Harness Capabilities Worth Planning

Beyond the list above, the next high-value harness capabilities are:

### 1. Capability Metadata And Requirement Declarations

Every skill, provider bridge, MCP server, and tool should be able to declare:

- required sandbox profile
- required network access
- required filesystem access
- required secrets
- expected execution mode

Without this, the control plane cannot reason correctly about execution safety.

### 2. Secret Scope And Redaction Policy

The harness should know:

- which secret may be exposed to which backend
- whether a secret is available in test, prod, or both
- how secrets are redacted from config and logs

This becomes important as soon as MCP and tool orchestration grow.

### 3. Execution Provenance

Every meaningful output should answer:

- which skill influenced it
- which provider handled it
- which tools ran
- which sandbox profile executed it
- which memory items or overlays were applied

Without provenance, debugging later harness behavior becomes guesswork.

### 4. Managed Artifact Lifecycle

The harness will eventually need a general artifact model for:

- managed skills
- generated prompt fragments
- reusable plans
- policy presets
- memory summaries

This should probably become a common artifact substrate rather than a one-off feature.

### 5. Evaluation And Replay Harness

As the harness grows, replay and evaluation become necessary.

The system should eventually support:

- replaying a run with the same skills and policies
- comparing before/after skill or sandbox changes
- regression testing orchestration and context behavior

This is a later phase, but it should stay visible in the architecture.

## Current Recommendation

The right next step is:

- Roadmaps 22 through 25 are now closed
- Roadmap 26 is now closed
- the next implementation roadmap should start at Roadmap 27: personal integrations
  platform

Then continue in this order:

1. personal integrations platform
2. delivery and notifications
3. calendar integration
4. mail integration
5. tasks and reminders
6. operator shell and onboarding
7. evaluation and replay harness
8. context engineering
9. memory
10. agent-managed skills
11. dreaming / self-improve

This keeps the harness grounded in execution control before it grows into adaptive knowledge systems.
