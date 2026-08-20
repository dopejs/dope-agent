# P0 Release Review

## Ship Decision

Safe with conditions.

The daemon now has:

- a closed runtime core
- supervised connector and capability state
- a real LLM dispatch plane
- a local-first trust boundary
- versioned persisted state
- contract tests for request, response, and persisted event payloads

That is enough for a guarded P0 release decision.

## Blocking Issues

No release-blocking defect is currently open from the implemented P0 daemon scope.

## Conditions

- release as a local-first single-user daemon, not as a multi-tenant control plane
- treat SQLite schema upgrades as forward-only
- take a backup of `~/.kura/daemon.sqlite` before upgrading across daemon versions
- do not market SSE framing as schema-stable yet; only the JSON event envelope is contract-tested

## Required Verification Steps

Before shipping a build:

1. run `make daemon-contract-test`
2. run `make daemon-test`
3. verify startup against an existing local data directory
4. verify auth pairing, run lifecycle, tool-call approval flow, and LLM dispatch in one smoke pass

## Rollback Considerations

- old binaries must not be pointed at newer schema versions
- rollback means restoring the previous SQLite file together with a compatible daemon binary
- there is no automatic down migration path in P0

## Residual Risks

- connector and capability supervision is contract-complete, but it is still state-and-policy supervision, not full external process orchestration
- SSE framing is not schema-backed
- policy is local-first and intentionally not an org-grade RBAC system
- migration strategy is forward-only, so operator backup discipline matters

## Release Notes For Engineers

- schema changes must now ship with contract test updates
- persisted state changes must now ship with a version bump and migration entry
- response wrappers are part of the public contract and need explicit schemas
