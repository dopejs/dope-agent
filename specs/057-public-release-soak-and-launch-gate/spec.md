# Feature Specification: Public Release Soak And Launch Gate

**Feature Branch**: `main`
**Created**: 2026-06-30
**Status**: Draft
**Phase / Roadmap**: Phase 72 — Roadmap 72
**Upstream authority**: [docs/specs/057-public-release-soak-and-launch-gate.md](../../docs/specs/057-public-release-soak-and-launch-gate.md)

## Overview
The final non-knowledge public release gate: a codified launch-gate validator over a release
evidence index proving the hosted product can run with real channels, providers, routines,
webhooks, delivery, approval, quota, live validation, evaluation, diagnostics, support evidence,
backup, restore, upgrade, and rollback. Missing required evidence is a no-ship condition;
real-account smoke may be skipped only with structured accepted reasons. This is the entry gate
for context/knowledge/memory work.

## User Scenarios & Testing *(mandatory)*
### US1 - Ship/no-ship from one evidence index (P1)
1. A release owner submits the evidence index; the gate returns a ship/no-ship decision with
   no-ship reasons and the non-knowledge-parity entry-gate flag.
### US2 - See which parity requirements passed/failed (P2)
1. Missing/failed required workloads, <3 channels, missing calendar/mail provider entries,
   unmet soak/support/redaction evidence each produce a specific no-ship reason.

## Requirements *(mandatory)*
- **FR-001**: The gate MUST define required workloads + evidence (status, owner, reason) and treat
  missing required evidence as no-ship.
- **FR-002**: The gate MUST require >= 3 channel entries (real or skipped-with-reason).
- **FR-003**: The gate MUST require calendar + mail provider entries (real or skipped-with-reason).
- **FR-004**: The gate MUST exercise activation, setup, channels, sessions, profile binding,
  routines, webhooks, quota denial, diagnostics, evaluation, live validation, support bundle,
  backup, restore, upgrade, and rollback.
- **FR-005**: The gate MUST state context/knowledge/memory may begin only after non-knowledge
  parity passes or residual exceptions are explicitly accepted.

### Key Entities
- LaunchGateEvidence (channels, provider smoke, workloads, soak/support/redaction flags),
  WorkloadEvidence, LaunchDecision (ship/no-ship + reasons + entry-gate flag).

## Compatibility & Operational Impact *(mandatory)*
- **Compatibility**: Reuses RealAccountSmokeStatus + ValidateRealAccountSmoke + smoke builders;
  adds a launch-gate validator + API endpoint. No new feature domain.
- **Migration / Rollback**: None; pure validator over caller-supplied evidence.
- **Verification**: validator tests for completeness + each no-ship rule; redaction/support-bundle
  exercised via the evidence; contract test for the decision schema.
- **Observability**: decision + reasons; the evidence index is the release artifact.

## Success Criteria *(mandatory)*
- **SC-001**: A complete index with reasoned skips ships and marks non-knowledge parity complete.
- **SC-002**: Each missing/failed required input yields a specific no-ship reason.
- **SC-003**: The decision carries the entry-gate statement gating context/knowledge/memory.

## Assumptions
- The validator is the codified gate; the actual hosted soak + real-account runs feed the
  evidence index (operator-run). Skips require accepted reasons. Context/knowledge/memory remain
  out of scope and are gated on this evidence.
