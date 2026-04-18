# First IM Channel Loop

Status: `closed`

Implemented channel:

- `Discord Bot`

Delivery path:

- `gateway`

## Purpose

This document defines the next closed delivery slice after managed providers.

The goal is not to add abstract connector plumbing again. The goal is to close one real IM channel end to end:

- receive a real IM message
- route it into daemon session truth
- issue a single-turn model request
- send the assistant reply back through the same IM channel

That is the minimum product bar for saying DopeAgent can actually talk through IM.

## Why This Is A Separate Roadmap

The daemon already has:

- connector supervision
- ingress routing
- session creation
- run creation
- chat query and streaming
- provider control plane

What it does **not** yet have is a closed channel loop.

If we only add:

- one more connector state object
- one more ingress API

then we are back to a narrow implementation.

This roadmap is only complete when one real IM connector can carry a full user message -> assistant reply loop.

## Reference Target

OpenClaw is the feature reference, but not the implementation model.

For the first closed IM slice, the recommended reference connector is:

- `Discord Bot`

Reason:

- official bot and application API
- stable, well-documented transport model
- closer to the long-term multi-channel agent surface than a simpler bot-only channel
- still official and supportable, unlike unofficial web reverse-engineered stacks

This does **not** mean Discord is the final strategic channel. It means Discord is the chosen first channel to close without building on a brittle transport.

## Roadmap 11

### Name

`Roadmap 11: First IM Channel Loop`

### Goal

Make DopeAgent able to receive and answer IM messages through one real connector using the existing daemon runtime and provider system.

Delivery result:

- Discord connector config is schema-backed and redacted through `/v1/config`
- Discord connector runtime is supervised through the daemon connector plane
- inbound Discord message normalization resolves or creates session truth
- inbound message creates run and step execution state
- daemon invokes the configured provider through the existing chat service
- daemon sends the reply back through the Discord connector and persists inbound/outbound delivery records
- duplicate inbound delivery is deduplicated durably by connector message identity
- connector and runtime events are persisted and replayable after restart

### Tasks

#### 1. IM Connector Runtime Contract

Scope:

- define the daemon-facing contract for a real IM connector process
- define inbound message normalization and outbound reply dispatch
- define connector auth/config shape for the first IM channel

Task definition of done:

- connector config is explicit and schema-backed
- connector can report readiness, failure, and webhook or poll health through daemon supervision
- inbound message envelope and outbound reply envelope are explicit
- connector-specific auth/config redaction is covered by config inspection rules
- tests cover config parsing and contract validation

#### 2. Discord Connector Implementation

Scope:

- implement the first real IM connector using the Discord bot/app API
- support one deployment path that is operationally realistic

Task definition of done:

- daemon can configure a Discord bot token and delivery mode
- connector can receive real inbound Discord messages through the chosen delivery path
- connector can send outbound text replies back to Discord
- auth and transport failures are classified explicitly
- tests cover inbound normalization, outbound request shaping, and transport/auth failure mapping

#### 3. Inbound-To-Reply Execution Loop

Scope:

- close the runtime loop from inbound IM message to assistant reply
- keep the first version single-turn and stateless on the model side

Task definition of done:

- an inbound Discord message creates or resolves the correct session
- daemon creates the corresponding run or message-handling execution path
- daemon invokes the configured provider through the existing chat contract
- daemon sends the reply back through the Discord connector
- duplicate delivery and retry behavior are explicitly handled enough to avoid obvious double-reply bugs
- tests cover at least one full success path and one failure path

#### 4. IM Operator Docs And End-To-End Verification

Scope:

- document how to configure and operate the first IM channel
- prove the loop works end to end

Task definition of done:

- operator docs explain token config, connector startup, webhook or polling mode, and failure visibility
- daemon APIs, schemas, and events are aligned for the new channel
- end-to-end verification proves message in -> reply out through the real connector boundary
- roadmap notes clearly state what remains out of scope

### Roadmap Definition Of Done

- DopeAgent can carry one real IM conversation turn through a real connector
- the connector is supervised, configurable, and observable through daemon APIs
- inbound routing and outbound reply behavior are durable enough for operator use
- docs, schemas, and tests are aligned with the actual IM loop

### Explicitly Out Of Scope

- multiple IM channels in the same roadmap
- long-term memory or context assembly for IM
- unofficial or reverse-engineered IM transports
- rich media, files, reactions, voice, or typing indicators

Current note:

- guild mention gating is supported through connector config
- the roadmap does not attempt to model richer group semantics beyond mention requirement and allowlists

## Recommended Delivery Choice

For this roadmap, choose one connector and close it fully.

Recommended:

- `Discord Bot`

Not recommended for the first closed roadmap:

- WeChat
- iMessage
- Signal
- Discord user-account style transports

Those may be important later, but they create unnecessary transport risk before the first IM loop is actually closed.

## Next Architectural Constraint

The connector implementation should still follow the daemon rules already established:

- daemon owns routing, runtime, policy, and persistence truth
- the channel adapter should not invent its own session semantics
- fragile SDK or transport logic should stay isolated from the daemon core when possible

## Completion Standard

This roadmap is not complete when:

- Discord can only ingest but not reply
- Discord can only reply through a manual test path
- daemon has connector state but no real IM execution loop
- the channel path bypasses daemon runtime or provider resolution

It is only complete when one real IM connector is closed end to end.
