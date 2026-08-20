# Implementation Plan: Sandbox Managed Provider Convergence

**Branch**: `001-sandbox-managed-providers` | **Date**: 2026-04-19 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-sandbox-managed-providers/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Close the managed-provider convergence sub-slice of the sandbox follow-on work by bringing
the remaining Claude and Codex provider-owned local state access under explicit sandbox
requirement, policy, and audit control. The plan keeps the current subprocess backend,
avoids a generic secret-ref system, and preserves existing managed-provider workflows while
making undeclared access fail closed and operator-visible.

## Technical Context

**Language/Version**: Go 1.24.0; Markdown design artifacts  
**Primary Dependencies**: `daemon/internal/managedproviders`, `daemon/internal/sandbox`, `daemon/internal/providers`, `daemon/internal/api`, `daemon/internal/store`, `daemon/internal/contracts`, `modernc.org/sqlite`  
**Storage**: SQLite daemon state; provider-owned local files under `~/.claude` and `~/.codex`; temp output files for Codex prompt execution  
**Testing**: `go test ./internal/managedproviders ./internal/sandbox ./internal/api ./internal/app ./internal/contracts`, `go test ./...`, `make daemon-contract-test` when API/schema/event surfaces change  
**Target Platform**: macOS/Linux local daemon in the default test environment  
**Project Type**: Go daemon/control-plane service with CLI-backed managed-provider bridges  
**Performance Goals**: Sandbox requirement evaluation and local-state preflight for managed-provider auth-status, logout, and prompt-execution workflows should add no more than 100 ms of daemon-side overhead per request in targeted regression verification, excluding upstream CLI runtime and external provider latency  
**Constraints**: Existing subprocess backend only; fail closed outside declared requirements; no per-file standalone sandbox execution requirement; no generic secret-ref substrate in this slice; preserve backward-compatible external surfaces; keep test/prod separation intact  
**Scale/Scope**: 2 managed providers (`claude_managed`, `codex_managed`), 3 in-scope workflows (`auth status`, `logout`, `prompt execution`), a small number of provider-owned local state classes, no new backend families

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Roadmap closure: PASS. This plan explicitly recuts the larger `Roadmap 17` prerequisite
  work into an auditable managed-provider convergence slice. The implementation may close
  this slice, but it MUST keep the parent roadmap open in docs until the remaining
  requirement-declaration, secret-scope, and provenance work outside managed providers is
  also complete.
- Production-grade change control: PASS. The design keeps the blast radius inside daemon
  control-plane, provider-bridge, schema/event, and docs surfaces. It avoids a new backend,
  avoids a generic secret substrate, and keeps rollback to the pre-convergence bridge logic
  possible.
- Contracts and auditability: PASS. The plan includes explicit contract artifacts for
  provider auth surfaces, sandbox execution surfaces, and event/schema updates so hidden
  local access paths cannot remain undocumented.
- Verification and observability: PASS. The plan names targeted provider, sandbox, API,
  contract, and full-suite verification, plus operator-visible provenance and failure
  classification expectations.
- Environment and secrets: PASS. The plan keeps local work in `KURA_ENV=test`, preserves
  `~/.kura-test` / `~/.kura` separation, and treats credential-bearing provider local state
  as sensitive and redacted.

Post-design re-check:

- PASS. Research decisions keep the feature on the current subprocess backend and do not
  imply Docker, SSH, remote execution, or VM-grade isolation.
- PASS. The data model separates logical managed-provider operations from subprocess-backed
  sandbox executions so Codex auth-state workflows can be audited even when no CLI process
  runs.
- PASS. Contracts stay additive and backward-compatible: no new top-level routes are
  required, and any schema/event changes remain explicit and versioned.

## Project Structure

### Documentation (this feature)

```text
specs/001-sandbox-managed-providers/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── managed-provider-sandbox-surfaces.md
└── tasks.md
```

### Source Code (repository root)

```text
daemon/
├── internal/
│   ├── app/
│   ├── api/
│   ├── contracts/
│   ├── managedproviders/
│   ├── providers/
│   ├── sandbox/
│   └── store/
└── go.mod

schemas/
├── api/
└── events/

docs/
├── harness/
└── runtime/

AGENTS.md
```

**Structure Decision**: This feature lives entirely inside the daemon control plane and its
contract/documentation boundaries. No client, web, TUI, or SDK work is planned. The
implementation focus is on `managedproviders`, `sandbox`, provider auth/runtime surfaces,
schema/event contracts, and the corresponding docs.

## Complexity Tracking

No constitution violations are expected. This plan deliberately avoids introducing a new
backend, a generic secret-ref substrate, or any claim that `Roadmap 17` is fully closed by
this managed-provider-only slice.
