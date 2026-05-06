# Execution Backend And Sandbox Profile UX

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 68, the execution
backend and sandbox profile user experience slice.

Primary source documents:
- `docs/harness/sandbox-execution-plane.md`
- `docs/harness/sandbox-backend-comparison.md`
- `specs/001-sandbox-managed-providers/`
- `specs/002-sandbox-requirement-contract/`
- `docs/specs/052-operator-managed-skill-and-capability-catalog.md`

## Background

HermesAgent advertises multiple execution backends. DopeAgent has stronger sandbox and
requirement primitives, but users need to understand which execution profiles exist, what
they allow, and why a tool cannot run.

## Goal

Expose execution backend and sandbox profile availability, requirements, health, and
selection through product surfaces.

## Fixed Decisions

- The daemon policy layer remains authoritative for execution permission.
- Profiles describe capability and risk; they do not grant hidden access.
- Hosted defaults must fail closed when a backend is unavailable.
- This roadmap does not add a new sandbox backend unless needed for UX closure.

## Dependencies On Completed Phases

- Roadmap 16: Sandbox Execution Plane
- Roadmap 17: Sandbox Requirement Declarations And Consumer Convergence
- Roadmap 67: Operator-Managed Skill And Capability Catalog

## In Scope

- execution profile list/detail projection
- backend health and readiness
- requirement satisfaction and missing-requirement explanation
- profile selection where policy allows
- operator shell UX for profile visibility
- support evidence for execution denial

## Out Of Scope

- arbitrary remote execution marketplace
- unmanaged SSH credential sprawl
- local privilege escalation
- memory-driven backend selection

## Operator Or User Problems To Solve

- Users need to know why a tool cannot run in hosted mode.
- Operators need to inspect sandbox profile health and requirement mismatches.

## User Stories

- As a user, I can see that a capability needs Docker, SSH, or local shell access.
- As an operator, I can diagnose backend unavailable or requirement denied states.

## Functional Requirements

- The system MUST expose available execution profiles with status, requirements, and risk
  classification.
- Denials MUST link to missing requirements or policy decisions.
- Catalog items MUST show compatible and incompatible profiles.
- Profile changes MUST be permission-gated and auditable.

## Compatibility And Operational Notes

Existing sandbox contracts remain authoritative. UX projections should not weaken
preflight or approval gates.

## Verification Expectations

- API/schema/SDK/web tests for profile projection and denials.
- Policy tests proving UX selection cannot bypass requirements.
- Fake backend health tests for unavailable, degraded, and ready profiles.

## Definition Of Done

- Users and operators can understand execution backend availability and sandbox denials
  from product surfaces.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/053-execution-backend-and-sandbox-profile-ux.md 完成 phase 68 的工作`
