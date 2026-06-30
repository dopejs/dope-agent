# Feature Specification: Real Calendar Provider Closure (Feishu/Lark)

**Feature Branch**: `main`
**Created**: 2026-06-04
**Status**: Draft
**Phase / Roadmap**: Phase 60 — Roadmap 60 (Real Calendar, first real provider closure)
**Input**: User description: "结合 docs/specs/045-real-calendar-provider-closure.md 完成 phase 60 的工作"
**Upstream authority**: [docs/specs/045-real-calendar-provider-closure.md](../../docs/specs/045-real-calendar-provider-closure.md)
**Provider decision (recorded during clarification)**: **Feishu/Lark Calendar** — reuses the existing `feishu_lark` backend kind already present in the integrations diagnostics plane.

## Overview

The calendar domain (Roadmap 29) ships today with a repo-owned fake backend and a
production-shaped operation model: account projection, event inspection, busy/free
availability, and timed single-event create / update / cancel. Every calendar surface is
exercised only against the fake backend.

Phase 59 closes **one real calendar provider — Feishu/Lark — end to end** so that calendar
actions run against a user's real account through OAuth and real provider scopes, while the
existing calendar operation model, diagnostics vocabulary, live-validation classification,
and delivery truth are reused unchanged. The fake backend remains mandatory and keeps
passing; the real provider is an additional backend behind the same calendar domain, not a
second calendar execution ledger.

This phase deliberately does **not** expand calendar capability semantics. Attendee /
RSVP, recurring-event mutation, and all-day-event mutation stay out of scope (they are
Roadmaps 60–61). Phase 59 is a depth-of-trust closure, not a breadth expansion.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Connect a real Feishu/Lark calendar and inspect events (Priority: P1)

A user connects their real Feishu/Lark calendar through the credential/OAuth setup flow,
the system projects their real calendar account, and the user can list and inspect real
events and query real busy/free availability — all expressed through the existing calendar
resources.

**Why this priority**: Read-only closure is the smallest slice that proves the real
provider path works (auth, scopes, account projection, event mapping) without any
side-effecting risk. It is the foundation every other story builds on and is independently
demonstrable.

**Independent Test**: With safe Feishu/Lark credentials (or a recorded/synthetic provider
response fixture), bind a calendar integration, project the account, list events, fetch one
event, and run a busy/free query. Verify every result maps onto the existing calendar
account, event, and availability resources with no new resource shapes.

**Acceptance Scenarios**:

1. **Given** a user with valid, authorized Feishu/Lark calendar credentials, **When** they
   bind a calendar integration backed by the real provider, **Then** the calendar account
   projection reflects the real account identity and reports a healthy readiness state.
2. **Given** a connected real calendar, **When** the user lists events for a time window,
   **Then** real provider events are returned mapped onto the existing calendar event
   resource (identity, timing, timezone preserved).
3. **Given** a connected real calendar, **When** the user runs a busy/free query for a
   window, **Then** real availability is returned through the existing availability query
   resource.
4. **Given** credentials that are expired or revoked, **When** any read is attempted,
   **Then** the operation fails with a stable diagnostics reason code (not a raw provider
   error) and no partial/garbled projection is persisted.

---

### User Story 2 - Create, update, and cancel a real timed event (Priority: P2)

A user creates a timed single event on their real primary Feishu/Lark calendar, later
updates its time, and finally cancels it — each as a distinct, audited calendar operation
with idempotent, ambiguous-commit-safe write behavior.

**Why this priority**: Mutation is the highest-value calendar capability and the highest
risk (real side effects on a real account). It depends on Story 1's auth/account closure
and must preserve the existing operation ledger and write-safety guarantees.

**Independent Test**: With safe credentials (or a recorded provider write fixture), create a
timed event, update its start/end, then cancel it. Verify each produces a distinct calendar
operation with preserved event identity, that retried writes do not duplicate the event,
and that an ambiguous commit (provider acknowledged unclearly) is recorded as such rather
than silently treated as success or failure.

