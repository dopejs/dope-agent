# Provider Streaming Timeout Model

## Purpose

This document defines the next provider hardening slice:

- streaming providers must not be governed by a single total-duration timeout
- SSE-style flows must be treated as long-lived progressive streams
- timeout behavior must be based on connection phase and observed progress
- partial output must be represented explicitly when a stream terminates after visible progress

## Problem Statement

The current `openai_compatible` streaming path uses a single request timeout for the entire stream.

This is incorrect for SSE-like providers because:

- a healthy long-running stream can legitimately exceed the request timeout
- progress is ignored once the stream starts
- an active stream and a stalled stream are both treated as the same timeout class
- downstream channels can already have shown visible content before the provider is marked failed

This causes:

- premature termination of valid long answers
- incorrect operator signals
- poor channel UX when a reply stops after visible progress
- inability to distinguish "no progress" from "slow but healthy progress"

## Target Model

Streaming provider timeout behavior must be split into phases.

### 1. Connect Timeout

Applies to:

- TCP/TLS connection establishment
- request dispatch before response headers are available

Failure meaning:

- upstream endpoint was not reachable or did not accept the request in time

### 2. First-Chunk Timeout

Applies to:

- time from request acceptance to first streamed content event

Failure meaning:

- provider did not begin useful output in time

### 3. Idle Timeout

Applies to:

- time between streamed chunks or streamed progress events

Failure meaning:

- the stream stalled after having started

This is the main timeout for SSE-style generation.

### 4. Optional Hard Cap

Applies to:

- exceptionally long-running streams that exceed an operator-defined maximum duration

Failure meaning:

- protective cap reached

This is not the primary business timeout and should be optional or very high by default.

## Core Rule

For streaming providers:

- progress resets the idle deadline
- a stream that continues to emit chunks is allowed to continue
- a stream should only time out when:
  - it never starts
  - it stalls
  - it exceeds an explicit operator hard cap

For non-streaming providers:

- a request-level total timeout remains correct

## Required Provider Semantics

Provider transports must expose or internally enforce these phases:

- `requestTimeoutMs`
- `streamFirstChunkTimeoutMs`
- `streamIdleTimeoutMs`
- `streamMaxDurationMs`

The exact config shape may vary, but these semantics must exist.

## Required Dispatch Semantics

The dispatch layer must stop collapsing all stream timeout cases into one generic failure.

Required error classes:

- `connect_timeout`
- `first_chunk_timeout`
- `idle_timeout`
- `max_duration_exceeded`

When a stream produced visible output before termination:

- dispatch must record that partial output existed
- downstream systems must be able to distinguish:
  - complete success
  - complete failure with no output
  - partial output followed by failure

## Required Channel Semantics

If the provider stream fails after visible output:

- the connector must not behave like "nothing happened"
- the user-facing reply path must be able to finish as partial

Minimum acceptable behavior:

- preserve already emitted output
- append a short termination marker or operator-defined fallback note
- emit explicit connector/runtime state showing partial completion

## Task Breakdown

### Task 1. Streaming Timeout Contract

Scope:

- define provider timeout phases
- define config shape and dispatch semantics

Task definition of done:

- provider config schema includes separate streaming timeout fields
- docs define connect, first-chunk, idle, and optional hard-cap semantics
- error codes are explicitly mapped
- implementation and contract tests agree on the timeout model

### Task 2. OpenAI-Compatible SSE Transport Refactor

Scope:

- refactor streaming transport to honor progress-based timeout handling

Task definition of done:

- openai-compatible stream no longer uses a single total timeout as the primary streaming timeout
- first chunk and idle timeout are enforced separately
- stream progress refreshes the idle deadline
- transport returns the correct timeout error classification
- tests cover:
  - first chunk timeout
  - idle timeout
  - long but healthy stream
  - optional hard-cap behavior if enabled

### Task 3. Dispatch And Partial-Result Semantics

Scope:

- separate provider completion from partial streamed completion

Task definition of done:

- dispatch records whether partial output was produced
- dispatch eventing differentiates:
  - completed
  - failed before output
  - partial then failed
- connector/runtime consumers can read this state without ambiguity
- tests cover partial stream termination after visible output

### Task 4. Channel Fallback And Operator Visibility

Scope:

- make connector behavior correct when stream termination happens after visible output

Task definition of done:

- IM/channel loop preserves visible partial reply
- connector emits explicit partial-failure or equivalent event trail
- operator can inspect why a stream ended
- tests cover partial streamed reply on Discord path

## Roadmap-Level Definition Of Done

This roadmap is complete only when:

- streaming providers use progress-aware timeout semantics
- openai-compatible SSE transport no longer kills healthy long streams due only to total duration
- timeout failures are classified by phase
- partial streamed output is represented explicitly in dispatch and connector state
- channel behavior after partial streamed failure is deterministic and observable
- contracts, tests, and operator docs all reflect the final behavior

## Explicitly Out Of Scope

- multi-turn memory or context engineering
- provider fallback trees across multiple upstreams
- non-SSE provider families unless required for shared dispatch semantics
- rich channel-specific recovery UX beyond minimal partial-reply correctness
