# Feature Specification: Real Mail Provider Closure (Feishu/Lark)

**Feature Branch**: `main`
**Created**: 2026-06-30
**Status**: Draft
**Phase / Roadmap**: Phase 63 — Roadmap 63
**Upstream authority**: [docs/specs/048-real-mail-provider-closure.md](../../docs/specs/048-real-mail-provider-closure.md)
**Provider decision (recorded during clarification)**: **Feishu/Lark Mail** — reuses the `feishu_lark` backend kind and the integration adapter plane (Roadmap 59), continuing the real provider pattern from Roadmap 60.

## Overview

The mail domain (Roadmap 30) ships with a repo-owned fake backend and production-shaped
operation truth: account/thread/message/draft projection, draft create/update, direct send,
send-existing-draft, reply, and forward. Every mail surface is exercised only against the fake.

Roadmap 63 closes **one real mail provider — Feishu/Lark — end to end**, implemented as an
adapter on the external integration adapter plane (not an in-process backend), so mail actions
run against a real mailbox through OAuth and real scopes while the existing mail operation
model, diagnostics vocabulary, live-validation classification, and delivery truth are reused
unchanged. The fake backend stays mandatory and green; the real provider is an additional
backend behind the same mail domain, never a second mail execution ledger.

Full attachment upload/download stays out of scope (Roadmap 64). Sends remain side effects
requiring explicit permission and evidence.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Connect a real mailbox and inspect mail (Priority: P1)
**Acceptance Scenarios**:
1. Bind a real Feishu/Lark mailbox; account projection reflects real identity + healthy readiness.
2. List threads / get thread / get message map onto existing mail resources (identity preserved).
3. List/get drafts map onto the existing draft resource.
4. Expired/revoked credentials fail reads with a stable diagnostic, no partial projection.

### User Story 2 - Draft and send mail (Priority: P2)
**Acceptance Scenarios**:
1. Create/update a draft on the real mailbox, recorded as distinct operations.
2. Direct send / send existing draft / reply / forward each produce a distinct send operation
   with preserved message identity.
3. A retried send produces no duplicate message (idempotency preserved).
4. An ambiguous send acknowledgement is recorded as ambiguous-commit with evidence.
5. Sends are gated by the existing per-action approval (externally-visible side effect).

### User Story 3 - Diagnose failures and review smoke evidence (Priority: P3)
**Acceptance Scenarios**:
1. OAuth/scope/token/rate-limit failures map to stable existing diagnostics reasons; provider-raw
   codes redacted.
2. Real-account smoke runs against safe mailbox credentials when available, else records an
   explicit structured skip; no message content beyond redacted evidence policy is exposed.

### Edge Cases
- Auth pending, partial scope (read but not send), token expiry mid-send, rate limiting,
  ambiguous send ack, fake/real coexistence — all handled per the calendar closure pattern.
- Attachment-bearing send with unresolved attachment references is blocked (full attachment
  transfer is Roadmap 64).

## Requirements *(mandatory)*

### Functional Requirements
- **FR-001**: A real Feishu/Lark mail backend MUST satisfy the existing mail backend contract
  (account/thread/message/draft projection, draft create/update, send/send-draft/reply/forward)
  without changing its shape, implemented as an adapter on the plane.
- **FR-002**: Provider responses MUST map onto existing mail resources — no parallel shapes.
- **FR-003**: Connecting MUST flow through the existing credential/OAuth + readiness model and
  the existing readiness/auth vocabulary.
- **FR-004**: Account/thread/message/draft identities MUST be preserved across read and send.
- **FR-005**: Send operation classes MUST remain distinct on the single mail operation ledger;
  the real provider MUST NOT create a second mail execution ledger.
- **FR-006**: OAuth/scope/token failures MUST map to existing stable diagnostics reasons
  (reason code, retry-safety, remediation owner, redaction) via the `feishu_lark` kind.
- **FR-007**: Sends MUST preserve idempotency (safe retry no duplicate) and ambiguous-commit
  evidence (unclear ack recorded ambiguous, not coerced).
- **FR-008**: Send outcomes MUST be classifiable through the existing live-validation matrix.
- **FR-009**: A real-account smoke MUST be runnable against safe credentials or record an
  explicit structured skip; no message content beyond redacted evidence is exposed.
- **FR-010**: No credential/token material MUST be exposed in any result/event/diagnostic/
  artifact/smoke output.
- **FR-011**: The existing fake mail backend and its tests MUST remain functional and required.

### Key Entities (reuse existing mail/integration models)
- Real Mail Integration (Feishu/Lark), Account Projection, Thread/Message/Draft Snapshot,
  Mail Operation (send/reply/forward/draft on the single ledger), Provider Diagnostic Evidence,
  Real-Account Smoke Result / Skip.

## Compatibility & Operational Impact *(mandatory)*
- **Compatibility**: Additive. New real backend behind the existing mail backend interface and
  `adapter_rpc` backend kind; no mail API/event/ledger shape changes expected.
- **Migration / Rollback**: No migration; rollback = fake backend / disable the real binding.
- **Verification**: provider adapter tests (account/thread/message/draft/send/reply/forward over
  synthetic responses); live-validation send classification; API/workflow tests; fake suite
  green; real-account smoke or structured skip.
- **Observability**: reuse mail operation events + diagnostics vocabulary, extended for
  `feishu_lark`. No new event families.
- **Environment & Secrets**: fake backend default in test; real provider only with explicit
  operator credentials; secrets via the hosted-secrets model, never logged.

## Success Criteria *(mandatory)*
- **SC-001**: Connect + inspect threads/messages/drafts on a real mailbox, 100% via existing
  resources.
- **SC-002**: Draft/send/reply/forward on the real mailbox, each a distinct operation preserving
  identity.
- **SC-003**: 100% of exercised auth/scope/token failures map to a stable reason code.
- **SC-004**: Retried sends produce zero duplicates; ambiguous acks recorded ambiguous.
- **SC-005**: Zero credential/message-content leakage beyond redacted evidence.
- **SC-006**: Fake mail suite green; readiness passes with a reasoned smoke skip.
- **SC-007**: Send outcomes classified through the live-validation matrix.

## Assumptions
- Provider is Feishu/Lark Mail; OAuth/credential + diagnostics surfaces from Roadmaps 46/42 are
  reused. Full attachment transfer is Roadmap 64. Fake backend remains primary dev/regression.