**Acceptance Scenarios**:

1. **Given** a connected real calendar, **When** the user creates a timed single event,
   **Then** the event is created on the real primary calendar and surfaced as a completed
   create operation with a stable event identity.
2. **Given** an existing real event, **When** the user updates its time, **Then** the real
   event is updated and recorded as a distinct update operation preserving the same event
   identity.
3. **Given** an existing real event, **When** the user cancels it, **Then** the real event
   is cancelled and recorded as a distinct cancel operation.
4. **Given** a write that is retried after a transient failure, **When** the retry runs,
   **Then** no duplicate real event is created (idempotency preserved).
5. **Given** a write whose provider acknowledgement is ambiguous, **When** the operation
   completes, **Then** the outcome is classified as ambiguous-commit with evidence, not as
   a clean success or failure.
6. **Given** a request targeting attendees, recurrence, all-day, or a non-primary calendar,
   **When** it is submitted, **Then** it is rejected with the existing out-of-scope reason
   (no partial real mutation) — Phase 59 does not expand these semantics.

---

### User Story 3 - Diagnose provider failures and review smoke / skip evidence (Priority: P3)

An operator (or release reviewer) can see Feishu/Lark calendar OAuth, scope, and token
failures mapped to the existing stable diagnostics reasons, and can inspect either
real-account smoke evidence or an explicit, reasoned skip when safe credentials are
unavailable.

**Why this priority**: Operability and release evidence are required for hosted readiness
but depend on the read/write paths existing first. They turn a working integration into a
launch-ready one.

**Independent Test**: Drive representative provider failures (auth pending, expired,
revoked, missing scope, provider unavailable) and confirm each maps to an existing
diagnostics reason code with correct retry-safety and redaction status. Run the real-account
smoke matrix; confirm it either passes against safe credentials or records a structured skip
with a reason, and that no credential or token material is logged or surfaced.

**Acceptance Scenarios**:

1. **Given** a Feishu/Lark calendar OAuth or scope failure, **When** diagnostics run,
   **Then** the failure maps to a stable existing diagnostics reason code with correct
   retry-safety and remediation owner, and provider-raw codes are redacted.
2. **Given** safe Feishu/Lark credentials are available, **When** the real-account smoke
   matrix runs, **Then** create/update/cancel live-validation rows are exercised and
   classified, with no token or credential material exposed in any output or artifact.
3. **Given** safe credentials are unavailable, expired, or revoked, **When** smoke is
   requested, **Then** an explicit structured skip with a reason is recorded and overall
   readiness can still pass because fake-backend and operational evidence pass.

---

### Edge Cases

- **Auth not yet completed**: binding a real calendar before OAuth finishes reports an
  auth-pending readiness state, not a hard error or a false-healthy projection.
- **Partial scope grant**: the user authorizes read but not write — reads succeed; writes
  fail with a stable "missing scope / permission" diagnostic, not a generic 500-style error.
- **Token expiry mid-operation**: an access token expires between request start and provider
  call — the operation surfaces a stable expired/auth diagnostic and is safe to replay.
- **Timezone / DST boundaries**: events crossing a DST transition preserve correct absolute
  start/end timing when mapped onto the existing event resource.
- **Provider rate limiting / transient unavailability**: classified as retry-safe transient
  failures, not permanent ones, so safe replay is possible.
- **Ambiguous write acknowledgement**: provider returns success-then-disconnect or an
  unparseable acknowledgement — recorded as ambiguous-commit with evidence.
- **Out-of-scope mutation attempt**: attendee/RSVP, recurrence, all-day, or alternate
  calendar requests are rejected before any real side effect.
- **Fake vs real coexistence**: switching a tenant/integration between fake and real backend
  does not corrupt or merge operation history across the two backends.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide a real Feishu/Lark calendar backend that satisfies the
  existing calendar backend contract (account projection, event list, event detail,
  busy/free, create, update, cancel) without changing that contract's shape.
