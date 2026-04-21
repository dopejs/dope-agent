# P0 Daemon Roadmaps

## Purpose

This document defines the execution structure for the P0 daemon.

The delivery unit is a **roadmap**, not an isolated task.

Each roadmap:

- forms a closed vertical slice
- contains multiple tasks
- has a roadmap-level definition of done
- is only considered complete when every in-scope task meets its own completion standard

Each implementation round should finish **one whole roadmap**.

## Execution Standard

- A roadmap is the planning and delivery boundary.
- A roadmap remains the only acceptable implementation delivery unit.
- A task is an auditable work item inside a roadmap.
- Tasks exist to make progress and gaps inspectable. They do not authorize shipping a partial roadmap.
- A task can only be marked `[x]` when its task-level definition of done is fully satisfied.
- A roadmap can only be marked `[x]` when every required task inside it is fully satisfied and the roadmap-level definition of done is met.
- Partial, provisional, or narrow implementations stay `[ ]` with a note like `(partial)` or `(blocked)`.
- If a roadmap is too large to finish in one round, it must be re-cut into smaller roadmaps before implementation starts. It should not be "partially completed and counted as done".
- Demo-grade shortcuts do not count as roadmap completion.

## Current State

The daemon now has a closed P0 control plane slice:

- system routes
- run and step resources
- event bus and SSE
- SQLite persistence
- checkpoint and restart recovery
- session routing
- supervision state
- LLM dispatch
- auth and approval gating
- contract tests
- versioned migrations

The next execution focus is provider-stream hardening for SSE-style upstreams.

The current execution focus after that is repository-safe local development workflow, so agents default to the test environment instead of production state.

The next execution focus is first-class skill loading and prompt support, including support for user-level assets under `~/.agents`.

The sandbox execution plane is now closed as the first harness control-plane slice.

The sandbox prerequisite slice is now implemented for the current in-scope consumers:

- shared requirement declarations
- secret scope and redaction policy
- durable execution provenance
- convergence of managed providers, the current skill registry and explicit skill-selection surfaces, and the current high-risk local tool-call path

The remaining roadmap work is narrow operator-surface closure; the next major sandbox-backed harness subsystem is MCP.

## Roadmap 1: Runtime Closure

Status: `[x] complete`

### Goal

Turn the daemon runtime from a set of resource handlers into a closed, restart-safe execution core.

### Tasks

#### 1. Runtime Lifecycle Commands

Scope:

- add run cancel command
- add run resume command
- add step cancel command

Task definition of done:

- explicit command routes exist for run cancel, run resume, and step cancel
- runtime applies legal transitions only
- invalid transitions return stable API errors
- emitted events cover the command result and resulting runtime state changes
- persistence and checkpoint state remain correct after each command
- unit and API tests cover happy path and rejection path

#### 2. Terminal-State And Idempotency Rules

Scope:

- define terminal-state validation
- define command idempotency rules

Task definition of done:

- terminal states are explicit for run and step
- illegal post-terminal mutations are rejected consistently
- duplicate command submissions have defined idempotent behavior
- behavior is documented in API contract or task notes
- tests cover duplicate submissions and terminal-state violations

#### 3. Daemon Lifecycle Hooks And System Events

Scope:

- add graceful subsystem shutdown hooks
- emit startup and shutdown system events

Task definition of done:

- daemon startup emits a durable startup event
- daemon shutdown emits a durable shutdown event when shutdown is orderly
- event bus, HTTP server, persistence layer, and checkpoint manager are shut down in defined order
- shutdown does not corrupt persisted runtime state
- tests cover orderly shutdown behavior at subsystem level

#### 4. Event Continuation Semantics

Scope:

- add event sequence or cursor semantics

Task definition of done:

- every persisted event has a stable ordering token
- SSE replay can resume from a cursor or sequence boundary
- event listing and streaming semantics are documented and tested
- duplicate or skipped replay behavior is explicit

#### 5. Config Inspection API

Scope:

- add config read API

Task definition of done:

- daemon exposes a read-only config inspection route
- response shape is schema-backed
- sensitive fields are redacted or omitted by policy
- tests cover config load, config read, and redaction behavior

#### 6. Runtime Contract Hardening

Scope:

- align runtime, session, and config responses with schemas

Task definition of done:

- current runtime and session resource responses match committed schemas
- drift between docs, schemas, and implementation is removed for in-scope routes
- tests fail on contract mismatch or fixture mismatch

#### 7. Restart Integration Coverage

Scope:

- add daemon restart integration test for runtime boundary

Task definition of done:

- an integration test proves runtime state, events, and checkpoints survive daemon restart
- the test covers at least one non-trivial run with steps and state transitions
- restart test fails if replay, ordering, or checkpoint restore breaks

### Roadmap Definition Of Done

- runtime lifecycle commands are explicit and restart-safe
- terminal-state and idempotency behavior are defined and enforced
- daemon lifecycle events and graceful shutdown are implemented
- event continuation semantics are stable enough for operator clients
- config can be inspected safely through API
- runtime-facing contracts are aligned with schemas
- restart integration coverage proves the runtime slice is closed

### Explicitly Out Of Scope

- connector supervision
- capability supervision
- LLM provider integration
- auth and pairing

## Roadmap 2: Supervision Plane

Status: `[x] complete`

### Goal

Turn connectors and capabilities into daemon-managed supervised units with explicit health and restart semantics.

### Tasks

#### 1. Connector Supervisor Contract

Scope:

- define connector supervisor contract
- define connector health model

Task definition of done:

- connector lifecycle states are explicit
- daemon can register, inspect, and report connector state
- health model and failure state are exposed through API and schema
- tests cover registration, health updates, and failure observation

#### 2. Capability Supervisor Contract

Scope:

- define capability supervisor contract
- define capability health model

Task definition of done:

- capability lifecycle states are explicit
- daemon can register, inspect, and report capability state
- health model and failure state are exposed through API and schema
- tests cover registration, health updates, and failure observation

#### 3. Restart And Backoff Policy

Scope:

- add restart and backoff policy

Task definition of done:

- restart policy is explicit for connectors and capabilities
- backoff state is observable
- repeated failures do not create uncontrolled restart loops
- tests cover restart, backoff escalation, and terminal failure

#### 4. Connector And Capability APIs

Scope:

- add connector routes
- add capability routes
- create connector and capability schemas

Task definition of done:

- daemon exposes list/get and health routes for connectors and capabilities
- responses align with schemas
- tests cover API behavior and schema-backed fixtures

#### 5. Tool-Call Dispatch Boundary

Scope:

- define capability dispatch boundary for tool calls
- attach tool calls to durable execution history

Task definition of done:

- tool calls have a durable store, not only runtime attachment
- tool call records identify dispatched capability boundary
- requested/completed/failed trail is queryable through API
- tests cover dispatch recording and failure recording

### Roadmap Definition Of Done

- daemon can supervise connectors and capabilities as first-class managed units
- health, restart, and backoff behavior are explicit and observable
- tool calls cross a defined capability boundary with durable execution history
- connector and capability contracts are schema-backed and tested

### Explicitly Out Of Scope

- real external IM integrations beyond supervised contract surface

## Roadmap 14: Test Environment Workflow

Status: `[x] complete`

### Goal

Make local daemon debugging default to a repository-safe test environment with explicit startup entrypoints and a project-level skill.

### Tasks

#### 1. Test-vs-Production Environment Defaults

Scope:

- separate test and production defaults
- make development default to test environment

Task definition of done:

- test environment defaults are explicit for data dir, config file, and bind addr
- production defaults remain explicit and opt-in
- API-visible config/system responses expose the active environment
- tests cover environment resolution

#### 2. Quick-Start Repository Entry Points

Scope:

- add standard startup and health commands for test and production

Task definition of done:

- repository-level commands exist for starting test and production daemon instances
- repository-level commands exist for checking test and production daemon health
- commands are documented and match actual runtime behavior
- operators do not need to remember ad hoc shell commands

#### 3. Project-Level Test Environment Skill

Scope:

- add a repository skill for local daemon debugging
- add repository-specific agent instructions

Task definition of done:

- the repository contains a project skill that tells future agents to use the test environment by default
- the repository contains repo-level agent instructions aligned with the skill
- production usage is clearly treated as explicit opt-in

#### 4. Documentation Closure

Scope:

