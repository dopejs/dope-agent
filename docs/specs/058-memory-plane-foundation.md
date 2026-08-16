# 058 — Memory Plane Foundation

## Roadmap Context

Roadmap 78 (first slice of the context/knowledge/memory program). Gated on
the Roadmap 77 public launch-gate ship decision; this document is the
design-phase deliverable authorized while the gate is closed
(`docs/runtime/daemon-roadmaps.md`, Roadmap 78+).

## Goal

Give the daemon a first-class memory plane: typed, attributable, scoped, and
reversible memory records with an explicit write policy, retention rules, and
an inspection API — the substrate later slices (context engineering,
knowledge retrieval) consume. No behavior of any existing surface changes
when the memory plane holds no records.

## In Scope

- A `memory` domain crate owning the record model, write policy, and
  lifecycle (create / supersede / revoke / expire).
- Memory record types: `observation` (facts the agent noticed),
  `preference` (user-stated), `decision` (operator/agent choices with
  rationale), `reference` (pointers to external resources). Each type has a
  distinct retention default and write-policy strictness.
- Attribution: every record carries `writtenBy` (actor kind + id), the
  originating scope (`tenantId`, `threadId`, `runId`, source event id where
  applicable), and a `sourceLink` back to the evidence it derives from.
- Write policy: writes are proposals evaluated by a policy hook (default:
  operator-visible auto-accept for `preference`/`reference`, approval-gated
  for `observation`/`decision` above a configurable volume, always-rejected
  for records without attribution). Policy decisions are auditable events.
- Reversibility: superseding writes chain (`supersedesRecordId`); revocation
  tombstones a record without deleting the audit chain; retention expiry is
  a recorded transition, not a silent delete.
- Store: a `memory_records` table (tenant-partitioned per the
  schema-inventory standard) with the versioned-migration flow; manager
  restore on boot.
- API: `GET/POST /v1/memory/records`, `GET /v1/memory/records/{id}`,
  `POST /v1/memory/records/{id}/revoke`, list filters by type/scope/state.
  Protected() like every resource family; tenant-scoped reads/writes.
- Events: `memory.record_written`, `memory.record_superseded`,
  `memory.record_revoked`, `memory.record_expired`.
- Schemas under `schemas/api/` + contract tests; SDK methods; operator-shell
  read surface (list + detail + revoke).

## Out Of Scope

- Retrieval/ranking/embedding of any kind (059/060).
- Automatic memory writes from conversations (the write policy exists, but
  no producer is wired in this slice; producers arrive with 059+).
- Cross-tenant or shared memory.
- A knowledge graph as the storage model (product-outline non-goal).

## Fixed Decisions

- Memory writes must be attributable, scoped, and reversible
  (product-outline binding constraint). A record without attribution is
  rejected at the API boundary, not down-graded.
- Recalled memory is evidence, not truth: the record model carries
  `sourceLink` so consumers can cite back; nothing in this plane asserts
  authority over the linked source.
- The write path is policy-gated from day one; there is no unguarded write
  API to retrofit later.
- Storage is SQLite via the existing store crate; no new storage engine.

## Dependencies On Completed Phases

- Tenant partitioning + schema inventory (Roadmaps 34/35).
- Policy/approval plane (Roadmap 4) for approval-gated writes.
- Thread/session lifecycle (Roadmap 54) for scope references.
- Events plane and contract-test pipeline (Roadmaps 1/5).

## Operator Stories

- As an operator I can list every memory record the system holds about my
  tenant, see who wrote each one and from what evidence, and revoke any of
  them; the revocation is immediate and auditable.
- As an operator I can require approval for a class of memory writes and
  see pending proposals in the existing approvals surface.

## Functional Requirements

- FR-001: records are immutable after write; changes happen by supersede.
- FR-002: every lifecycle transition emits an event and is persisted.
- FR-003: list/get never return revoked/expired records unless
  `includeInactive=true`.
- FR-004: write policy hooks are pluggable with a fail-closed default when
  a configured hook errors.
- FR-005: retention sweeps are idempotent and restart-safe.

## Verification Expectations

- Behavioral tests for policy accept/approve/reject paths, supersede
  chains, revoke, retention expiry, restart restore.
- Contract tests for all new schemas; route-parity gate updated.
- A live-boot check on the test environment data dir.

## Definition Of Done

- The memory plane exists end-to-end (crate, store, API, events, SDK,
  operator-shell read surface) with zero producers wired, all tests green,
  and the write-policy audit chain demonstrated in the quickstart.

## Recommended Start Prompt

Implement `docs/specs/058-memory-plane-foundation.md` as Roadmap 78 phase 1;
update `schemas/` first, then the domain crate, store migration, API,
SDK, and operator-shell surfaces, closing with contract + behavioral tests.
