# P0 Module Map

## Purpose

This document defines the primary module boundaries for P0.

It is derived from:

- the P0 feature scope
- the decision to use `Kura: Go daemon + TS clients`
- the requirement that the daemon is a T0 long-lived process

The main goal is to decide:

- what belongs inside the daemon
- what belongs outside the daemon
- what should be a client
- what should be a supervised capability process

## Top-Level System Shape

P0 should be shaped as four major zones:

1. daemon core
2. operator clients
3. capability processes
4. shared contracts and storage

Expressed simply:

- `daemon` is the system spine
- `clients` talk to the daemon
- `capabilities` are supervised by the daemon
- `schemas` define the boundaries

## 1. Daemon Core

The daemon core is the T0 process and system of record.

It should be implemented in Go.

The daemon core should contain the following P0 modules.

### 1.1 Process And Bootstrap

Responsibilities:

- process startup and shutdown
- configuration load and validation
- background service registration
- signal handling
- version and health reporting

Why it belongs here:

- this is the root lifecycle owner for the whole system

### 1.2 API And Control Plane

Responsibilities:

- HTTP API
- WebSocket or streaming event transport
- auth and operator sessions
- pairing and local trust decisions
- API surface for clients and automation

Why it belongs here:

- clients must have one stable control-plane entry point
- this is part of the daemon's T0 responsibility

### 1.3 Session Router

Responsibilities:

- inbound message normalization
- channel-to-session routing
- direct-message and group isolation
- agent or workspace selection
- session lifecycle policies

Why it belongs here:

- routing must be authoritative and durable
- it cannot be delegated to UI or channel plugins ad hoc

### 1.4 Runtime Core

Responsibilities:

- run lifecycle
- task and step lifecycle
- event append flow
- cancellation and resume
- checkpoint triggers
- runtime state transitions

Why it belongs here:

- this is the execution heart of the system
- it must remain inside the T0 daemon

### 1.5 Policy Engine

Responsibilities:

- permission checks
- approval gates
- capability allow and deny decisions
- escalation hooks
- enforcement of runtime constraints

Why it belongs here:

- policy decisions must be made by the authoritative process

### 1.6 LLM Dispatch

Responsibilities:

- provider selection
- request shaping
- retries and timeouts
- streaming response handling
- model usage accounting

Why it belongs here:

- LLM orchestration affects run state directly
- request control must be centrally enforced

### 1.7 Connector Supervisor

Responsibilities:

- start and stop IM connectors
- track connector health
- reconnect and backoff
- isolate connector failures

Why it belongs here:

- connector state is part of daemon operational state
- supervision should not live inside clients

### 1.8 Capability Supervisor

Responsibilities:

- spawn capability processes
- heartbeat and restart policies
- health and readiness checks
- capability registration and availability

Why it belongs here:

- external capabilities must be explicitly supervised by the daemon

### 1.9 State Store Access Layer

Responsibilities:

- persistence for sessions, runs, steps, events, checkpoints, schedules, and config metadata
- artifact metadata tracking
- transaction boundaries for daemon state changes

Why it belongs here:

- the daemon owns system truth and should own writes

## 2. Operator Clients

Operator clients are not part of the system spine.

They are replaceable surfaces that consume daemon APIs.

## 2.1 Web UI

Recommended stack:

- TypeScript
- React
- Vite

Responsibilities:

- chat and operator dashboard
- session inspection
- run and task visibility
- config editing
- plugin and capability management
- diagnostics and logs view

Hard boundary:

- must not own runtime truth
- must not bypass daemon state transitions

## 2.2 TUI

Responsibilities:

- terminal operator console
- command and inspection workflows
- lightweight dashboard flows

Hard boundary:

- must call daemon APIs
- must not be coupled to daemon internals

Open question for later:

- Go-native TUI or TypeScript client

## 2.3 Automation Clients

Examples:

- local scripts
- future CLI helpers
- remote automation

Responsibilities:

- invoke daemon APIs
- subscribe to events
- trigger workflows

Hard boundary:

- same contract as other clients
- no hidden privileged backchannel

## 3. Capability Processes

Capability processes are outside the daemon but under daemon supervision.

These should be used when a feature:

- has weaker reliability
- depends on fragile SDKs
- needs stronger isolation
- may crash or hang independently
- has large dependency weight

## 3.1 Browser Capability

Responsibilities:

- browser automation
- page navigation
- DOM extraction
- screenshots or snapshots

Why separate:

- browser stacks are heavy and failure-prone
- they should not destabilize the daemon

## 3.2 IM Connectors With Fragile SDKs

Examples:

- connectors that depend on unofficial web protocol stacks
- connectors with large Node ecosystems

Why separate:

- isolate churn and reconnect storms
- allow language choice per connector if needed

## 3.3 Media And Voice Workers

Responsibilities:

- speech
- transcription
- audio pipelines
- camera and media operations

Why separate:

- media pipelines are operationally different from the daemon core

## 3.4 Future Memory Or Indexing Workers

Not a P0 priority, but should already fit the same boundary.

Why separate:

- long-running indexing or retrieval jobs should not contend with the daemon spine

## 4. Shared Contracts And Storage

These modules are not user-facing but are foundational.

## 4.1 Schemas

Responsibilities:

- daemon API contracts
- event contracts
- capability RPC contracts
- plugin manifests
- config schemas

Why separate:

- all boundaries should depend on shared contracts rather than informal JSON

## 4.2 Storage Model

Responsibilities:

- define durable entities
- define artifact paths and metadata rules
- define checkpoint serialization shape

Why separate:

- storage concerns should be explicit and versionable

## P0 Module Inventory

The practical P0 module list is:

- `daemon/process`
- `daemon/config`
- `daemon/api`
- `daemon/router`
- `daemon/runtime`
- `daemon/policy`
- `daemon/llm`
- `daemon/connectors`
- `daemon/capabilities`
- `daemon/store`
- `web`
- `tui`
- `schemas`
- `capabilities/browser`
- `capabilities/connectors/*`

This is a module map, not yet a repo map. Several of these may live in the same repository package at first.

## Ownership Boundaries

## Daemon Owns

- session truth
- run truth
- policy truth
- connector truth
- capability liveness truth
- checkpoint truth

## Clients Own

- rendering
- interaction UX
- local view state
- user input composition

## Capability Processes Own

- tool-specific execution environments
- heavy or unstable third-party dependencies
- isolated failure domains

## What Must Not Happen

- clients mutating daemon state outside the daemon API
- connectors inventing routing semantics locally
- capability processes becoming hidden runtime coordinators
- workspace files becoming the only source of operational truth
- direct coupling between Web UI components and runtime internals

## Immediate Next Design Work

This module map should feed the next documents:

- daemon API and event model
- deployment model
- config model
- storage model

## Short Version

P0 should center around a Go daemon that owns system truth.

Web UI and TUI are replaceable clients.

Heavy or fragile integrations should run as supervised capability processes.

All boundaries should be schema-defined and explicit.