- document the environment split and startup workflow

Task definition of done:

- README documents test and production defaults
- the dedicated design note exists for test-environment workflow
- docs, scripts, and commands all refer to the same ports and paths

### Roadmap Definition Of Done

- local development defaults to the test environment
- standard commands exist to run and inspect test and production daemon instances
- project-level agent instructions and skill both push debugging toward the test environment
- the startup and environment model is documented and verified

### Explicitly Out Of Scope

- containerized dev environments
- multiple named test profiles
- environment-specific database migrations beyond the existing data-dir split

## Roadmap 15: Skill Registry And Prompt Support

Status: `[x] complete`

### Goal

Add a first-class skill registry to the daemon, load skills from the active Dope data dir and `~/.agents`, and make explicit skill usage available through the chat plane.

### Tasks

#### 1. Skill Source And Overlay Discovery

Scope:

- discover data-dir and user-level agent roots
- load skill directories from `<dataDir>/skills` and `~/.agents/skills`
- load top-level agent overlays from `<dataDir>/AGENTS.md` and `~/.agents/AGENTS.md`

Task definition of done:

- daemon can discover skills from `<dataDir>/skills` and from `~/.agents/skills`
- daemon can discover supported agent overlay files from `<dataDir>` and `~/.agents`
- discovery precedence between `dataDir` and `~/.agents` is explicit
- invalid skill roots fail safely without corrupting the registry
- tests cover discovery, precedence, and missing-root behavior

#### 2. Skill Registry And Inspection API

Scope:

- add in-memory skill registry
- add skill list/get/reload APIs

Task definition of done:

- daemon exposes list/get/reload routes for skills
- skill resources include source, path, metadata, and bundled file inventory
- overlay metadata is inspectable through API
- responses are schema-backed and covered by contract tests

#### 3. Explicit Skill Support In Chat

Scope:

- extend chat contract with explicit skill selection
- compile requested skills and overlays into prompt input

Task definition of done:

- `/v1/chat/query` and `/v1/chat/query/stream` accept explicit skill identifiers
- requested skills are resolved deterministically from the registry
- `~/.agents` and `dataDir` overlays are applied in documented order
- skill-backed chat requests work in both non-stream and stream paths
- tests prove the resulting LLM messages include overlays and selected skills

#### 4. Operator Documentation And Verification

Scope:

- document skill roots, precedence, and supported file types
- close roadmap-level verification

Task definition of done:

- README and dedicated design doc describe skill loading behavior
- project-level docs explain `~/.agents` support and `dataDir` support
- tests cover registry, API, and chat integration

### Roadmap Definition Of Done

- skills are loaded from both `<dataDir>/skills` and `~/.agents/skills`
- agent overlay files are loaded from supported roots
- operators can inspect the loaded registry through daemon API
- chat can explicitly apply selected skills without bypassing the daemon
- contracts, docs, and tests all reflect the final behavior

### Explicitly Out Of Scope

- automatic skill selection from natural-language intent
- marketplace install/update flows
- executing bundled skill scripts automatically
- context-engine level reference-file loading heuristics
- full browser or media capability implementations
- auth and pairing

## Roadmap 16: Sandbox Execution Plane

Status: `[x] complete`

### Goal

Add a first-class sandbox control plane to the daemon and a multi-backend execution substrate for skills, provider bridges, MCP, and future tool orchestration.

### Tasks

#### 1. Sandbox Contract And Policy Model

Scope:

- define sandbox profile resource model
- define execution request and execution result contracts
- define filesystem, environment, network, and approval policy shape

Task definition of done:

- daemon has explicit sandbox profile and execution contracts
- policy fields distinguish filesystem, env, network, timeout, and approval rules
- contracts are designed for multiple backends, not only local subprocess execution
- operator-facing docs explain capability model and degradation behavior
- schemas and tests cover the in-scope contract surface

#### 2. Sandbox Control Plane APIs

Scope:

- add sandbox inspection and lifecycle routes
- add operator visibility for active sandboxes and profiles

Task definition of done:

- daemon exposes list/get style APIs for sandbox profiles and active executions
- control plane can explain effective policy for a sandboxed execution
- control plane supports reload or recreation semantics where needed
- responses are schema-backed and covered by contract tests

#### 3. Subprocess Sandbox Runner

Scope:

- implement the first execution backend as a managed subprocess runner
- enforce cwd, env filtering, timeout, and cancellation semantics

Task definition of done:

- daemon can execute sandboxed subprocess work through a defined backend boundary
- cwd, env injection, timeout, stdout/stderr capture, and cancellation are enforced
- backend behavior is observable and restart-safe where required
- tests cover success, timeout, cancellation, and rejection paths

#### 4. Isolation Controls And Audit Trail

Scope:

- add filesystem and network policy enforcement for the first backend
- add durable audit and failure visibility

Task definition of done:

- sandbox executions record allow/deny outcome, effective policy, and execution trace
- audit trail distinguishes policy rejection from runtime failure
- first backend enforces documented filesystem and network controls
- operator APIs and docs make failures and policy decisions inspectable

### Roadmap Definition Of Done

- daemon has a sandbox control plane separate from individual tools
- at least one backend executes through the sandbox substrate
- sandbox policy covers filesystem, env, timeout, network, and approval concerns
- executions are auditable and inspectable through daemon APIs
- the design is ready to host skills, provider bridges, MCP, and future tool orchestration

### Explicitly Out Of Scope

- browser-grade or VM-grade isolation
- multiple production backends in the same roadmap
- full tool orchestration engine
- context engineering
- memory system

## Roadmap 17: Sandbox Requirement Declarations And Consumer Convergence

Status: `[x] completed`

### Goal

Turn sandbox from an available execution substrate into the declared default boundary for local harness consumers, and close the prerequisite control-plane contracts needed before MCP.

### Tasks

#### 1. Execution Requirement Declaration Contract

Scope:

- define a common requirement model for skills, provider bridges, MCP servers, and future tools
- express filesystem, network, secret, and execution-mode requirements explicitly

Task definition of done:

- daemon has a documented requirement declaration shape that multiple consumer types can share
- requirement declarations can identify required sandbox profile, filesystem scope, network scope, and secret refs
- the declaration model is explicit enough for policy explanation and audit surfaces
- docs and tests cover the new contract boundary

#### 2. Managed Consumer Convergence

Scope:

- close remaining managed-provider local state and credential access paths outside sandbox policy
- remove ad hoc local execution or home-directory access assumptions where still present

Task definition of done:

- managed local consumers no longer rely on hidden subprocess or filesystem access paths outside sandbox truth
- remaining home-directory reads and writes used by managed providers are documented, policy-shaped, and auditable
- failure classification distinguishes sandbox denial from consumer-specific auth or local-state failure
- tests cover success and rejection paths for the converged behavior

Current note:

- the managed-provider convergence slice under `specs/001-sandbox-managed-providers/` is now closed for the in-scope workflows `auth_status`, `logout`, and `prompt_execution`
- provider-owned local state used by Claude and Codex is now declared, policy-shaped, redacted, and auditable for this slice
- the shared declaration vocabulary now also covers the current skill registry and explicit skill-selection surfaces plus the current high-risk local tool-call path
- approval and decision operator surfaces now project durable sandbox provenance for current high-risk local-tool preflight paths, including restart-safe lookup
- MCP lifecycle and generic skill or local-tool subprocess migration stay out of scope for this roadmap and remain follow-on work

#### 3. Secret Scope And Redaction Foundation

Scope:

- define sandbox secret refs and env-injection rules
- define operator-visible redaction behavior for sandbox-backed consumers

Task definition of done:

- secret scope is explicit by consumer and environment
- sandbox env policy can inject secrets by reference instead of uncontrolled inheritance
- config inspection, execution history, and events redact sensitive material consistently
- tests cover secret injection and redaction behavior

Current note:

- secret scope is now persisted as environment-scoped consumer bindings with redacted operator-visible projections
- current managed-provider auth state, config inspection, skill inspection, sandbox execution history, and local-tool approval-gate responses all use the same redaction model

#### 4. Execution Provenance And Consumer Visibility

Scope:

- add provenance fields that identify which consumer requested sandbox work
- expose those fields through APIs, events, and history

Task definition of done:

- sandbox execution history identifies consumer kind and consumer id
- operator inspection can explain which skill, provider bridge, MCP server, or tool initiated an execution
- provenance survives restart and remains queryable through APIs
- tests cover durable provenance behavior

