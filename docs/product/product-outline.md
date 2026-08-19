# Product Outline

## Positioning

Kura is a personal agent operating system rather than a single-purpose coding agent.

It should support:

- always-on personal assistance
- long-lived sessions and identity
- tool use across communication, documents, browser, files, and code
- domain-specialized behaviors such as coding, debugging, review, and architecture design

Coding is a major capability domain, but not the product boundary.

## Product Thesis

Most current agent products are prompt shells with lightweight memory. They can feel capable in a single session, but become unreliable over time because they lack:

- structured runtime state
- layered memory with retention policy
- stable handoff protocols
- replay and audit
- explicit separation between host shell and agent core

Kura should instead be built as:

- a host shell and control plane
- a runtime plane for runs, steps, and checkpoints
- a memory plane with multiple memory types
- a policy plane for permissions and write guards
- a capability plane for domain-specific agents and tools

## Primary Design Goals

- Make long-running, cross-session behavior reliable.
- Prevent memory pollution and silent context drift.
- Support multiple personas, projects, and domains without state leakage.
- Keep the system inspectable by engineers under failure.
- Make coding and architecture work first-class capability domains.

## Non-Goals For V1

- fully autonomous self-directed behavior without user or policy control
- a general-purpose knowledge graph as the primary storage model
- end-to-end replacement of all OpenClaw product surfaces
- self-modifying agent behavior without review or audit

## Core Principles

- Control plane decisions should be deterministic where possible.
- The model can propose actions, but the runtime and policy layer should decide execution.
- Memory writes must be attributable, scoped, and reversible.
- Recalled memory is evidence, not truth. Important decisions must be able to link back to source.
- Every long-lived workflow must be resumable from checkpoints.
