# Non-Knowledge Multi-Turn Continuity

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 55, the bounded
multi-turn continuity slice that must land before knowledge-plane context work.

Primary source documents:
- `docs/specs/039-daemon-owned-thread-and-session-lifecycle.md`
- `docs/runtime/minimal-chat-clients.md`
- `docs/runtime/runtime-architecture.md`

## Background

Minimal chat explicitly excludes daemon-owned multi-turn history. A public personal agent
needs basic continuity inside active threads, but this must not become memory or a hidden
context engine. The goal is bounded, inspectable recent-turn continuity.

## Goal

Support bounded multi-turn continuity from structured thread history without introducing
memory retrieval, semantic knowledge, or long-term personalization.

## Fixed Decisions

- Continuity is limited to explicit recent thread turns and runtime artifacts.
- Inclusion rules must be inspectable.
- No memory writes, semantic retrieval, or knowledge graph behavior is introduced.
- Users can reset continuity at thread level.

## Dependencies On Completed Phases

- Roadmap 54: Daemon-Owned Thread And Session Lifecycle

## In Scope

- persisted recent-turn records
- bounded inclusion window policy
- reset-aware continuity behavior
- operator-visible continuity preview
- chat and channel path support
- tests proving bounded behavior

## Out Of Scope

- memory recall
- context engineering beyond fixed recent-turn assembly
- summarization
- long-term preference learning

## Operator Or User Problems To Solve

- Users expect a thread to remember the immediate prior exchange.
- Operators need to see what recent-turn context was included when behavior is questioned.

## User Stories

- As a user, I can ask a follow-up in the same thread without restating the immediate
  prior message.
- As an operator, I can inspect which recent turns were included.
- As a user, I can reset a thread and stop prior turns from affecting the next response.

## Functional Requirements

- The system MUST persist bounded thread turns with source and runtime linkage.
- The system MUST apply deterministic inclusion limits.
- The system MUST expose included recent-turn references in runtime evidence.
- Reset MUST exclude prior turns from future continuity.
- Continuity MUST work across Web, TUI where applicable, and channel paths that support
  thread identity.

## Compatibility And Operational Notes

This roadmap creates the last non-knowledge continuity layer. Later context engineering
may replace or extend assembly, but must preserve observability.

## Verification Expectations

- Chat and channel tests for follow-up behavior.
- Reset tests proving old turns are excluded.
- Evidence tests showing included turn references.
- Performance tests for bounded turn assembly.

## Definition Of Done

- Multi-turn conversation works for recent thread continuity while remaining visibly
  separate from memory and knowledge-plane behavior.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/040-non-knowledge-multi-turn-continuity.md 完成 phase 55 的工作`
