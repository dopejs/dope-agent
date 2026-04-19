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
- Roadmap 9: `Provider Identity And Profiles`
- Roadmap 10: `Managed Coding Providers`
- Roadmap 11: `First IM Channel Loop`
- Roadmap 12: `Channel Reply Progression`
- Roadmap 13: `Provider Streaming Timeout Semantics`
- Roadmap 14: `Test Environment Workflow`
- Roadmap 15: `Skill Registry And Prompt Support`

Execution is roadmap-driven:

- a roadmap is the delivery unit
- a roadmap contains multiple tasks
- every task has a completion boundary
- a round is only considered complete when the whole roadmap is closed
- partial or narrow implementations do not count as complete
- if a target is too broad, it must be split into multiple closed roadmaps before implementation

## Planning Docs

Docs are now grouped by module under [docs/README.md](/Users/John/Code/agent-os/docs/README.md).

High-signal entry points:

- Product scope: [01-product-outline.md](/Users/John/Code/agent-os/docs/product/product-outline.md)
- Runtime roadmap and task registry:
  - [15-daemon-roadmaps.md](/Users/John/Code/agent-os/docs/runtime/daemon-roadmaps.md)
  - [14-daemon-tasks.md](/Users/John/Code/agent-os/docs/runtime/daemon-tasks.md)
- Provider architecture: [23-provider-architecture.md](/Users/John/Code/agent-os/docs/providers/provider-architecture.md)
- Channel behavior: [28-channel-reply-progression.md](/Users/John/Code/agent-os/docs/channels/channel-reply-progression.md)
- Harness architecture: [32-harness-architecture.md](/Users/John/Code/agent-os/docs/harness/harness-architecture.md)
- Test environment workflow: [30-test-environment-workflow.md](/Users/John/Code/agent-os/docs/dev/test-environment-workflow.md)

## Local Environment Modes

Development defaults to the **test** environment:

- `DOPE_ENV=test`
- data dir: `~/.dope-test`
- config file: `~/.dope-test/config.json`
- daemon bind addr: `127.0.0.1:19192`

Production is explicit:

- `DOPE_ENV=prod`
- data dir: `~/.dope`
- config file: `~/.dope/config.json`
- daemon bind addr: `127.0.0.1:19191`

Repository entrypoints:

- `make daemon-run-test`
- `make daemon-run-test-live`
- `make daemon-test-status`
- `make daemon-run-prod`
- `make daemon-prod-status`

For local debugging, use the project skill at `.agents/skills/dope-test-env/SKILL.md`.

`make daemon-run-test` is the safe default for debugging and keeps Discord disabled unless you explicitly opt in with `make daemon-run-test-live` or `DOPE_CONNECTORS_DISCORD_ENABLED=true`.

## Working Assumptions

- OpenClaw is treated as a feature and product benchmark, not a required runtime base.
- Context engineering, memory, planning, handoff, and policy will be redesigned rather than lightly patched.
- Long-lived agent state must be observable, replayable, and safe to evolve.
- The first implementation milestone is a minimal runtime skeleton, not a memory subsystem.
