# Workspace And Capability Binding

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 58, the workspace
and capability binding slice for non-knowledge personal-agent parity.

Primary source documents:
- `docs/specs/042-agent-profile-and-persona-configuration.md`
- `docs/architecture/module-map.md`
- `docs/specs/007-mcp-catalog-management-and-distribution.md`
- `docs/specs/022-hosted-secrets-integrations-and-connector-isolation.md`

## Background

A personal agent must know which profile, workspace, and capability set applies to a
channel, account, or thread. This must be structured before context and memory use these
bindings as inputs.

## Goal

Bind agent profiles, workspaces, channels, integration accounts, and visible capabilities
through tenant-scoped, auditable product state.

## Fixed Decisions

- Bindings are explicit configuration, not memory.
- Capability visibility must be enforced by policy and reflected in runtime evidence.
- Workspace binding does not imply filesystem access unless a capability grants it.
- This roadmap does not create a plugin marketplace.

## Dependencies On Completed Phases

- Roadmap 57: Agent Profile And Persona Configuration
- Roadmap 48: Channel Connector Conformance Contract
- Roadmap 37: Hosted Secrets, Integrations, And Connector Isolation

## In Scope

- workspace resource or projection sufficient for binding
- channel-to-profile and channel-to-workspace binding
- integration-account-to-profile defaults where needed
- capability visibility and default enablement
- runtime evidence for active binding selection
- web shell and SDK support

## Out Of Scope

- memory-backed workspace knowledge
- per-tenant physical workspace storage migration
- community marketplace
- autonomous capability selection beyond policy-allowed visibility

## Operator Or User Problems To Solve

- Users need different channels or workspaces to use different defaults safely.
- Operators need to inspect why a capability was visible or hidden for a run.

## User Stories

- As a user, I can bind a channel to an agent profile and workspace.
- As an operator, I can see which capability set was active for a run.
- As a user, I can disable risky capabilities for a profile.

## Functional Requirements

- The system MUST expose tenant-scoped bindings between profile, workspace, channel,
  integration account, and capability visibility.
- Runtime records MUST include active binding IDs where they influenced execution.
- Policy checks MUST enforce hidden or disabled capabilities.
- Binding changes MUST be auditable and restart-safe.

## Compatibility And Operational Notes

Existing default behavior should map to one default personal profile and workspace to
avoid breaking current users.

## Verification Expectations

- API, store, schema, SDK, and web tests for binding lifecycle.
- Runtime/policy tests proving hidden capabilities cannot execute.
- Connector tests proving channel binding affects new runs without rewriting history.

## Definition Of Done

- Profile, workspace, channel, and capability selection are explicit product truth ready
  for later context and memory inputs.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/043-workspace-and-capability-binding.md 完成 phase 58 的工作`
