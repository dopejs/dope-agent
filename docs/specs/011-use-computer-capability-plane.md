# Use-Computer Capability Plane

Status: proposed

Authority: This document is the authoritative upstream spec for first-class computer-use capability work after tool-call orchestration.

Primary source documents:
- `docs/product/personal-agent-non-knowledge-roadmap-split.md` (removed 2026-08, in git history)
- `docs/runtime/operator-trust-model.md`
- `docs/runtime/daemon-scope.md`

## Background

Personal agents need a controlled way to operate browser or desktop surfaces that are not already covered by MCP or document integrations. Current `browser` capability handling is only a policy placeholder, not a product-complete computer-use surface.

## Goal

Introduce a first-class computer-use capability plane that can be scheduled, orchestrated, inspected, and approved through the existing daemon runtime.

## Fixed Decisions

- use-computer stays a capability family on the current runtime plane
- approval, sandbox, provenance, and artifacts remain attached to each concrete action
- browser-first support is in scope before generalized desktop automation
- screenshots, DOM extracts, and other evidence become first-class artifacts

## In Scope

- browser session lifecycle
- page snapshot and screenshot artifacts
- controlled navigation and action model
- workflow integration
- approval and safety model

## Out Of Scope

- full remote desktop stack
- mobile-device automation
- memory-driven UI exploration

## User Stories

- As an operator, I can inspect what page or target the agent is about to act on before high-risk actions execute.
- As an operator, I can see screenshots or other artifacts that explain what happened during computer use.
- As an operator, I can run computer-use steps inside a normal workflow without bypassing policy or audit boundaries.

## Functional Requirements

- the daemon MUST expose a first-class computer-use capability surface rather than treating browser control as an opaque local tool
- computer-use executions MUST create normal runtime steps and tool calls with additive page or session linkage
- high-risk computer-use actions MUST preserve approval and artifact-backed audit truth
- screenshots, snapshots, and downloads MUST be projected as artifacts or equivalent operator-visible outputs
- computer-use failure classes MUST distinguish policy denial, unavailable consumer, navigation failure, and target mismatch

## Compatibility And Operational Notes

- existing skill and MCP execution paths remain unchanged
- orchestration should be able to include computer-use steps beside MCP and skills
- stronger isolation backends may later be reused here without changing the external resource model

## Verification Expectations

- targeted capability and API tests
- contract coverage for new computer-use resources or artifacts
- one manual browser-based `DOPE_ENV=test` verification path

## Definition Of Done

- browser-first computer use is inspectable, policy-aware, and artifact-backed
- workflows can include computer-use steps without hidden side paths
- operator-facing truth makes actions and side effects understandable after the fact

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/011-use-computer-capability-plane.md 完成 phase 26 的工作`
