# Sandbox Backend Comparison

## Purpose

Record the current backend posture after Roadmap 20 so operators and future roadmap work do
not need to reconstruct it from code or commit history.

## Implemented Backends

### `subprocess`

Use for:

- default executable skills
- current local-tool and MCP paths that already run on the sandbox plane
- hosts where no stronger backend is required

Guarantees:

- declared filesystem scoping with daemon-side preflight validation
- declared network policy with approval and audit truth
- filtered host-environment injection

Limits:

- filesystem and network guarantees are not container-enforced
- operator trust depends on truthful daemon mediation rather than OS isolation

### `docker`

Use for:

- executable skills that explicitly declare `execution.profile_id: docker_default`
- workloads that require materially stronger filesystem or network isolation than
  `subprocess`

Guarantees:

- container mount scoping rather than host-process cwd checks alone
- container network mode selection (`none` or bridged) instead of declaration-only network
  truth
- explicit backend identity preserved in sandbox execution and tool-call history

Limits:

- this slice does not support every declared access pattern on `docker`
- allow-list host/port semantics and loopback guarantees still fail closed as
  `unsupported` instead of silently degrading
- attached execution is still subprocess-only

## Host Prerequisites

`docker` is considered ready only when:

- the `docker` CLI is available on `PATH`
- the local docker runtime is reachable
- the configured image is available locally

If those prerequisites are absent, the daemon reports:

- backend availability as `unavailable`
- host status as `missing_prerequisite`
- request outcome as `unsupported` when `docker` is explicitly required

## Degradation Rules

- No request that explicitly requires `docker` may silently fall back to `subprocess`.
- If a `docker` request cannot satisfy declared access guarantees, the result is
  `unsupported` with mismatch classification, not a normal runtime failure.
- Unmodified executable skills remain on `subprocess`; there is no heuristic auto-promotion.

## Migration Inventory

Already sandbox-backed:

- managed-provider inspection and prompt execution
- MCP lifecycle and tool exposure policy
- executable skills
- current high-risk local tools: `exec`, `shell`, `browser`

Using `docker` in this slice:

- only executable skills that explicitly opt in

Still deferred:

- broader migration of lower-risk local capability families
- moving MCP, managed providers, or all local tools onto `docker`
- SSH, remote, or VM-grade backends