- **FR-002**: The real backend MUST map provider responses onto the existing calendar
  account, event, operation, availability, and artifact resources — introducing no parallel
  or provider-specific calendar resource shapes.
- **FR-003**: Connecting a real calendar MUST flow through the existing credential/OAuth
  setup and integration-readiness model (Roadmap 46 / 31), reusing the existing readiness
  and auth-state vocabulary (not_configured, auth_pending, healthy, degraded, unavailable;
  authorized/expired/revoked).
- **FR-004**: Account binding MUST preserve real account identity and event identity across
  read and write operations, consistent with the calendar domain's identity guarantees.
- **FR-005**: Busy/free, read, create, update, and cancel MUST remain distinct operation
  classes recorded on the single existing calendar operation ledger; the real provider MUST
  NOT create a second calendar execution ledger.
- **FR-006**: OAuth, scope, and token failures MUST map to the existing stable integration
  diagnostics reasons (Roadmap 42 / 27), including reason code, retry-safety, remediation
  owner, and redaction status, reusing the `feishu_lark` diagnostics provider kind.
- **FR-007**: Side-effecting writes (create/update/cancel) MUST preserve idempotency so that
  a safe retry does not duplicate or corrupt the real event.
- **FR-008**: Side-effecting writes MUST preserve ambiguous-commit evidence so an unclear
  provider acknowledgement is recorded as ambiguous rather than coerced to success/failure.
- **FR-009**: Write outcomes MUST be classifiable through the existing live-validation matrix
  for calendar create/update/cancel tool classes.
- **FR-010**: The system MUST reject out-of-scope mutations (attendee/RSVP, recurring-event,
  all-day-event, and non-primary/alternate calendar) with the existing out-of-scope reasons
  and perform no partial real side effect.
- **FR-011**: A real-account smoke matrix MUST be runnable; it MUST run against safe
  operator-provided credentials when available, and otherwise record an explicit structured
  skip with a reason, per the real-account smoke policy.
- **FR-012**: No credential, token, or other secret material MUST be logged, reported, backed
  up, restored, or otherwise exposed in any operation result, event, diagnostic, artifact,
  or smoke output.
- **FR-013**: The existing fake calendar backend and its tests MUST remain fully functional
  and required; real-provider availability MUST NOT be a precondition for fake-backend
  verification or for overall readiness when skips are explicitly reasoned.
- **FR-014**: Scheduled/background calendar workflows MUST be able to invoke the real backend
  through the normal runtime workflow path (reusing existing delivery targets and outcome
  history), with no calendar-only notification plane introduced.

### Key Entities *(data involved — all reuse existing calendar/integration models)*

- **Real Calendar Integration (Feishu/Lark)**: an integration resource bound to the
  `feishu_lark` backend kind for the calendar domain; carries readiness, auth state, and
  provenance. Reuses the existing integration resource — no new shape.
- **Calendar Account Projection**: the inspectable real account identity and primary-calendar
  binding projected from provider responses onto the existing account resource.
- **Calendar Event**: a real timed single event mapped onto the existing event resource
  (identity, start/end, timezone).
- **Calendar Operation**: a recorded busy/free, read, create, update, or cancel action on the
  existing single operation ledger, including outcome classification and ambiguous-commit
  evidence.
- **Availability Query**: a busy/free lookup result mapped onto the existing availability
  query resource.
- **Provider Diagnostic Evidence**: redacted OAuth/scope/token failure evidence classified
  into the existing stable diagnostics reason vocabulary via the `feishu_lark` provider kind.
- **Real-Account Smoke Result / Skip**: smoke matrix evidence, or a structured skip with a
  reason, produced under the real-account smoke policy with no secret material.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Additive. A new real calendar backend implementation plugs in
  behind the existing calendar backend interface and the existing `feishu_lark` integration
  backend kind. No changes to existing calendar API request/response schemas, event payloads,
  or the operation ledger shape are expected; if any schema gains optional fields, it MUST be
  additive and backward compatible. Existing fake-backend behavior is unchanged.