Current note:

- current implementation persists consumer policy records for launched, denied, unsupported, preflight-only, and approval-pending paths
- provenance is queryable today through sandbox execution resources, provider auth state and events, skill inspection surfaces, high-risk tool-call records, and enriched policy approval and decision resources
- restart coverage now verifies that preflight-only local-tool provenance remains inspectable after daemon restart

### Roadmap Definition Of Done

- local harness consumers can declare sandbox requirements explicitly
- managed providers and similar local consumers no longer bypass sandbox policy and audit boundaries in hidden ways
- secret scope, redaction, and execution provenance are explicit enough to support MCP
- the sandbox plane is ready for daemon-managed MCP lifecycle work

### Explicitly Out Of Scope

- full MCP registry and transport lifecycle
- multi-step orchestration engine
- stronger OS-level isolation backend
- memory or context engineering

## Roadmap 18: MCP Execution Plane

Status: `[x] complete`

### Goal

Make MCP a first-class daemon-managed subsystem that executes through sandbox profiles instead of introducing a parallel unmanaged process model.

### Tasks

#### 1. MCP Server Registry And Profile Binding

Scope:

- add MCP server resource model
- bind MCP servers to sandbox profiles and requirement declarations

Task definition of done:

- daemon can register, inspect, and configure MCP servers as first-class resources
- each MCP server resolves to an explicit sandbox profile and requirement declaration
- operator APIs expose effective MCP execution policy and failure state
- tests cover registration, inspection, and invalid configuration paths

#### 2. MCP Transport And Lifecycle Through Sandbox

Scope:

- launch and supervise MCP server processes through sandbox execution
- define restart, cancellation, and shutdown behavior

Task definition of done:

- MCP server startup, restart, and shutdown run through sandbox-managed execution paths
- transport lifecycle failures are classified separately from sandbox denial and launch failure
- daemon recovery behavior for in-flight or managed MCP servers is explicit and tested
- operator APIs surface lifecycle and health state

#### 3. MCP Credential Isolation And Tool Exposure Policy

Scope:

- inject MCP credentials through sandbox secret refs
- define which MCP tools are exposed to which runtime surface

Task definition of done:

- MCP credentials are provided through explicit sandbox env policy rather than uncontrolled process inheritance
- tool exposure policy is inspectable and approval-aware
- operators can explain why an MCP tool is available, blocked, or approval-gated
- tests cover credential isolation, exposure policy, and denial paths

#### 4. MCP Audit And Operator Verification

Scope:

- add MCP-focused docs, events, and verification coverage
- prove sandbox and MCP contracts stay aligned

Task definition of done:

- docs explain MCP server configuration, profile binding, credentials, and failure visibility
- APIs, schemas, events, and tests are aligned for MCP lifecycle and tool exposure
- verification proves MCP runs through sandbox rather than a side path

### Roadmap Definition Of Done

- MCP is a daemon-managed subsystem with explicit registry, policy, and lifecycle
- MCP servers run through sandbox-backed execution instead of unmanaged process launch
- credential injection, tool exposure, and failure visibility are operator-auditable
- restart and recovery behavior is explicit enough for production debugging

Implementation notes:

- daemon exposes `/v1/mcp/servers` registry, lifecycle, tools, and per-tool exposure routes
- stdio MCP sessions are launched through sandbox-backed subprocess execution and recover from persisted enabled-server state on daemon restart
- MCP tool exposure is deny-by-default, explicit per tool and runtime surface, and can be marked approval-required without turning normal server lifecycle into an approval flow
- config inspection, event history, and sandbox consumer provenance now include MCP-visible state
- rollback remains a single change-set revert of the MCP registry, lifecycle, exposure, and contract additions while keeping the underlying sandbox substrate intact

### Explicitly Out Of Scope

- multi-backend MCP placement beyond the first backend profile support
- full orchestration planner across multiple tools
- browser or desktop isolation
- long-term memory integration

## Roadmap 19: Skill And Local Tool Sandbox Execution

Status: `[x] complete`

### Goal

Move real skill and local tool execution onto sandbox so the harness no longer depends on ad hoc local subprocess paths outside the control plane.

### Tasks

#### 1. Skill Requirement Manifest

Scope:

- define how executable skills declare sandbox requirements
- connect skill metadata to sandbox requirement declarations

Task definition of done:

- executable skills can declare required sandbox profile, filesystem access, network access, and secret refs
- skill inspection APIs expose execution requirements clearly
- invalid or unsafe skill requirements remain visible as `unavailable`
- tests cover manifest parsing, approval defaulting, and rejection behavior

#### 2. Local Tool Execution Through Sandbox

Scope:

- route local tool and script execution through sandbox execution requests
- remove remaining ad hoc subprocess launch paths in scope

Task definition of done:

- in-scope local tool execution goes through sandbox-backed execution
- timeout, cancellation, stdout/stderr capture, and approval behavior are consistent across tools
- failure classification distinguishes policy, launch, process, timeout, cancellation, and restart-recovery outcomes
- tests cover success, timeout, denial, cancellation, redaction, and preflight latency paths

#### 3. Runtime Integration And Provenance

Scope:

- attach sandbox-backed tool execution to runtime truth
- surface provenance and audit records through daemon APIs and events

Task definition of done:

- runtime can identify which skill and tool launched each sandbox execution
- tool-call history links to sandbox executions where applicable
- restart and recovery semantics remain explicit and tested
- operator inspection can reconstruct the execution path without reading logs only

#### 4. Operator Docs And Verification

Scope:

- document executable skill and tool behavior
- verify the final sandbox-backed execution path

Task definition of done:

- docs explain how skill execution requirements are declared and enforced
- docs explain tool execution failures, approvals, and audit visibility
- verification proves the supported tool paths no longer bypass sandbox

### Roadmap Definition Of Done

- executable skills and local tools run through sandbox-backed execution
- operators can inspect tool provenance, policy, and failure state through daemon surfaces
- the harness is ready for richer orchestration on top of a single execution boundary

Implementation notes:

- executable skills now project `executionManifest`, `availabilityStatus`, and
  `availabilityReason` through the skill registry routes
- runtime tool calls now carry `invocationKind`, `skillId`, `sandboxExecutionId`, and
  `failureClass`
- approvals, decisions, tool calls, and sandbox executions preserve linked consumer-policy
  and secret-scope provenance
- operator-visible stdout/stderr and sandbox results redact resolved secret values
- daemon restart recovers interrupted in-flight tool executions as `cancelled`

### Explicitly Out Of Scope

- graph planner or multi-step orchestration engine
- additional hardened backends
- memory, context packing, or self-improvement

## Roadmap 20: Stronger Isolation And Additional Sandbox Backends

Status: `[x] complete`

### Goal

Strengthen the sandbox substrate beyond subprocess by adding at least one more isolation-capable backend and explicit backend capability negotiation.

### Tasks

#### 1. Backend Capability Contract

Scope:

- define backend capability metadata
- define how profiles require stronger guarantees

Task definition of done:

- backend capability differences are explicit in contracts and docs
- profiles can express when subprocess is insufficient
- explain and inspection APIs surface capability mismatch clearly
- tests cover capability selection and rejection paths

#### 2. Second Backend Implementation

Scope:

- implement one stronger backend: `docker`
- keep the control-plane contract stable across backends

Task definition of done:

- daemon can execute sandbox work through a second backend without changing the common execution contract
- backend-specific metadata stays isolated from shared execution fields
- success, timeout, cancellation, and failure semantics are covered by tests
- docs state the operational requirements and degradation behavior of the new backend

#### 3. Stronger Filesystem And Network Enforcement

Scope:

- improve enforcement strength beyond subprocess preflight checks
- make enforcement guarantees operator-visible

Task definition of done:

- at least one backend provides materially stronger filesystem or network isolation guarantees than subprocess
- operator APIs and docs explain which guarantees are hard-enforced and which remain declarative
- failure classification distinguishes policy mismatch from backend capability limits
- tests cover enforcement and degradation behavior

#### 4. Backend Selection And Migration Guidance

Scope:

- document when to use each backend
- provide a migration path for higher-risk consumers

Task definition of done:

