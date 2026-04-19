# Feature Phasing Against OpenClaw

## Purpose

This document organizes the OpenClaw comparison into phased product targets.

The goal is:

- first match the right capability surface
- avoid prematurely copying OpenClaw's internal architecture
- leave room for a different runtime, storage model, and system boundary

This is a feature benchmark, not an implementation commitment.

## Principle

We are not required to use OpenClaw's architecture to achieve OpenClaw-like functionality.

Specifically:

- matching feature scope does not mean matching runtime structure
- matching user-facing surfaces does not mean matching prompt, workspace, or memory design
- matching plugin or channel coverage does not mean matching protocol internals

OpenClaw is the comparison target for product completeness. Our architecture may diverge significantly if it yields a cleaner daemon, runtime, context system, or memory system.

## P0: Required To Be In The Same Product Category

P0 defines the minimum feature set required for DopeAgent to count as a serious OpenClaw-class personal agent daemon.

### 1. Daemon And Gateway Core

- long-running background daemon
- local or server deployment
- config loading and reload
- auth, pairing, and operator access
- health, logs, and basic diagnostics

### 2. Primary User Surfaces

- Web UI
- TUI
- at least one chat connector
- a unified daemon API behind all surfaces

### 3. Session And Routing Model

- direct-message sessions
- group or room isolation
- routing by source or channel
- basic multi-agent or per-workspace isolation
- reset and lifecycle handling for sessions

### 4. Core Agent Runtime

- task or run lifecycle
- tool invocation framework
- model invocation and streaming
- policy and permission gates
- observable event flow

### 5. Workspace And Persona Layer

- agent identity and operator preferences
- editable workspace or config overlays
- bootstrap flow for first-run personalization

### 6. Tools And Capability Hosting

- shell or local command execution
- files and workspace access
- a minimal browser or web capability
- extensible tool registration model

### 7. Extensibility Baseline

- plugin system or equivalent extension mechanism
- skill or capability packaging model
- config-driven enablement and isolation

### 8. Baseline Context And Memory

- session context assembly
- context window management
- minimal durable memory or persistence layer

This does not require OpenClaw-style markdown memory. It only requires that the product has durable continuity and bounded context behavior.

## P1: Strong Product Parity

P1 brings DopeAgent closer to OpenClaw's broader platform value.

### 1. Richer Channel Coverage

- multiple built-in chat connectors
- plugin channels
- consistent routing and identity handling across channels

### 2. Advanced Operations And Automation

- cron jobs
- webhooks
- scheduled wakeups or background triggers
- richer admin and diagnostics workflows

### 3. Node And Device Capabilities

- external or mobile nodes
- device capability registration
- camera, screen, location, or notification actions

### 4. Stronger Multi-Agent Support

- agent bindings per channel or account
- per-agent workspaces
- per-agent capability visibility
- safer cross-agent isolation

### 5. Better Product Management Surface

- dashboard controls for channels, sessions, config, and plugins
- config patch and apply flow
- restart and health visibility

### 6. Better Context And Memory Internals

- inspectable context breakdown
- replaceable context engine
- replaceable memory engine
- replay-aware persistence boundaries

## P2: Ecosystem And Ambient Agent Features

P2 covers the features that make the platform feel mature, ambient, and ecosystem-rich rather than merely functional.

### 1. Mobile And Ambient Interaction

- mobile companion apps or nodes
- voice wake
- talk mode
- continuous conversational workflows

### 2. Visual Workspace Features

- Canvas or equivalent live visual surface
- agent-driven UI surfaces
- snapshot and control workflows

### 3. Broad Plugin Ecosystem

- plugin marketplace or registry
- community channel adapters
- domain plugins for providers, voice, search, and media

### 4. Polished Onboarding And Remote Access

- guided onboarding
- local and remote daemon setup
- remote control access patterns
- secure device approval flows

### 5. Advanced Memory And Knowledge Layers

- semantic memory backends
- promotion pipelines
- reflective consolidation
- provenance-rich knowledge views

## What This Means For Design

P0 should guide the first architecture decisions.

That means the first design pass should focus on:

- daemon lifecycle
- operator API
- session and routing model
- runtime core
- tool host
- plugin boundary
- primary user surfaces

Memory and context should be designed as first-class future modules, but not as the first implementation focus.

## What This Does Not Mean

This phased benchmark does not imply that we should:

- copy OpenClaw's workspace-file contract
- copy its WebSocket protocol
- copy its context-engine lifecycle
- copy its markdown-first memory design
- copy its plugin packaging model

Those are implementation choices, not product requirements.

## Immediate Next Use

Use this document as input to the next three design documents:

- product scope
- module map
- deployment and technical stack
