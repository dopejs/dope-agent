# Channel Reply Progression

## Purpose

This document defines how DopeAgent should model reply progression across channels.

The key rule is:

- reply progression is **channel-specific**
- it is not a universal guarantee

Every channel must be evaluated case by case.

## Design Principle

Do not ask:

- "does every channel support streaming?"

Ask:

- "what reply phases can this channel make visible?"

The two most important visible phases are:

1. `thinking`
2. `incremental output`

If a channel can support them, DopeAgent should support them.

If a channel cannot support them, DopeAgent should wait for the full reply and then send a final message.

## Capability Model

Each channel should be analyzed against at least these capabilities:

- `supportsThinking`
- `supportsStreaming`

Useful secondary capabilities include:

- `supportsMessageEdit`
- `supportsCardUpdate`
- `supportsPlaceholderMessage`
- `maxSafeUpdateFrequency`
- `failureFallback`

For the daemon implementation, the minimum contract is:

- channels may expose `thinking`
- channels may expose incremental reply updates
- channels that do not expose those phases must still support final-only reply delivery

## Reply Progression Levels

### Level 1: Final Only

Channel behavior:

- no thinking visibility
- no incremental output visibility
- user receives only the final reply

Daemon behavior:

- generate the full reply
- send one final outbound message

### Level 2: Thinking Plus Final

Channel behavior:

- can show a temporary thinking state
- cannot safely show incremental content updates

Daemon behavior:

- surface thinking while the provider is running
- send one final reply when complete

### Level 3: Thinking Plus Incremental Output

Channel behavior:

- can show a temporary thinking state
- can update the visible reply while generation is still in progress

Daemon behavior:

- surface thinking at the start
- send an initial reply once enough output exists
- update the same reply incrementally using a channel-safe cadence
- finalize the same reply when generation completes

## Degradation Policy

The degradation policy is explicit:

- if `supportsThinking=true` and `supportsStreaming=true`
  - use `thinking + streaming`
- if `supportsThinking=true` and `supportsStreaming=false`
  - use `thinking + final`
- if `supportsThinking=false`
  - use `final_only`

This is not an error path.

It is the normal way the daemon adapts to channel limits.

## Ownership Boundary

Reply progression must stay inside daemon-owned execution truth.

That means:

- the connector must not bypass session routing
- the connector must not bypass run or step lifecycle
- the connector must not bypass provider resolution
- inbound and outbound delivery state must remain durable

The connector adapter is responsible for channel mechanics.

The daemon remains responsible for:

- routing
- runtime lifecycle
- provider invocation
- persistence
- eventing
- degradation policy

This boundary now matters even more because roadmap 28 adds a separate delivery plane:

- foreground reply progression is for active connector conversations
- background delivery targets are for routed results after the source work reaches a
  terminal state
- a successful foreground reply does not stand in for background delivery truth
- background delivery may reuse connector transport mechanics, but it still records its
  own outcome and attempt ledger

## Discord Mapping

Discord currently maps to:

- `supportsThinking=true`
- `supportsStreaming=true`

Thinking visibility:

- Discord typing indicator

Incremental output visibility:

- send initial reply message
- edit that same reply message with throttled updates

Current daemon strategy for Discord:

- send an immediate typing indicator
- continue typing periodically while no final visible reply has been settled
- stream provider output through daemon-owned progression logic
- create the outbound message on the first visible chunk
- edit the same Discord message on later chunks
- finalize the same message when generation completes

## Safety Rules

Reply progression must not devolve into unsafe edit spam.

So even when a channel supports streaming:

- updates must be throttled
- token-level edits are not required
- the system should prefer chunked visible progress over maximum update frequency

For Discord, the current implementation uses throttled incremental edits instead of per-token message edits.

## Current Scope

This design currently covers:

- capability-based reply progression
- Discord thinking support
- Discord incremental output support

It does not yet cover:

- Feishu card progression
- channel-specific stop controls
- rich media progression
- multi-turn conversation memory