- operator docs compare backend guarantees, costs, and operational tradeoffs
- consumer migration guidance exists for moving from subprocess to stronger backends
- verification covers at least one real consumer running on the stronger backend

### Roadmap Definition Of Done

- sandbox supports more than one real backend with explicit capability semantics
- higher-risk consumers can require stronger isolation without redesigning the control plane
- backend differences are inspectable, testable, and documented for operators

### Implemented In This Slice

- `/v1/config`, sandbox profile inspection, and sandbox explain now project explicit backend
  capability and host-prerequisite truth for `subprocess` and `docker`
- `docker_default` is available as the first stronger backend through the existing sandbox
  execution plane
- executable skills can opt into `docker` explicitly; unmodified skills remain on
  `subprocess`
- `docker`-required requests fail closed as `unsupported` when the host cannot satisfy
  prerequisites or declared access guarantees
- sandbox execution, runtime tool-call history, approvals, decisions, and events preserve
  backend identity and mismatch classification
- stronger-backend verification covers positive-path execution, unsupported host mismatch,
  timeout/cancellation semantics, restart recovery, and contract surfaces
- real-`docker` executable-skill verification was completed on `zentalk-1`
  (`CentOS Stream 9`, `docker 29.4.0`, `go 1.24.0`) in addition to local negative-path
  and contract coverage

### Remaining Follow-On Work

- migrate more consumer families beyond executable skills
- add stronger backends beyond `docker`
- add VM-grade isolation or remote execution control planes

### Explicitly Out Of Scope

- VM-grade isolation
- fleet-scale remote execution control plane
- full orchestration planner
- memory and self-improvement systems

## Roadmap 21: Complete MCP Runtime And Catalog

Status: `[x] complete`

### Goal

Close the remaining MCP product surface after Roadmap 18 by making MCP tools callable
through the daemon runtime plane, shipping a curated installable catalog, and adding one
real remote transport without creating a second unmanaged MCP control path.

### Tasks

#### 1. MCP Tool Invocation Through Runtime

Scope:

- invoke daemon-managed MCP tools through `/v1/runs/.../tool-calls`
- preserve approval, provenance, failure truth, and history on the existing runtime plane

Task definition of done:

- MCP-originated tool calls project server identity, transport kind, session id, and
  authorization result
- blocked, approval-required, unhealthy-server, timeout, and remote tool errors remain
  explicit
- operator-visible outputs and history remain redacted

#### 2. Bundled Catalog And Install Flows

Scope:

- ship a curated starter catalog
- support daemon API install and repo helper install without diverging resource models

Task definition of done:

- bundled entries can be listed and inspected through daemon routes
- daemon API and script install both converge on one installed MCP server resource shape
- bundled entries surface truthful `ready`, `blocked`, `unavailable`, or `unsupported`
  state with explicit reasons

#### 3. Remote Transport Completion

Scope:

- add `streamable-http` beside stdio
- keep transport health and failure semantics operator-visible

Task definition of done:

- remote MCP sessions initialize and discover tools through the same daemon-owned manager
- session bootstrap has explicit timeout bounds so daemon startup and restore do not hang
- transport-specific failures are distinguishable from local subprocess launch failures

#### 4. Docs And Operator Verification

Scope:

- align docs, routes, schemas, events, and quickstart evidence
- prove real catalog install and invocation behavior in `DOPE_ENV=test`

Task definition of done:

- docs explain starter catalog, transport coverage, blocked paths, and install workflows
- contract and targeted regressions cover MCP invocation, catalog install, and remote
  transport
- manual verification records one real installed-catalog invocation plus one truthful
  blocked or unavailable path

### Roadmap Definition Of Done

- MCP is complete as a daemon product surface: installable, inspectable, startable,
  invocable, and auditable
- MCP invocation remains on the existing runtime tool-call plane
- bundled starter entries remain truthful about host prerequisites, credentials, and
  transport support
- daemon restart and MCP bootstrap stay bounded and debuggable under unresponsive servers

### Implemented In This Slice

- `/v1/mcp/catalog`, `/v1/mcp/catalog/{entryId}`, and
  `/v1/mcp/catalog/{entryId}/install` now expose the bundled starter catalog
- bundled starters include `filesystem`, `Context7`, `GitHub`, `Postgres`, and `Slack`
- repo helper `scripts/install-mcp-catalog-entry.sh` defaults to `DOPE_ENV=test`,
  performs local pairing bootstrap when needed, and writes through the daemon-owned
  install path
- MCP tool invocation now creates `mcp_tool` runtime tool calls with persisted MCP
  provenance fields and redacted operator-visible output
- `streamable-http` is the first remote MCP transport, with explicit `Accept` handling and
  notification semantics compatible with the real Context7 endpoint
- MCP session bootstrap and tool discovery use bounded timeouts so an unresponsive server
  cannot stall daemon startup or restore indefinitely
- manual verification in `DOPE_ENV=test` covered:
  - truthful unavailable install for `filesystem` until a local stdio override is supplied
  - truthful blocked install for `GitHub` without `GITHUB_TOKEN`
  - successful real remote start and tool discovery for `Context7`
  - successful real `Context7` invocation through `/v1/runs/.../tool-calls`

### Explicitly Out Of Scope

- additional MCP remote transports beyond `streamable-http`
- marketplace-scale third-party catalog discovery
- non-MCP integration catalogs
- full multi-tool orchestration planner

## Roadmap 22: MCP Catalog Management And Distribution

Status: `[x] complete`

Detailed spec: [docs/specs/007-mcp-catalog-management-and-distribution.md](../specs/007-mcp-catalog-management-and-distribution.md)

### Goal

Turn the current curated bundled MCP catalog into a managed operator surface with explicit
package lifecycle, source provenance, and post-install maintenance semantics.

### Tasks

#### 1. Catalog Lifecycle Management

Scope:

- add uninstall, reinstall, and explicit update flows for installed catalog entries
- define how operators refresh or replace installed definitions without hand-editing JSON

Task definition of done:

- operators can remove, reinstall, or explicitly refresh installed catalog entries through daemon-owned workflows
- update behavior is fail-closed when an installed resource has operator modifications or conflicting state
- install provenance and rollback semantics remain explicit in API, events, and docs
- implemented via `POST /v1/mcp/servers/{serverId}/refresh`, `/reinstall`, `/uninstall`
  on the existing MCP server resource plane
- active lifecycle or tool-call state now returns explicit `busy`, and locally modified or
  otherwise conflicting resources return explicit `conflict`

#### 2. Source And Version Provenance

Scope:

- persist catalog source identity, version metadata, and install provenance
- prepare the catalog model for future non-bundled sources without introducing them yet

Task definition of done:

- installed MCP resources preserve source kind, source revision or version, and install/update timestamps
- operator inspection can explain where a catalog entry came from and whether it is stale relative to the current catalog payload
- contracts and persistence remain additive to the current MCP registry model
- implemented through additive `catalogManagement` projection on MCP server resources,
  including installed/current revision, drift summary, install snapshot, and last action

#### 3. Health Revalidation And Drift Detection

Scope:

- re-check starter prerequisites after install
- surface config drift or prerequisite drift without requiring a manual start attempt

Task definition of done:

- installed MCP resources can be revalidated on demand or at daemon-defined checkpoints
- operator surfaces distinguish catalog drift, credential loss, binary loss, and transport health failure
- docs explain the difference between catalog freshness, install success, and runtime health
- phase 22 intentionally narrowed this to operator-triggered revalidation only; no daemon
  startup sweep or background loop was introduced
- explicit `POST /v1/mcp/servers/{serverId}/revalidate` now persists ordered issue
  summaries plus a primary classification

#### 4. Docs And Verification

Scope:

- align docs, routes, schemas, events, and quickstart for managed catalog lifecycle
- prove at least one safe uninstall or reinstall path in `DOPE_ENV=test`

Task definition of done:

- docs explain install, update, reinstall, uninstall, and drift semantics
- contract and targeted regressions cover lifecycle transitions and failure truth
- manual verification records one full install-to-remove or install-to-refresh workflow
- verified in `DOPE_ENV=test` on 2026-04-20 with:
  - install -> inspect -> revalidate
  - idle locally modified `refresh -> 409 conflict`
  - idle `uninstall -> completed + removed`
  - active resource `refresh/uninstall -> 409 busy`

### Roadmap Definition Of Done

