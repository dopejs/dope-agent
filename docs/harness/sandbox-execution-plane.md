# Sandbox Execution Plane

## Purpose

This document defines the detailed design target for `Roadmap 16: Sandbox Execution Plane`.

The sandbox system is not only a local command runner. It is the execution boundary for the Dope harness.

Its purpose is to make execution:

- policy-driven
- inspectable
- auditable
- backend-agnostic
- safe to extend across skills, provider bridges, MCP, and future tool orchestration

## Design Position

The design direction is:

- a **sandbox control plane**
- over a **multi-backend execution substrate**

This is intentionally closer to:

- OpenClaw's explicit sandbox control plane
- HermesAgent's multi-backend execution substrate
- Claude Code and Codex style permission and approval semantics

but without inheriting workspace-specific assumptions that Dope does not currently have.

## Goals

Roadmap 16 should close these goals:

1. make sandbox policy a first-class daemon object
2. support more than one backend model in the architecture, even if only one backend is implemented first
3. give skills, provider bridges, and future MCP servers a common execution boundary
4. distinguish policy rejection from runtime failure
5. leave an auditable trail for all sandboxed executions

## Non-Goals

Roadmap 16 should **not** try to close:

- VM-grade isolation
- browser or desktop isolation
- full container orchestration
- full orchestration planner
- context or memory behavior
- automatic self-improvement

Those should sit on top of the sandbox substrate, not inside it.

## Benchmark Synthesis

### What to take from OpenClaw

- sandbox is a visible control-plane object
- effective policy should be explainable
- operators should be able to list and inspect active sandboxes
- backend, scope, and access shape should be explicit

### What to take from HermesAgent

- backend should not be hard-coded
- local subprocess, container, ssh, and remote execution should all fit the model
- execution environments should be pluggable without changing the control plane

### What to take from Claude Code and Codex

- permission semantics should be explicit
- approval gates should be part of the system model, not ad hoc wrappers
- allow, ask, and deny should be representable at policy level
- local execution and remote execution should eventually share one execution contract

## Core Principles

### 1. Control Plane First

Execution should not be configured ad hoc at the point where a tool runs.

The daemon should own:

- profiles
- effective policy
- backend selection
- approval requirements
- audit visibility

### 2. Backend Independence

The control plane should not assume:

- subprocess only
- container only
- local only

The first backend can be local subprocess. The model must still be written so docker or ssh backends fit later without rewriting policy objects.

### 3. Policy Is Separate From Capability

A skill, provider bridge, MCP server, or future tool can declare what it needs.

The sandbox system decides:

- whether that need is allowed
- which backend is acceptable
- whether approval is required

### 4. Audit Is Part Of The Product

The output of sandbox execution is not just `stdout` and `stderr`.

It must also record:

- what policy was applied
- which backend executed
- whether approval was required
- whether execution was allowed or denied
- whether the failure was policy, transport, timeout, cancellation, or subprocess failure

### 5. Test And Prod Isolation Must Stay Intact

Sandbox defaults must respect the current environment split:

- `test` uses `~/.dope-test`
- `prod` uses `~/.dope`

Nothing in the sandbox design should collapse that separation.

## Object Model

The sandbox system should introduce these first-class objects.

### SandboxProfile

Represents reusable policy and backend intent.

Suggested fields:

- `profileId`
- `title`
- `description`
- `backendKind`
- `defaultWorkDir`
- `filesystemPolicy`
- `networkPolicy`
- `envPolicy`
- `approvalPolicy`
- `processPolicy`
- `defaultTimeoutMs`
- `maxTimeoutMs`
- `restartable`
- `source`
- `active`

### SandboxBackend

Represents the backend family, not an individual execution.

Suggested values:

- `subprocess`
- `docker`
- `ssh`
- `remote`

Only `subprocess` needs implementation in Roadmap 16.

### SandboxExecutionRequest

Represents one execution attempt.

Suggested fields:

- `executionId`
- `profileId`
- `backendKind`
- `command`
- `args`
- `cwd`
- `env`
- `stdinPolicy`
- `timeoutMs`
- `requestedBy`
- `resourceKind`
- `resourceId`
- `scope`
- `approvalId`
- `reason`
- `metadata`

### SandboxExecutionResult

Represents the terminal result of one execution attempt.

Suggested fields:

- `executionId`
- `status`
- `startedAt`
- `completedAt`
- `exitCode`
- `signal`
- `stdout`
- `stderr`
- `outputTruncated`
- `errorClass`
- `errorCode`
- `error`
- `partial`
- `backendMetadata`

### SandboxDecision

Represents the policy decision before execution starts.

Suggested fields:

- `decisionId`
- `executionId`
- `resolution`
- `matchedRules`
- `approvalRequired`
- `approvalStatus`
- `effectiveProfileId`
- `effectiveBackendKind`
- `explanation`

## Policy Model

The policy model should be explicit and composable.

### Filesystem Policy

Suggested shape:

- `mode`
  - `none`
  - `scoped`
  - `full`
- `readRoots`
- `writeRoots`
- `tempRoots`
- `allowDataDir`
- `allowUserAgentsDir`
- `allowHomeRead`
- `allowHomeWrite`

