<!--
Sync Impact Report
Version change: template -> 1.0.0
Modified principles:
- Template principle slot 1 -> I. Roadmap-Closed Delivery
- Template principle slot 2 -> II. Production-Grade, Minimal, Reversible Change
- Template principle slot 3 -> III. Contracts, Compatibility, And Auditability
- Template principle slot 4 -> IV. Verification And Observability Are Mandatory
- Template principle slot 5 -> V. Safe Environment And Secret Discipline
Added sections:
- Engineering Constraints
- Delivery Workflow
Removed sections:
- None
Templates requiring updates:
- ✅ .specify/templates/plan-template.md
- ✅ .specify/templates/spec-template.md
- ✅ .specify/templates/tasks-template.md
- ✅ .specify/templates/commands/*.md (directory not present; no update required)
- ✅ README.md (reviewed; no update required)
- ✅ AGENTS.md (reviewed; no update required)
Follow-up TODOs:
- None
-->
# DopeAgent Constitution

## Core Principles

### I. Roadmap-Closed Delivery
All implementation work MUST close one whole roadmap or explicitly recut the roadmap
before coding begins. Tasks exist for auditability only; they MUST NOT be used to justify
shipping a partial roadmap. Partial, provisional, or demo-grade slices remain incomplete
until the roadmap definition of done is satisfied end-to-end.

Rationale: DopeAgent is building control-plane and harness infrastructure where partial
closure creates misleading state, operational risk, and long-lived debt.

### II. Production-Grade, Minimal, Reversible Change
Every requested change MUST be treated as production work unless the requester explicitly
says otherwise. Changes MUST optimize for correctness, maintainability, debuggability, and
low blast radius; broad refactors are allowed only when required to close the target
roadmap safely. Backward compatibility MUST be preserved unless a breaking change is
explicitly approved and paired with migration and rollback guidance.

Rationale: The daemon, harness, and contract surfaces are long-lived operator systems, not
demo code.

### III. Contracts, Compatibility, And Auditability
API, schema, event, config, and persistence changes MUST remain explicit, versioned, and
auditable. Any change to those surfaces MUST update committed schemas, fixtures,
documentation, and compatibility notes together. Hidden side paths, ad hoc execution
paths, and undocumented control-plane behavior MUST be converged onto explicit daemon-owned
contracts.

Rationale: DopeAgent depends on replayability, operator trust, and precise debugging
across daemon, providers, channels, and harness subsystems.

### IV. Verification And Observability Are Mandatory
Every production change MUST include targeted verification at the affected layer and
explicit operator-facing failure visibility. Contract tests are REQUIRED whenever
API/schema/event surfaces change; restart, migration, concurrency, or execution-boundary
changes MUST add the strongest coverage the repository supports. Missing observability,
residual risks, and unverified paths MUST be called out explicitly rather than hidden.

Rationale: Production-grade control planes fail at boundaries; they are only operable when
tests, logs, events, and docs stay aligned with the real behavior.

### V. Safe Environment And Secret Discipline
Development and verification MUST default to the test environment, not live state.
Production access, live connectors, and privileged execution MUST be explicit. Secrets
remain operator-owned material: they MUST NOT be logged, echoed, or inherited into
execution paths without declared scope, redaction behavior, and need.

Rationale: The project is designed to run on personal machines and servers where unsafe
defaults can damage real operator state.

## Engineering Constraints

- Implementation MUST read local code and docs before changing behavior.
- New abstractions MUST follow existing module boundaries and repository conventions.
- Failure modes, rollback path, and operator impact MUST be documented for non-trivial
  changes.
- Sandbox and harness features MUST be honest about enforcement strength; declarative
  policy MUST NOT be misrepresented as hard isolation.
- Test and production separation under `~/.dope-test` and `~/.dope` MUST remain intact.

## Delivery Workflow

1. Restore local context from repository docs and current code before planning or
   implementation.
2. Select or recut a roadmap so the delivery unit is a closed vertical slice.
3. Define compatibility, migration, verification, and observability impacts before
   editing behavior.
4. Implement minimal reversible changes that close the selected roadmap or task boundary
   without creating ad hoc side paths.
5. Run the required verification and record anything not verified.
6. Update docs, schemas, fixtures, and operator guidance in the same change whenever the
   surface changed.

## Governance

This constitution overrides conflicting local habits, ad hoc shortcuts, and template
defaults. Amendments require a documented rationale, sync updates to affected templates
and guidance docs, and an explicit semantic version bump assessment. Compliance review is
mandatory for every plan, spec, task list, implementation, and review that claims roadmap
completion.

Versioning policy is:

- MAJOR for removing or materially redefining a principle or governance rule
- MINOR for adding a new principle or materially expanding required behavior
- PATCH for clarifications that do not change required behavior

**Version**: 1.0.0 | **Ratified**: 2026-04-19 | **Last Amended**: 2026-04-19
