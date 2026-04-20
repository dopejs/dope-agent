---
name: specs-phase-splitter
description: Use when turning roadmap phases or architecture follow-on work into standalone upstream spec documents under docs/specs, or when preparing authoritative docs/specs inputs for later speckit-specify work.
---

# Specs Phase Splitter

Use this skill when the user wants to:

- split a roadmap phase into one or more standalone spec docs
- prepare `docs/specs/*.md` as upstream product specs before running `speckit-specify`
- reorganize follow-on phases so future speckit work starts from one authoritative spec

## Goal

Produce stable, standalone `docs/specs` documents that capture enough information for a
later `/speckit-specify` run to start with minimal ambiguity.

## Required Inputs

Read these first:

- `docs/runtime/daemon-roadmaps.md`
- `docs/harness/harness-architecture.md`
- any existing `docs/specs/*.md` documents relevant to the phase

Read implementation-facing docs only if they materially affect scope or dependencies.

## Output Location

Write upstream specs under `docs/specs/`.

Number them sequentially with three digits:

- `007-...`
- `008-...`
- `009-...`

These numbers are independent from branch names and `specs/<NNN>-...` speckit feature
directories.

## What Each docs/specs File Must Contain

Each document should be strong enough to serve as the main input to a later
`/speckit-specify` run.

Include:

- title
- status
- authority statement
- primary source documents
- background
- goal
- fixed decisions
- dependencies on already completed phases
- in-scope and out-of-scope boundaries
- operator or user problems to solve
- user stories
- functional requirements
- compatibility and operational notes
- verification expectations
- definition of done
- recommended `/speckit-specify` input

## Splitting Rules

1. Do not keep extending a completed roadmap slice with hidden follow-on work.
2. Split catalog management, transport expansion, and orchestration into separate docs if
   they have different execution surfaces, risks, or verification stories.
3. Keep each doc roadmap-closed. A later implementation phase should be able to finish the
   slice without absorbing adjacent roadmap work.
4. Record fixed decisions explicitly so later `speckit-specify` runs do not reopen already
   settled scope unless the user asks.

## Required Follow-Through

After adding or updating `docs/specs`:

- update `docs/runtime/daemon-roadmaps.md` to link the detailed spec from the roadmap item
- update `docs/specs/README.md` if numbering or mappings changed
- keep `docs/harness/harness-architecture.md` sequencing aligned if the phase ordering
  changed

## Style

- Write for future planning, not implementation detail
- Be specific about scope boundaries
- Prefer concise, durable statements over verbose explanation
- Optimize for “can later feed this into `/speckit-specify` directly”
