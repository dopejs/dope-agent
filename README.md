# DopeAgent

Working project for DopeAgent, a personal agent OS with stronger coding and architecture capabilities.

## Current Goal

Build DopeAgent as a personal agent OS that can run as a daemon on a user machine or server, with Web UI, TUI, and chat-channel access. OpenClaw is a reference product for capability scope, but not a required implementation base or architecture.

This repository currently contains planning documents plus an early daemon implementation foundation. The initial design goals are:

- personal-agent-first, not coding-agent-only
- replace prompt-centric context packing with a structured context assembly pipeline
- replace flat memory with layered memory plus explicit write and recall policy
- keep coding and architecture as high-value domain capabilities inside a broader agent OS
- preserve replayability, auditability, and recoverability from the start

## Current Implementation Priority

Memory is intentionally deferred.

The immediate build priority is the daemon foundation:

- runtime core objects
- run and step lifecycle
- event log
- checkpoint and restore
- tool-call envelopes
- policy gates
- host-platform integration boundary

That daemon foundation is now closed through:

- Roadmap 6: `Real Conversation Core`
- Roadmap 7: `Minimal Chat Clients`
- Roadmap 8: `Ingress Routing Closure`

Execution is roadmap-driven:

- a roadmap is the delivery unit
- a roadmap contains multiple tasks
- every task has a completion boundary
- a round is only considered complete when the whole roadmap is closed
- partial or narrow implementations do not count as complete
- if a target is too broad, it must be split into multiple closed roadmaps before implementation

## Planning Docs

- `docs/01-product-outline.md`
- `docs/02-memory-system.md`
- `docs/03-runtime-architecture.md`
- `docs/04-openclaw-integration.md`
- `docs/05-roadmap.md`
- `docs/06-foundation-first.md`
- `docs/07-feature-phasing.md`
- `docs/08-openclaw-architecture-gaps.md`
- `docs/09-tech-stack-recommendation.md`
- `docs/10-module-map.md`
- `docs/11-repo-layout.md`
- `docs/12-daemon-scope.md`
- `docs/13-daemon-api-and-event-model.md`
- `docs/14-daemon-tasks.md`
- `docs/15-daemon-roadmaps.md`
- `docs/16-operator-trust-model.md`
- `docs/17-schema-contract-pipeline.md`
- `docs/18-migration-versioning.md`
- `docs/19-p0-release-review.md`
- `docs/20-minimal-conversation-path.md`
- `docs/21-minimal-chat-clients.md`
- `docs/22-ingress-routing-closure.md`

## Working Assumptions

- OpenClaw is treated as a feature and product benchmark, not a required runtime base.
- Context engineering, memory, planning, handoff, and policy will be redesigned rather than lightly patched.
- Long-lived agent state must be observable, replayable, and safe to evolve.
- The first implementation milestone is a minimal runtime skeleton, not a memory subsystem.