Roadmap 16 should not assume whole-home access is acceptable.

### Network Policy

Suggested shape:

- `mode`
  - `deny`
  - `allow_list`
  - `full`
- `allowedHosts`
- `allowedPorts`
- `allowLoopback`

The first backend may not be able to hard-enforce every network rule at OS level. The model should still be explicit, and enforcement strength should be visible.

### Environment Policy

Suggested shape:

- `mode`
  - `clean`
  - `inherit_safe`
  - `inherit_all`
- `allowedVars`
- `injectedVars`
- `secretRefs`
- `redactedVars`

This matters immediately for:

- provider bridges
- MCP credentials
- local CLI integrations

### Approval Policy

Suggested shape:

- `mode`
  - `allow`
  - `ask`
  - `deny`
- `requiredForCommands`
- `requiredForWritesOutsideRoots`
- `requiredForNetwork`
- `requiredForUnknownBackends`

### Process Policy

Suggested shape:

- `timeoutMs`
- `maxTimeoutMs`
- `killGraceMs`
- `captureStdout`
- `captureStderr`
- `maxOutputBytes`
- `allowStreaming`
- `restartOnFailure`

## Control Plane APIs

Roadmap 16 should expose at least these API groups.

### Profiles

- `GET /v1/sandboxes/profiles`
- `GET /v1/sandboxes/profiles/{profileId}`
- `POST /v1/sandboxes/profiles/reload`

### Executions

- `GET /v1/sandboxes/executions`
- `GET /v1/sandboxes/executions/{executionId}`
- `POST /v1/sandboxes/executions`
- `POST /v1/sandboxes/executions/{executionId}/cancel`

### Explain

- `POST /v1/sandboxes/explain`

This route is important. Operators need to know:

- which profile would be selected
- whether approval would be required
- which directories and network policy would apply
- why execution would be denied

### Events

Suggested event categories:

- `sandbox.execution_requested`
- `sandbox.decision_recorded`
- `sandbox.execution_started`
- `sandbox.execution_completed`
- `sandbox.execution_failed`
- `sandbox.execution_cancelled`
- `sandbox.execution_denied`

## Backend Model

### First Backend: Subprocess

Roadmap 16 should implement one backend:

- `subprocess`

This backend should support:

- explicit cwd
- explicit env policy
- timeout and cancellation
- stdout and stderr capture
- basic filesystem boundary checks before launch
- policy-level network declaration

It does **not** need to provide hard OS containerization in the first roadmap.

### Future Backends

The model should be compatible with later backends:

- `docker`
- `ssh`
- `remote`

That means backend-specific fields must stay in backend metadata, not leak into the common execution contract.

## Integration Points

### Skills

Skills should not directly execute arbitrary scripts through ad hoc logic.

Instead, when execution becomes allowed in later roadmaps, skills should declare:

- required profile
- required filesystem access
- required network access

and run through sandbox execution requests.

### Provider Bridges

Managed CLI providers are an immediate consumer:

- `claude_managed`
- `codex_managed`

They should eventually move from ad hoc subprocess handling to explicit sandboxed provider-bridge execution.

### MCP

MCP should be designed assuming:

- each MCP server may run in a sandbox profile
- credentials are provided through env policy
- network access is explicitly declared

### Tool Orchestration

Tool orchestration should not invent its own process execution path.

It should submit work into sandbox execution with:

- profile
- approval context
- scope
- execution metadata

## Audit And Observability

Roadmap 16 should record enough information to answer:

- what tried to run
- which policy applied
- whether execution was denied
- whether approval was required
- which backend ran it
- what it wrote to stdout or stderr
- whether it timed out
- whether it was cancelled

The system should explicitly distinguish:

- policy denial
- approval rejection
- backend launch failure
- runtime process failure
- timeout
- cancellation

## Failure Model

The smallest useful error classes are:

- `policy_denied`
- `approval_required`
- `approval_rejected`
- `invalid_profile`
- `backend_unavailable`
- `launch_failed`
- `process_failed`
- `timeout`
- `cancelled`
- `io_capture_failed`

These should be durable and queryable through APIs, not only logs.

## Security Position

Roadmap 16 should be honest about its security posture.

The first sandbox backend will be:

- policy-rich
- auditable
- process-isolated

but not yet:

- kernel-level fully isolated
- container-grade hardened
- VM-grade hardened

That is acceptable for the roadmap as long as:

- the control plane is explicit
- the limits are documented
- later backends can strengthen isolation without redesigning the system

## Suggested Implementation Order

1. Define schema-backed contracts for profiles, decisions, and executions
2. Add control-plane APIs for profiles, executions, and explain
3. Implement subprocess backend with cwd, env, timeout, and cancellation
4. Add filesystem and network policy evaluation
5. Add durable events and execution history
6. Add operator docs and contract tests

## Roadmap 16 Completion Standard

Roadmap 16 is only complete when:

- sandbox is a first-class daemon plane
- the first backend executes through explicit control-plane contracts
- policy can explain allow, ask, or deny outcomes
- executions are auditable and inspectable
- the design is demonstrably ready to host skills, provider bridges, MCP, and future tool orchestration