- MCP catalog management is a first-class daemon product surface rather than a one-shot install helper
- installed MCP resources preserve enough provenance to explain source, version, and drift
- operators can maintain bundled MCP integrations without hand-editing resource definitions
- environment scoping, redaction, and audit history remain additive and restart-safe

### Explicitly Out Of Scope

- third-party marketplace discovery
- signed remote catalog distribution
- non-MCP package ecosystems

## Roadmap 23: Additional MCP Transports

Status: `[x] complete`

Detailed spec: [docs/specs/008-additional-mcp-transports.md](../specs/008-additional-mcp-transports.md)

### Goal

Expand MCP transport support beyond `stdio` and `streamable-http` while preserving one
daemon-owned MCP registry, lifecycle, authorization, and invocation plane.

### Tasks

#### 1. Transport Capability Contract

Scope:

- define transport-specific capability, prerequisite, and health metadata
- make transport selection and unsupported states operator-visible

Task definition of done:

- each supported MCP transport has explicit capability and prerequisite metadata
- operator inspection can explain why a transport is ready, degraded, unsupported, or blocked
- contracts distinguish transport mismatch from server runtime failure

#### 2. Additional Remote Or Session-Based Transport

Scope:

- implement at least one MCP transport beyond `stdio` and `streamable-http`
- keep daemon lifecycle and invocation semantics stable

Task definition of done:

- the new transport initializes, discovers tools, and invokes through the same daemon-owned manager
- session bootstrap and restore remain bounded
- transport-specific failures are explicit and covered by tests

#### 3. Transport Operations And Recovery

Scope:

- define reconnection, retry, cancellation, and restart behavior per transport family
- preserve operator-visible truth during transient remote failures

Task definition of done:

- restart and reconnect semantics are explicit for every supported transport
- daemon recovery does not silently lose MCP session truth across transport families
- events and history distinguish lifecycle recovery from invocation failure

#### 4. Docs And Verification

Scope:

- document supported transport families and host prerequisites
- validate at least one real server on the newly added transport

Task definition of done:

- docs explain transport selection, tradeoffs, and failure modes
- contract and targeted regressions cover transport capability and lifecycle truth
- manual verification records one real end-to-end server on the new transport

### Roadmap Definition Of Done

- MCP supports more than two real transport families without splitting into multiple control planes
- transport readiness, failure, and recovery semantics remain inspectable and auditable
- remote MCP support can expand without weakening the current runtime or policy model

### Implemented In This Slice

- `GET /v1/mcp/transports` and additive `/v1/config.mcp.transports[]` now expose
  operator-visible capability truth for `stdio`, `streamable-http`, and `websocket`
- MCP server create, update, inspection, and runtime tool-call surfaces now support
  `transportKind="websocket"` without introducing a transport-specific control path
- `websocket` auth is explicit and secret-ref-backed through `websocketConfig.auth`; inline
  secret material and anonymous fallback were intentionally not added
- websocket-originated tool calls persist `mcpTransportKind="websocket"` and stay on the
  existing `/v1/runs/.../tool-calls` runtime plane
- daemon-managed websocket reconnect is bounded, persisted, and auditable through
  reconnect-scheduled, reconnect-completed, and reconnect-failed events plus MCP server
  state projection
- restore preserves websocket session and recovery truth across daemon restart instead of
  collapsing reconnect or restore failure into generic lifecycle noise
- manual verification in `DOPE_ENV=test` on 2026-04-21 covered:
  - under-5-minute operator inspection using `GET /v1/mcp/transports`,
    `GET /v1/config`, and `GET /v1/mcp/servers/{id}`
  - truthful blocked websocket server creation when `MCP_WS_TOKEN` is unavailable in test
  - successful real websocket server start, tool discovery, and runtime tool invocation
    through the repo-owned helper server on localhost

### Explicitly Out Of Scope

- marketplace catalog work
- multi-tool orchestration planner
- non-MCP remote execution control planes

## Roadmap 24: Tool-Call Orchestration

Status: `[x] complete`

Detailed spec: [docs/specs/009-tool-call-orchestration.md](../specs/009-tool-call-orchestration.md)

### Goal

Build a policy-aware orchestration layer on top of the unified runtime tool-call plane so
skills, local tools, and MCP tools can participate in ordered or graph-shaped workflows.

### Tasks

#### 1. Planning And Selection Model

Scope:

- define how the daemon plans multi-step tool workflows
- preserve operator-visible rationale, policy gates, and execution boundaries

Task definition of done:

- orchestration decisions are explicit, inspectable, and replayable
- tool selection and ordering can be explained without reading raw logs
- policy and approval requirements remain attached to each concrete execution step
- implemented with first-class workflow resources nested under runs plus additive
  workflow linkage on runtime `Run`, `Step`, and `ToolCall` resources

#### 2. Workflow Execution Semantics

Scope:

- execute ordered or graph-shaped tool workflows through the existing runtime plane
- define retries, cancellation, and partial-failure behavior

Task definition of done:

- orchestrated workflows preserve per-step provenance and terminal truth
- retries and cancellations do not hide already-visible side effects
- partial workflow failure is represented explicitly in runtime history
- daemon restart now marks in-flight workflows `interrupted` while preserving completed
  runtime history and completed workflow-step truth

#### 3. Cross-Consumer Coordination

Scope:

- coordinate MCP tools, local tools, and executable skills in one execution story
- preserve sandbox and approval guarantees across mixed workflows

Task definition of done:

- orchestration can combine at least two consumer families without bypassing existing policy or sandbox boundaries
- cross-tool data flow and handoff remain operator-visible
- docs explain mixed workflow guarantees and limits
- verified with one repo-owned MCP helper plus one executable skill in `DOPE_ENV=test`
  on 2026-04-21

#### 4. Docs And Verification

Scope:

- align docs, schemas, events, and operator workflows for orchestration
- prove at least one real mixed workflow end to end

Task definition of done:

- docs explain orchestration lifecycle, decision visibility, and failure handling
- contract and targeted regressions cover ordered execution, retries, cancellation, and partial failure
- manual verification records one mixed-tool workflow using the daemon-owned runtime plane
- verified on 2026-04-21 with:
  - `make daemon-contract-test`
  - `cd daemon && go test ./internal/api ./internal/store ./internal/runtime ./internal/app ./internal/contracts`
  - `cd daemon && go test ./...`
  - manual `DOPE_ENV=test` workflow planning inspection, blocked-MCP visibility, mixed
    MCP+skill success, and legacy direct-skill regression

### Roadmap Definition Of Done

- the harness can run controlled multi-step tool workflows on one execution boundary
- orchestration decisions are auditable and policy-aware
- mixed MCP, skill, and local-tool workflows do not introduce bypass paths

### Explicitly Out Of Scope

- autonomous self-improvement loops
- memory-driven planning systems
- marketplace or ecosystem discovery surfaces

## Planned Follow-On Roadmaps After Tool-Call Orchestration

The next roadmap split is intentionally non-demo and pre-knowledge-plane. The goal is to
finish the ambient personal-agent product surface before context and memory become the main
investment focus.

### Roadmap 25: Scheduled Tasks And Wakeups

Status: `[ ] planned`

Detailed spec: [docs/specs/010-scheduled-tasks-and-wakeups.md](../specs/010-scheduled-tasks-and-wakeups.md)

Goal: add a durable trigger plane that can wake the daemon and launch normal runs or
workflows through the existing runtime.

### Roadmap 26: Use-Computer Capability Plane

Status: `[ ] planned`

Detailed spec: [docs/specs/011-use-computer-capability-plane.md](../specs/011-use-computer-capability-plane.md)

Goal: add first-class browser-first computer use with approval, artifacts, and workflow
integration on the current runtime plane.

### Roadmap 27: Personal Integrations Platform

Status: `[ ] planned`

Detailed spec: [docs/specs/012-personal-integrations-platform.md](../specs/012-personal-integrations-platform.md)

Goal: define the shared integration substrate for account-backed personal systems before
domain-specific implementations land.

### Roadmap 28: Delivery And Notifications

Status: `[ ] planned`

Detailed spec: [docs/specs/013-delivery-and-notifications.md](../specs/013-delivery-and-notifications.md)

Goal: add a durable delivery plane for background results, alerts, and summaries.

### Roadmap 29: Calendar Integration

Status: `[ ] planned`

