# Daemon-Owned Thread And Session Lifecycle

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 54, the
non-knowledge thread and session lifecycle productization slice.

Primary source documents:
- `docs/runtime/minimal-chat-clients.md`
- `docs/runtime/runtime-architecture.md`
- `docs/channels/channel-reply-progression.md`
- `docs/specs/033-channel-connector-conformance-contract.md`

## Background

The daemon has sessions, runs, steps, events, and channel routing, but product-facing
thread/session lifecycle remains minimal. OpenClaw/Hermes-style parity requires users and
operators to inspect, reset, archive, and reason about ongoing conversations without using
memory as a substitute.

## Goal

Make sessions and threads first-class daemon-owned product resources with lifecycle,
inspection, reset, archive, and runtime linkage.

## Fixed Decisions

- This roadmap provides conversation continuity metadata, not memory recall.
- Thread truth is structured state, not prompt-file state.
- Reset and archive are explicit lifecycle operations.
- Channel connectors must route into daemon-owned sessions rather than local state.

## Dependencies On Completed Phases

- Roadmap 25: Scheduled Tasks And Wakeups
- Roadmap 28: Delivery And Notifications
- Roadmap 48: Channel Connector Conformance Contract

## In Scope

- thread/session resource model refinement
- list, detail, reset, archive, and reopen actions
- channel/source linkage
- run, workflow, approval, delivery, and connector-message projections
- restart-safe lifecycle state
- SDK and operator shell views

## Out Of Scope

- memory recall
- context packing
- semantic summaries
- autonomous conversation pruning

## Operator Or User Problems To Solve

- Users need to find and reset conversations.
- Operators need to trace a channel message to the run, workflow, approval, and delivery
  records it caused.

## User Stories

- As a user, I can inspect recent conversations and reset one explicitly.
- As an operator, I can trace a thread across channel messages and runtime records.

## Functional Requirements

- The daemon MUST expose tenant-scoped thread/session list and detail routes.
- Threads MUST link source channel/account identity to runs, workflows, approvals, and
  delivery outcomes.
- Reset and archive MUST be durable and auditable.
- Reset MUST not delete historical runtime evidence.
- Archived threads MUST remain inspectable.

## Compatibility And Operational Notes

Existing session routing must remain compatible. New lifecycle actions should be additive
and migration-safe.

## Verification Expectations

- Store, API, SDK, and web tests for lifecycle operations.
- Restart tests proving thread state and linkage survive daemon restart.
- Connector regression proving inbound messages attach to daemon-owned thread truth.

## Definition Of Done

- Thread/session state is inspectable product truth and no longer hidden behind isolated
  channel or chat behavior.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/039-daemon-owned-thread-and-session-lifecycle.md 完成 phase 54 的工作`
