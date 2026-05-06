# Group Room Reset And Handoff Semantics

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 56, the group,
room, reset, and handoff semantics slice for multi-channel personal-agent use.

Primary source documents:
- `docs/specs/033-channel-connector-conformance-contract.md`
- `docs/specs/039-daemon-owned-thread-and-session-lifecycle.md`
- `docs/specs/040-non-knowledge-multi-turn-continuity.md`

## Background

Public channel use introduces group rooms, direct messages, mentions, shared channels,
handoffs between surfaces, and explicit reset expectations. These rules must be
structured and consistent before memory or advanced context uses them.

## Goal

Define and implement group, room, reset, and cross-surface handoff semantics on top of
daemon-owned thread truth.

## Fixed Decisions

- Group/room isolation is explicit product state.
- Reset is a lifecycle operation, not a prompt instruction.
- Handoff creates traceable linkage between source and destination surfaces.
- No memory or semantic continuity is introduced.

## Dependencies On Completed Phases

- Roadmap 54: Daemon-Owned Thread And Session Lifecycle
- Roadmap 55: Non-Knowledge Multi-Turn Continuity

## In Scope

- group and room identity model
- mention and participation policy linkage
- reset behavior for DM and group threads
- handoff from one channel or web shell to another
- operator-visible handoff and reset events
- connector conformance additions where needed

## Out Of Scope

- group memory
- team knowledge base behavior
- autonomous agent-to-agent delegation
- enterprise channel governance beyond tenant permissions

## Operator Or User Problems To Solve

- Users need predictable behavior in groups and DMs.
- Operators need to know when a conversation was reset or handed off.

## User Stories

- As a user, I can reset a group thread without deleting evidence.
- As a user, I can continue a thread from a channel in the web shell with traceable
  handoff.
- As an operator, I can inspect group participation policy.

## Functional Requirements

- The system MUST distinguish DM, group, room, and web-originated threads.
- Reset MUST be durable, auditable, and source-specific.
- Handoff MUST create source and destination references.
- Group routing MUST honor mention and allowlist policy.
- Handoff MUST not bypass tenant or connector permissions.

## Compatibility And Operational Notes

Existing connector routing must remain backward compatible. Handoff behavior should be
additive and visible in thread detail.

## Verification Expectations

- API and connector tests for DM/group reset.
- Handoff tests across channel and web shell surfaces.
- Tenant and permission tests for group participation policy.

## Definition Of Done

- Multi-channel group and handoff behavior is predictable, resettable, and inspectable
  without relying on memory.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/041-group-room-reset-and-handoff-semantics.md 完成 phase 56 的工作`