Detailed spec: [docs/specs/014-calendar-integration.md](../specs/014-calendar-integration.md)

Goal: add a first production-grade personal calendar domain built on the shared
integration substrate.

### Roadmap 30: Mail Integration

Status: `[ ] planned`

Detailed spec: [docs/specs/015-mail-integration.md](../specs/015-mail-integration.md)

Goal: add a first production-grade personal mail domain with truthful draft and send
semantics.

### Roadmap 31: Tasks And Reminders

Status: `[ ] planned`

Detailed spec: [docs/specs/016-tasks-and-reminders.md](../specs/016-tasks-and-reminders.md)

Goal: add user-facing reminders and lightweight task follow-up on top of the trigger and
delivery planes.

### Roadmap 32: Operator Shell And Onboarding

Status: `[ ] planned`

Detailed spec: [docs/specs/017-operator-shell-and-onboarding.md](../specs/017-operator-shell-and-onboarding.md)

Goal: add the minimum product shell required to configure, inspect, and trust the personal
agent without falling back to raw daemon APIs.

### Roadmap 33: Evaluation And Replay Harness

Status: `[ ] planned`

Detailed spec: [docs/specs/018-evaluation-and-replay-harness.md](../specs/018-evaluation-and-replay-harness.md)

Goal: add replay and comparison support so ambient personal-agent behavior can be changed
without losing confidence.

## Roadmap 13: Provider Streaming Timeout Semantics

Status: `[x] complete`

### Goal

Refactor streaming providers so SSE-style generation is governed by progress-aware timeout semantics instead of a single total-duration timeout.

### Tasks

#### 1. Streaming Timeout Contract

Scope:

- define provider timeout phases
- define config and dispatch contract for streaming timeouts

Task definition of done:

- streaming timeout phases are explicit:
  - connect timeout
  - first-chunk timeout
  - idle timeout
  - optional hard cap
- config and operator docs expose the final semantics
- contract tests cover the shape and classification of timeout outcomes

#### 2. OpenAI-Compatible SSE Transport Refactor

Scope:

- refactor the current openai-compatible SSE transport

Task definition of done:

- first chunk and idle timeout are enforced separately
- progress refreshes the idle deadline
- healthy long-running streams are not terminated only because of total duration
- timeout failures are returned with correct phase-specific classification
- tests cover:
  - no-first-chunk timeout
  - idle timeout
  - long but healthy streaming success
  - optional hard-cap termination if enabled

#### 3. Dispatch And Partial-Result Semantics

Scope:

- represent partial streamed output explicitly in dispatch state

Task definition of done:

- dispatch differentiates:
  - completed
  - failed before output
  - partial then failed
- event payloads and persistence make partial output inspectable
- downstream consumers do not need to infer partial completion from raw logs
- tests cover partial streamed failure after visible output

#### 4. Channel Fallback And Operator Visibility

Scope:

- make channel behavior correct when streamed generation ends after visible progress

Task definition of done:

- IM/channel loop preserves already emitted content
- user-facing behavior is deterministic when the provider stalls or times out mid-stream
- operator-facing events clearly show why the stream ended
- Discord path has regression coverage for partial streamed termination

### Roadmap Definition Of Done

- streaming timeout logic is phase-aware rather than total-duration-only
- openai-compatible SSE transport is safe for long-running healthy streams
- dispatch and connector state correctly represent partial streamed failures
- operator observability is sufficient to tell whether a stream never started, stalled, or was hard-capped
- tests and docs fully describe the final behavior

### Explicitly Out Of Scope

- memory or context engineering
- multi-provider failover
- rich post-processing UX beyond minimal partial-reply correctness

## Roadmap 3: LLM Dispatch Plane

Status: `[x] complete`

### Goal

Make model invocation a first-class daemon subsystem instead of a placeholder.

### Tasks

#### 1. Provider Abstraction

Scope:

- define provider abstraction
- define request and response model

Task definition of done:

- daemon has a concrete provider interface with request/response types
- provider errors and cancellation behavior are explicit
- schema coverage exists for dispatch resources or events as needed
- tests cover provider success and failure paths

#### 2. Usage Accounting

Scope:

- add usage accounting model

Task definition of done:

- requests and responses can record token or usage metadata
- usage data is persisted or durably attached to dispatch history
- tests cover accounting write path

#### 3. Retry And Timeout Policy

Scope:

- add retries and timeout policy

Task definition of done:

- timeout behavior is explicit
- retry policy distinguishes retryable and non-retryable failures
- tests cover timeout, retry, and final failure paths

#### 4. Streaming Integration

Scope:

- add streaming response integration

Task definition of done:

- daemon can stream model output through a stable contract
- streaming cancellation and completion semantics are explicit
- tests cover streamed success and interrupted stream cases

### Roadmap Definition Of Done

- daemon can issue model requests through a real provider abstraction
- usage, retries, timeout, and streaming semantics are explicit
- dispatch contracts are durable enough to support real runtime execution

### Explicitly Out Of Scope

- provider-specific optimization
- long-context memory integration
- advanced tool routing policy

## Roadmap 4: Operator Trust And Security

Status: `[x] complete`

### Goal

Make the daemon operable as a local-first personal agent service with a concrete trust boundary.

### Tasks

#### 1. Local-First Auth And Pairing

Scope:

- add local-first auth model
- add pairing model

Task definition of done:

- daemon exposes a concrete local auth mechanism
- pairing flow is explicit for operator clients
- auth failure behavior is tested
- operator-facing docs explain trust assumptions

#### 2. Approval Enforcement Integration

Scope:

- connect policy approvals to real guarded actions

Task definition of done:

- high-risk actions can require approval before execution
- approval state gates execution rather than existing only as a resource
- tests cover allowed, pending, approved, and rejected flows

#### 3. Approval Durability

Scope:

- persist approvals and decisions if required by final contract

Task definition of done:

- approval and decision state survive daemon restart
- recovery behavior is explicit and tested
- API behavior after restart is stable

### Roadmap Definition Of Done

- daemon has a real local trust boundary
- operator clients can authenticate or pair against that boundary
- approval flows can gate real actions and survive restart if required

### Explicitly Out Of Scope

- org-scale RBAC
- remote multi-tenant control plane

## Roadmap 5: Contract Hardening And Ship Readiness

Status: `[x] complete`

### Goal

Close the remaining contract, migration, and release-readiness gaps that block P0.

### Tasks

#### 1. Schema Enforcement Pipeline

Scope:

- add schema enforcement and generation pipeline

Task definition of done:

- implementation path validates or reliably checks schema conformance
- contract drift is detectable in CI or test workflow
- schema update process is documented

#### 2. Migration And Versioning Plan

Scope:

- add data migration and versioning plan

Task definition of done:

- persisted state has a versioning strategy
- migration steps and rollback expectations are documented
- tests cover at least one migration or version check path

#### 3. Restart And Recovery Coverage Expansion

Scope:

- expand restart and recovery integration coverage beyond runtime

Task definition of done:

- restart coverage includes the major persisted subsystems in scope
- failure modes around replay and partial recovery are tested

#### 4. Release Contract Review

Scope:

- clean remaining API, schema, and documentation drift

Task definition of done:

- critical API/doc/schema mismatches are closed
- daemon can be reviewed as a coherent P0 foundation

### Roadmap Definition Of Done

- persisted contracts are enforceable and evolvable
- restart and migration behavior are defined
- API, schema, and docs are aligned enough for a P0 release decision

### Explicitly Out Of Scope

- advanced memory system
- rich workflow automation
- broad ecosystem integrations

## Roadmap 6: Real Conversation Core

Status: `[x] complete`

### Goal

Make DopeAgent able to serve real single-turn AI replies over daemon APIs once a provider is configured.

This roadmap closes the daemon-side slice only:

- one user query in
- one assistant reply out
- one real configured provider
- one stable chat contract
- no client delivery in this roadmap

### Tasks

#### 1. Real Provider Configuration Model

Scope:

- extend daemon config with provider settings
- define default provider and default model behavior
- define redaction policy for secrets in config inspection

Task definition of done:

- config supports at least one real provider registration path
- provider config can be loaded from file and env overrides
- secrets are never exposed through config inspection APIs
- invalid provider config fails clearly at startup
- tests cover load, override, redaction, and invalid config behavior

#### 2. OpenAI-Compatible Provider Integration

Scope:

