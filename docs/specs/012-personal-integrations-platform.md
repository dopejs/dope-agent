# Personal Integrations Platform

Status: proposed

Authority: This document is the authoritative upstream spec for the shared integration substrate that calendar, mail, and reminder domains should build on.

Primary source documents:
- `docs/product/personal-agent-non-knowledge-roadmap-split.md`
- `docs/product/openclaw-architecture-gaps.md`
- `docs/runtime/operator-trust-model.md`

## Background

Personal-agent value comes from controlled access to the user's external systems. Calendar, mail, reminders, files, and similar domains should not each invent their own connection, approval, history, and delivery model.

## Goal

Define the shared integration-layer contract for user-account-backed personal systems before domain-specific integrations are implemented.

## Fixed Decisions

- integrations remain daemon-managed resources with explicit identity, auth, health, and scope
- domain implementations reuse one run, workflow, approval, event, and artifact model
- integrations can be implemented through MCP, managed providers, or native capability layers, but their operator surface should converge

## In Scope

- integration resource model
- account binding and readiness truth
- domain-agnostic auth and health semantics
- event and artifact conventions
- environment and secret-scope rules

## Out Of Scope

- domain-specific calendar or mail behavior details
- marketplace discovery
- multi-user tenancy

## User Stories

- As an operator, I can tell which personal systems are connected, healthy, and available to workflows.
- As an operator, I can inspect integration provenance and auth readiness without reading raw config files.
- As an operator, I can build domain-specific features on one shared integration model.

## Functional Requirements

- the daemon MUST expose first-class integration resources with account identity, status, health, and environment scope
- integration resources MUST distinguish not configured, auth pending, healthy, degraded, and unavailable states
- integration-backed executions MUST preserve approval, provenance, and redacted secret-scope truth
- domain implementations MUST reuse shared integration identity and readiness semantics rather than redefining them

## Compatibility And Operational Notes

- integration resources may front MCP servers, managed providers, or future native connectors
- operator-visible surfaces should converge even when implementation backends differ
- delivery and notification behavior should integrate with, but remain separate from, integration readiness

## Verification Expectations

- contract coverage for shared integration resources
- health and auth-state regressions
- one repo-owned fake or local integration fixture path

## Definition Of Done

- the shared integration substrate exists and is stable enough for multiple domain specs to depend on it
- operators can inspect readiness and provenance consistently across integration types

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/012-personal-integrations-platform.md 完成 phase 27 的工作`
