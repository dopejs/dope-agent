# Tasks: Real Calendar Provider Closure (Feishu/Lark)

**Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md) | **Roadmap**: 60

Format: `[ID] [P?] [Story] Description`. `[P]` = parallelizable (different files, no ordering
dep). Stories map to spec.md user stories US1 (read), US2 (write), US3 (diagnostics/smoke).

## Phase 1: Setup

- [X] T001 [Setup] Confirm baseline: calendar + integrations + capabilities packages green;
  adapter plane (Roadmap 59) wired in `app.go` via `KURA_INTEGRATION_ADAPTER`.
- [X] T002 [Setup] Fix stale scaffolding header in spec.md (Phase 59 -> 60) and stale upstream
  pointer (044 -> 045).

## Phase 2: Foundational (blocking)

- [X] T003 [Foundational] Add `adapterprovider` package: stdio RPC serve loop dispatching to a
  `Handler` (capability Ready handshake + domain ops), reusing `adapterrpc` codec + contract
  version; deterministic redacted-diagnostic + failure-kind responses.
- [X] T004 [Foundational] Add `providers/feishulark/credential.go`: parse the scoped credential
  envelope (access token + granted scopes); fail closed when absent/empty.
- [X] T005 [Foundational] Add `providers/feishulark/client.go`: Feishu Open API HTTP client with
  injectable base URL + `*http.Client`; bearer auth; never logs token material.
- [X] T006 [Foundational] Add `providers/feishulark/errors.go`: map provider HTTP status / error
  codes to `adapterrpc.FailureKind` (auth/scope/rate_limited/unavailable/internal) and a
  redacted diagnostic carrying only stable, non-secret fields.

## Phase 3: US1 — Read closure (account, events, busy/free)

- [X] T007 [US1] feishulark.ProjectAccount: map primary calendar + timezone -> AccountProjection
  (supports inspection/busy-free/timed mutation), preserving real account identity (FR-004).
- [X] T008 [US1] feishulark.ListEvents + GetEvent: map Feishu events -> calendar.Event,
  preserving identity, absolute start/end across timezone/DST, lifecycle state (FR-002).
- [X] T009 [US1] feishulark.BusyFree: map Feishu freebusy -> AvailabilityQuery busy intervals +
  conflict count (FR-002).
- [X] T010 [P] [US1] httptest unit tests for ProjectAccount/ListEvents/GetEvent/BusyFree incl.
  expired/revoked-credential read failure mapping (FR-001 AS4, SC-001, SC-003).

## Phase 4: US2 — Write closure (create/update/cancel)

- [X] T011 [US2] feishulark.CreateEvent/UpdateEvent/CancelEvent: map timed single-event writes,
  preserving event identity across create->update->cancel (FR-002, SC-002).
- [X] T012 [US2] Calendar adapter shim: map `adapterrpc.FailureKind` -> stable operation failure
  class (auth/scope/rate_limited/unavailable) so the ledger + DiagnosticFailure carry stable
  reasons; keep ambiguous-commit classification for unconfirmed writes (FR-006, FR-008).
- [X] T013 [P] [US2] httptest write tests: create/update/cancel success; ambiguous provider ack
  (success-then-disconnect / undecodable) recorded as ambiguous_commit not success/failure;
  retried write after transient failure produces no duplicate (FR-007, FR-008, SC-004).
- [X] T014 [P] [US2] Manager test: out-of-scope mutation (attendee/recurrence/all-day/alternate
  calendar) rejected before any provider call with existing out-of-scope reason (FR-010, AS6).

## Phase 5: US3 — Diagnostics, wiring, smoke

- [X] T015 [US3] App wiring: `feishu_lark` secret-backed `IntegrationCredentialFetcher` via the
  Roadmap 37 secret path; install on the adapter client; select real provider when configured.
- [X] T016 [US3] `cmd/kura-integration-adapter`: `KURA_ADAPTER_PROVIDER=feishu_lark` runs the
  real calendar provider serve loop; default stays the reference skeleton.
- [X] T017 [US3] Diagnostics mapping test: representative OAuth/scope/token/rate-limit/unavailable
  failures map to stable existing reason codes with correct retry-safety + redaction; no raw
  provider code leaks (FR-006, SC-003, SC-005).
- [X] T018 [US3] `opsreadiness` calendar real-account smoke: create/update/cancel rows when safe
  credentials available, else explicit structured skip with reason; never exposes credential
  material; readiness still passes on reasoned skip (FR-011, FR-012, SC-006).
- [X] T019 [P] [US3] Live-validation: confirm calendar create/update/cancel write outcomes are
  classified through the existing live-validation matrix for the adapter path (FR-009, SC-007).

## Phase 6: Polish & verification

- [X] T020 [Polish] Additive schema: provider-kind on the integration-adapter capability schema
  (if needed); run `make daemon-contract-test`.
- [X] T021 [Polish] Docs: note Feishu/Lark calendar provider closure in provider-architecture +
  real-account-smoke docs.
- [X] T022 [Polish] Full verification: `go build ./...`; `go test` for calendar, integrations,
  providers/feishulark, adapterprovider, capabilities, opsreadiness, app; fake-backend suite
  unchanged.

## Dependencies

T003–T006 block all provider work. T007–T009 (read) precede T011 (write reuses client). T012
depends on T006/T011. T015–T016 depend on T003–T006. Smoke (T018) depends on T011+T012.
</content>
