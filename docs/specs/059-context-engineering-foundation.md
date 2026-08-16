# 059 — Context Engineering Foundation

## Roadmap Context

Roadmap 79 (context/knowledge/memory program, slice 2). Gated on the
Roadmap 77 ship decision; depends on 058 (memory plane).

## Goal

Deterministic, inspectable context assembly: a daemon-owned pipeline that
builds each dispatch's context from typed sources (thread continuity,
workspace bindings, persona, memory records) under an explicit budget, and
records exactly what was included and why.

## In Scope

- A `context` domain crate: source registry, budget model (per-source token
  allowances), assembly order, and an `AssemblyRecord` capturing every
  included/excluded item with its reason and source link.
- Sources in this slice: thread continuity previews (Roadmap 55), persona
  profile (Roadmap 57), workspace bindings (Roadmap 58), and memory records
  (058) — recalled memory enters as citable evidence, never silently.
- Chat/dispatch integration behind an explicit per-request opt-in flag;
  default behavior is unchanged.
- `GET /v1/context/assemblies/{id}` inspection API + events
  (`context.assembled`), schemas, SDK, contract tests.

## Out Of Scope

- Retrieval/ranking beyond deterministic source rules (060).
- Automatic learning of assembly weights; budgets are configuration.

## Fixed Decisions

- Assembly is deterministic given the same inputs; no model-driven choice
  inside the pipeline (control-plane determinism principle).
- Every assembled context is reconstructable after the fact from its
  AssemblyRecord (inspectable-under-failure principle).

## Verification / Definition Of Done

- Behavioral tests per source and for budget-overflow exclusion ordering;
  an end-to-end chat dispatch showing the assembly record; contract tests;
  unchanged behavior with the flag off.
