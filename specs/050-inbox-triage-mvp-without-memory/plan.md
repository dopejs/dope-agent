# Implementation Plan: Inbox Triage MVP Without Memory

**Branch**: `main` | **Spec**: [spec.md](./spec.md) | **Upstream**: [docs/specs/050-inbox-triage-mvp-without-memory.md](../../docs/specs/050-inbox-triage-mvp-without-memory.md)
**Phase / Roadmap**: Phase 65 — Roadmap 65

## Summary

Add a new, self-contained `internal/triage` subsystem: a triage policy resource (ordered explicit
rules), a deterministic triage run over a selected/unread message set, fixed classifications, and
proposed outcomes (draft reply / reminder / delivery digest / no-action). Decisions are
transparent (matched rule + evidence), replayable, and never silently auto-sent. No memory, no
learned preferences. The run is a pure function of (policy, messages) so it is deterministic.

## Technical Context
- **Language**: Go 1.24 (daemon). New additive package; reuses mail message snapshots as input.
- **Dependencies**: `internal/triage` (new); `internal/mail` (message snapshot fields, mapped to
  triage.Message); proposed reminder/delivery outcomes reference existing managers but are not
  executed by triage (proposals only); `internal/api` + schemas; `internal/app` wiring.
- **Storage**: policies in-memory with Restore (mirrors other rule managers); runs are computed
  and returned (optionally recorded).
- **Testing**: rule-matching unit tests (each classification, default, first-match, validation),
  replay determinism, proposed-outcome (no silent send).
- **Constraints**: explicit-rule only; no memory/learned model; no silent auto-send; deterministic;
  no credential/content leakage beyond message preview.

## Constitution Check
- **Roadmap closure**: transparent, auditable, replayable rule-driven triage with proposed
  outcomes — the upstream DoD.
- **Production-grade**: deterministic, validated rules, first-match ordering, proposals gated.
- **Contracts first**: triage policy/run/decision schemas; contract tests validate.
- **Verification**: rule + run + determinism + validation coverage.
- **Environment**: opt-in (no policy = no triage); fake mail input in test.

## Project Structure
```
specs/050-inbox-triage-mvp-without-memory/  spec.md plan.md tasks.md checklists/
daemon/internal/triage/types.go      # Policy/Rule/Condition/Decision/Run + classifications/outcomes
daemon/internal/triage/manager.go    # CreatePolicy/GetPolicy/ListPolicies/Run + matcher
daemon/internal/triage/*_test.go     # rule/run/determinism/validation tests
daemon/internal/api/triage.go, types.go, server.go  # policy CRUD + run endpoints (additive)
daemon/internal/app/app.go           # construct + register triage manager
schemas/api                          # triage policy/run/decision schemas
```

## Complexity Tracking
No violations. Triage is an additive, self-contained subsystem; outcomes are proposals so no new
side-effect plane is introduced.
</content>