- implement one real provider using an OpenAI-compatible chat/completions surface
- use OpenClaw as a reference for provider behavior and request mapping where helpful

Task definition of done:

- daemon can issue real non-echo model requests to a configured upstream provider
- auth, base URL, model, timeout, and error mapping are explicit
- non-stream and stream execution both work through the existing dispatch plane
- retry behavior remains compatible with current dispatcher rules
- tests cover request mapping, auth failure, upstream failure, and streamed success

#### 3. Minimal Conversation API Contract

Scope:

- add a single-turn conversation contract above raw LLM dispatch
- keep daemon stateless with respect to conversation history

Task definition of done:

- daemon exposes a minimal chat query route and a streaming variant
- request shape is query-first, not full runtime/run-step oriented
- response shape is schema-backed and stable enough for UI clients
- implementation is explicit that the daemon does not retain multi-turn conversation state
- tests cover success, provider error, auth error, and streaming completion

#### 4. End-To-End Verification And Operator Docs

Scope:

- add real-provider smoke coverage
- document operator setup and failure modes

Task definition of done:

- there is a documented setup path from empty config to first successful reply
- there is a documented failure path for bad key, bad endpoint, and missing model
- end-to-end verification proves one real configured provider can serve both HTTP and streaming chat paths
- rollback or disable path is documented if provider integration must be turned off

### Roadmap Definition Of Done

- daemon can talk to at least one real configured provider
- the chat contract is explicit, schema-backed, and independent from future memory/context design
- one operator can reach a successful single-turn reply through daemon HTTP APIs alone
- operator setup, error handling, and verification are documented well enough to support client development

### Explicitly Out Of Scope

- Web UI delivery
- TUI delivery
- multi-turn conversation state in daemon
- memory or context engineering
- tool use orchestration during chat
- provider marketplace or multi-provider routing policy

## Roadmap 7: Minimal Chat Clients

Status: `[x] complete`

### Goal

Make the configured daemon usable from both a minimal Web UI and a minimal TUI for single-turn chat.

This roadmap depends on Roadmap 6 being complete first.

### Tasks

#### 1. Shared Client Contract Adoption

Scope:

- use the daemon chat contract as the only conversation surface for both clients

Task definition of done:

- Web and TUI both use the same daemon chat routes
- neither client introduces hidden provider-specific request logic
- neither client depends on daemon-side multi-turn state
- smoke verification proves contract parity across both clients

#### 2. Web Chat Surface

Scope:

- add a minimal Web UI for single-turn chat

Task definition of done:

- operator can enter a query and receive a real assistant response
- loading, error, and retry states are visible
- provider configuration assumptions are documented in the UI or operator docs
- the Web UI does not depend on memory or context subsystems
- tests or smoke verification cover the basic chat path

#### 3. TUI Chat Surface

Scope:

- add a minimal TUI for single-turn chat

Task definition of done:

- operator can enter a query and receive a real assistant response from terminal
- loading, error, and retry states are visible
- TUI uses the same daemon chat contract as the Web UI
- the TUI does not carry hidden multi-turn state in daemon
- tests or smoke verification cover the basic TUI chat path

#### 4. Cross-Client Verification

Scope:

- verify operator setup and failure behavior across both client surfaces

Task definition of done:

- one configured daemon can serve both Web and TUI without client-specific backend switches
- auth failure, provider failure, and streaming failure are visible from both clients
- operator docs cover how to start each client against the daemon

### Roadmap Definition Of Done

- Web UI and TUI can both issue a single-turn query and render the reply
- both clients rely on the same daemon chat contract
- client behavior is documented and smoke-verified

### Explicitly Out Of Scope

- multi-turn conversation state
- memory or context engineering
- tool use orchestration during chat
- rich conversation UX beyond minimal operator flow

## Roadmap 8: Ingress Routing Closure

Status: `[x] complete`

### Goal

Close the remaining ingress and routing gap so external connector input can route into sessions and create session-bound runs without relying on local-only fallback semantics.

### Tasks

#### 1. Route-Aware Run Creation

Scope:

- allow `POST /v1/runs` to resolve a session from explicit route input

Task definition of done:

- `POST /v1/runs` accepts either `sessionId`, explicit `route`, or the existing local fallback
- ambiguous requests are rejected consistently
- route-driven runs bind to the resolved session and persist correctly
- API and contract tests cover explicit route-based run creation

#### 2. Connector Ingress Contract

Scope:

- add an explicit connector ingress API for inbound messages

Task definition of done:

- daemon exposes a connector ingress route for inbound messages
- ingress resolves or creates the correct session from connector/channel identity
- ingress can optionally create a run bound to that session
- connector status gating is explicit and tested
- request and response shapes are schema-backed

#### 3. Ingress Event And Recovery Closure

Scope:

- make ingress behavior durable and restart-safe

Task definition of done:

- session and run state created through ingress are persisted
- ingress emits explicit connector/session/run events
- restart recovery restores ingress-created session and run state
- tests cover persistence and restart behavior for ingress-created state

#### 4. Contract And Documentation Closure

Scope:

- close schema, event, and operator documentation for ingress routing

Task definition of done:

- request/response schemas exist for ingress routes
- event schemas exist for ingress-specific events
- contract tests validate ingress responses and events
- operator docs explain how connector ingress maps to session/run state

### Roadmap Definition Of Done

- inbound connector traffic has a first-class daemon API
- runs can be bound to sessions from explicit route data, not only local fallback
- ingress-created session and run state are durable and restart-safe
- routing behavior is documented, schema-backed, and fully tested

### Explicitly Out Of Scope

- real external connector transport implementations
- message queueing or async ingress buffering
- automatic reply generation from inbound connector traffic

## Roadmap 9: Provider Identity And Profiles

Status: `[x] complete`

### Goal

Turn provider handling from a hidden bootstrap detail into an operator-visible, testable, daemon-managed profile system.

The daemon already has:

- provider abstraction
- one real `OpenAI-compatible` provider
- query-first chat APIs

What it does **not** have yet is a first-class provider profile surface. Today, providers are mostly implicit inside config and dispatch behavior.

### Tasks

#### 1. Provider Profile Resource Model

Scope:

- define provider profile resources
- distinguish provider family, auth mode, and profile identity

Task definition of done:

- daemon has a provider profile resource with stable IDs
- provider family and auth mode are explicit fields, not hidden conventions
- at least API-key based provider profiles can be represented
- responses expose effective metadata without leaking secrets
- schemas and tests cover resource shape

#### 2. Provider Inventory And Introspection

Scope:

- add provider list/get APIs

Task definition of done:

- daemon exposes list and get routes for provider profiles
- responses include provider family, auth mode, effective base URL, default model, timeout, and capability flags
- responses do not leak secrets
- schemas and API tests cover list/get behavior

#### 3. Provider Preflight And Health Checks

Scope:

- add explicit provider check routes

Task definition of done:

- operator can trigger a provider check through daemon API
- check verifies config completeness and a real upstream request path
- success and failure results are durable and queryable
- failure taxonomy distinguishes config error, auth error, transport error, upstream error, and timeout
- tests cover healthy and failing checks

#### 4. Provider Resolution And Override Policy

Scope:

- define how daemon resolves provider/model/defaults for dispatch and chat

Task definition of done:

- resolution order is explicit for request override, configured defaults, profile defaults, and provider defaults
- validation failures are consistent across dispatch and chat routes
- incompatible provider/model requests fail early
- docs and tests cover effective resolution behavior

#### 5. Provider Contracts And Operator Docs

Scope:

- add schemas, event coverage, and operator-facing docs for the provider plane

Task definition of done:

- provider profile request/response schemas exist
- provider check events exist and are validated
- contract tests cover provider APIs and events
- operator docs explain how to configure, inspect, and verify providers without ad hoc curl

### Roadmap Definition Of Done

- provider profiles are first-class daemon resources
- providers are visible through daemon APIs
- operators can inspect effective provider configuration safely
- operators can run a daemon-managed provider check and understand the failure class
- provider resolution behavior is explicit, tested, and documented

### Explicitly Out Of Scope

- Claude login flow
- Codex / ChatGPT login flow
- multi-provider routing or fallback policy
- provider marketplace or remote profile registry

## Roadmap 10: Managed Coding Providers

Status: `[x] complete`

### Goal

Add the first managed login providers so users can authenticate to coding plans without having to provide `baseURL + apiKey`.

