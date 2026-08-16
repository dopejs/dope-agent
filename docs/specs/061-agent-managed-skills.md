# 061 — Agent-Managed Skills

## Roadmap Context

Roadmap 81 (context/knowledge/memory program, slice 4). Depends on the
operator catalog (Roadmap 68) and the sandbox skill execution plane
(Roadmap 19).

## Goal

Let the agent propose new or revised skills that become usable only through
the existing operator-approval and catalog lifecycle — agent-authored,
operator-governed.

## In Scope

- A skill-proposal resource: agent-side drafting APIs producing a proposal
  (manifest + content + declared sandbox requirements + provenance of the
  conversation/run that motivated it).
- Review workflow on the policy/approval plane: operator sees the diff,
  the declared requirements, and the motivating evidence; approval
  publishes the skill into the catalog (Roadmap 68 versions/trust tiers);
  rejection archives the proposal with the reason.
- Runtime guard: proposed-but-unapproved skills are not loadable; the
  skills registry only consumes catalog-published versions.
- Events, schemas, SDK, operator-shell review surface.

## Out Of Scope

- Autonomous approval, self-modifying behavior without review (product
  non-goal), and skill execution changes (the sandbox plane is reused
  as-is).

## Fixed Decisions

- The catalog remains the only publication path; agent proposals never
  bypass version/trust/rollback semantics.
- Every published agent-authored skill records its motivating evidence
  links permanently.

## Verification / Definition Of Done

- Behavioral tests: propose → approve → publish → execute, propose →
  reject → archived, unapproved-skill load refusal; contract tests; an
  end-to-end quickstart.