- **Migration / Rollback**: No data migration expected — the real backend reuses existing
  persistence shapes (account projections, operations, artifacts). Rollback is selecting the
  fake backend (or disabling the real integration binding); fake-backend coverage and history
  remain intact. Real and fake operation histories MUST NOT merge or corrupt one another.
- **Verification Strategy**: (1) provider unit tests using recorded or synthetic Feishu/Lark
  responses for read, write, and failure mapping; (2) API/workflow tests exercising both the
  fake and the real adapter boundaries; (3) live-validation classification for create/update/
  cancel write outcomes; (4) contract tests (`make daemon-contract-test`) if any schema
  changes additively; (5) the existing fake-backend test suite continues to pass
  (`cd daemon && go test ./internal/calendar ./internal/integrations ./internal/api ...`);
  (6) manual real-account smoke where safe credentials exist, else a recorded structured skip.
- **Observability Impact**: Reuses existing calendar operation events
  (requested/completed/failed), account-projected and artifact-recorded events, and the
  existing diagnostics reason vocabulary — all extended to carry `feishu_lark`-sourced
  outcomes. No new event families are required. Operator docs (provider architecture /
  real-account smoke) should note the Feishu/Lark calendar provider closure.
- **Environment & Secrets**: Default development and CI run against the fake backend in
  `DOPE_ENV=test` with no live credentials required. Real-provider paths require
  operator-provided safe Feishu/Lark credentials/scopes and run only when explicitly
  enabled (live mode), never against prod config or live connectors without explicit intent.
  Secret material is resolved through the existing hosted-secrets/credential model and MUST
  never be logged or exposed.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can connect a real Feishu/Lark calendar and successfully list, inspect,
  and run a busy/free query against real events, with 100% of returned data expressed through
  the existing calendar resources (zero new resource shapes).
- **SC-002**: A user can create, update, and cancel a timed single event on their real
  primary calendar, each recorded as a distinct calendar operation preserving event identity.
- **SC-003**: 100% of exercised OAuth/scope/token failure modes map to a stable existing
  diagnostics reason code (no raw provider error reaches the user-facing surface) with correct
  retry-safety classification.
- **SC-004**: Retried writes produce zero duplicate real events, and every ambiguous provider
  acknowledgement is recorded as ambiguous-commit rather than coerced to success or failure.
- **SC-005**: Zero credential or token material appears in any log, event, diagnostic,
  artifact, or smoke output (verified by inspection of smoke and operation outputs).
- **SC-006**: The full existing fake-backend calendar test suite continues to pass unchanged,
  and overall readiness can pass with an explicit, reasoned real-account smoke skip when safe
  credentials are unavailable.
- **SC-007**: All create/update/cancel write outcomes are classified through the existing
  live-validation matrix.

## Assumptions

- The real provider for this phase is **Feishu/Lark Calendar** (recorded clarification
  decision), reusing the existing `feishu_lark` backend kind and diagnostics provider kind.
- The OAuth/credential setup wizard (Roadmap 46 / spec 031) and integration diagnostics
  (Roadmap 42 / spec 027) are complete and provide the auth, readiness, and diagnostics
  surfaces this phase reuses; this phase does not redefine them.
- "Timed single-event" semantics from Roadmap 29 are the closure target; attendee/RSVP,
  recurrence, and all-day mutation are explicitly deferred to Roadmaps 60–61 and rejected
  here with existing out-of-scope reasons.
- Mutations target the user's primary calendar only, consistent with Roadmap 29.
- Safe real-account credentials may be unavailable in CI; the real-account smoke policy's
  structured-skip path is the expected default outside operator-driven live runs.
- Existing persistence shapes for account projections, operations, and artifacts are
  sufficient for real-provider data; any schema additions are optional and backward
  compatible.
- The repo-owned fake backend remains the primary development and regression backend.