### Tasks

#### 1. Managed Provider Auth Surface

Scope:

- define managed provider login/logout/refresh flows
- support at least provider-owned session or token style auth

Task definition of done:

- daemon exposes login-state APIs for managed providers
- operators can start, complete, inspect, and revoke managed-provider auth state
- auth state is durable and restart-safe
- tests cover success, invalid auth state, and expiry or refresh failure

#### 2. Claude Managed Provider

Scope:

- add first-class Claude provider integration

Task definition of done:

- Claude can be configured through managed login rather than only raw API config
- daemon can inspect Claude model availability through the provider profile system
- daemon can dispatch single-turn requests through Claude with the same high-level contract
- tests cover auth and dispatch behavior

#### 3. Codex Or ChatGPT Managed Provider

Scope:

- add first-class Codex or ChatGPT-style managed provider if technically viable

Task definition of done:

- operator can authenticate without `baseURL + apiKey`
- daemon can expose model selection and default model behavior
- daemon can dispatch through the provider using the same high-level contract
- tests cover auth and dispatch behavior

#### 4. Model Catalog And Compatibility Metadata

Scope:

- expose provider model catalogs and capability metadata

Task definition of done:

- daemon can list models per provider profile
- model metadata includes enough operator-facing compatibility information
- operator can set profile default model and request overrides safely
- docs and tests cover model selection behavior

### Roadmap Definition Of Done

- Claude and Codex managed providers can be inspected without `baseURL + apiKey`
- daemon can dispatch through managed provider auth
- model selection is visible and configurable for managed providers
- operator flows for login, inspect, and verify are documented and tested

### Explicitly Out Of Scope

- every provider family in the market
- automatic multi-provider fallback
- billing optimization or smart routing

## Roadmap 11: First IM Channel Loop

Status: `[x] complete`

### Goal

Close one real IM channel end to end so DopeAgent can receive a user message, route it through daemon truth, invoke the configured provider, and send the reply back through the same channel.

### Tasks

#### 1. IM Connector Runtime Contract

Scope:

- define the daemon-facing contract for a real IM connector runtime
- define inbound message normalization and outbound reply dispatch
- define connector auth/config shape for the first IM channel

Task definition of done:

- connector config is explicit and schema-backed
- connector readiness and failure state are supervised through daemon connector APIs
- inbound and outbound connector message envelopes are explicit and durable
- connector auth/config redaction is covered by config inspection rules
- tests cover config parsing and connector contract behavior

#### 2. Discord Connector Implementation

Scope:

- implement the first real IM connector using the official Discord bot API
- support one operator-realistic delivery path

Task definition of done:

- daemon can configure Discord bot token and delivery mode
- connector can receive inbound Discord messages through the chosen delivery path
- connector can send outbound text replies back to Discord
- auth and transport failures are classified explicitly
- tests cover inbound normalization, outbound request shaping, and failure mapping

#### 3. Inbound-To-Reply Execution Loop

Scope:

- close the runtime loop from inbound IM message to assistant reply
- keep the first version single-turn and stateless on the model side

Task definition of done:

- an inbound Discord message resolves or creates the correct session
- daemon creates the corresponding run and step execution path
- daemon invokes the configured provider through the existing chat contract
- daemon sends the reply back through the Discord connector
- duplicate delivery handling prevents obvious double-reply bugs
- tests cover at least one full success path and one failure path

#### 4. IM Operator Docs And End-To-End Verification

Scope:

- document how to configure and operate the first IM channel
- prove the loop works through the real connector boundary and daemon runtime

Task definition of done:

- operator docs explain token config, gateway mode, mention and DM behavior, and failure visibility
- daemon APIs, schemas, and events are aligned for the new channel
- verification covers connector runtime, transport shaping, and message-in to reply-out runtime behavior
- roadmap notes clearly state what remains out of scope

### Roadmap Definition Of Done

- DopeAgent can carry one real IM conversation turn through a real connector
- the connector is supervised, configurable, and observable through daemon APIs
- inbound routing and outbound reply behavior are durable enough for operator use
- docs, schemas, and tests are aligned with the actual IM loop

### Explicitly Out Of Scope

- multiple IM channels in the same roadmap
- unofficial or reverse-engineered IM transports
- long-term memory or context assembly for IM
- rich media, files, reactions, voice, or typing indicators

## Roadmap 12: Channel Reply Progression

Status: `[x] complete`

### Goal

Define a channel capability model for reply progression and close the first enhanced UX slice where a real IM channel can show thinking state and incremental output when the channel supports it.

### Tasks

#### 1. Channel Capability And Degradation Model

Scope:

- define how channels declare support for thinking and incremental output
- define fallback behavior for channels that do not support those capabilities

Task definition of done:

- a channel capability model exists in committed docs
- the model explicitly separates `thinking` support from incremental output support
- fallback behavior is defined for `thinking + streaming`, `thinking + final`, and `final_only`
- implementation boundary matches the documented model

#### 2. Streaming-Capable Reply Progression In Daemon Runtime

Scope:

- add a daemon-owned reply progression path that can consume provider stream output and drive channel updates

Task definition of done:

- daemon can keep final-only behavior for channels without progression support
- daemon can use a streaming-capable path when the channel declares incremental output support
- progression remains inside daemon routing, runtime, and persistence truth
- tests cover the progression path

#### 3. Discord Thinking And Streaming UX

Scope:

- implement Discord-specific thinking display and incremental reply updates

Task definition of done:

- Discord can emit thinking state through its supported mechanism
- Discord can send an initial reply and update the same reply incrementally
- update frequency is throttled enough to avoid obviously unsafe edit spam
- transport/auth failures are still classified explicitly
- tests cover thinking, initial reply send, and reply edit behavior

#### 4. Operator Docs And Contract Closure

Scope:

- document the channel reply progression model and the Discord-specific behavior

Task definition of done:

- docs explain capability analysis per channel
- docs explain Discord thinking and streaming behavior
- roadmap and task state reflect the real implementation boundary
- full daemon test suite and contract tests pass

### Roadmap Definition Of Done

- reply progression is modeled as channel capabilities, not as a universal requirement
- channels that support thinking and incremental output can expose those phases through daemon-owned logic
- channels that do not support those phases still fall back cleanly to final-only replies
- Discord has a production-shaped implementation for thinking and incremental output

### Explicitly Out Of Scope

- making every channel support streaming
- token-level real-time updates without throttling
- multi-turn chat memory or context engineering
- rich media progression behavior

## Recommended Order

1. Roadmap 1: Runtime Closure
2. Roadmap 2: Supervision Plane
3. Roadmap 3: LLM Dispatch Plane
4. Roadmap 4: Operator Trust And Security
5. Roadmap 5: Contract Hardening And Ship Readiness
6. Roadmap 6: Real Conversation Core
7. Roadmap 7: Minimal Chat Clients
8. Roadmap 8: Ingress Routing Closure
9. Roadmap 9: Provider Identity And Profiles
10. Roadmap 10: Managed Coding Providers
11. Roadmap 11: First IM Channel Loop
12. Roadmap 12: Channel Reply Progression
13. Roadmap 13: Provider Streaming Timeout Semantics
14. Roadmap 14: Test Environment Workflow
15. Roadmap 15: Skill Registry And Prompt Support
16. Roadmap 16: Sandbox Execution Plane
17. Roadmap 17: Sandbox Requirement Declarations And Consumer Convergence
18. Roadmap 18: MCP Execution Plane
19. Roadmap 19: Skill And Local Tool Sandbox Execution
20. Roadmap 20: Stronger Isolation And Additional Sandbox Backends
21. Roadmap 21: Complete MCP Runtime And Catalog
22. Roadmap 22: MCP Catalog Management And Distribution
23. Roadmap 23: Additional MCP Transports
24. Roadmap 24: Tool-Call Orchestration
25. Roadmap 25: Scheduled Tasks And Wakeups
26. Roadmap 26: Use-Computer Capability Plane
27. Roadmap 27: Personal Integrations Platform
28. Roadmap 28: Delivery And Notifications
29. Roadmap 29: Calendar Integration
30. Roadmap 30: Mail Integration
31. Roadmap 31: Tasks And Reminders
32. Roadmap 32: Operator Shell And Onboarding
33. Roadmap 33: Evaluation And Replay Harness
