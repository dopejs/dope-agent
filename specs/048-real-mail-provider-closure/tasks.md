# Tasks: Real Mail Provider Closure (Feishu/Lark)

**Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md) | **Roadmap**: 63

Stories: US1 read (account/thread/message/draft), US2 send (draft/send/reply/forward), US3
diagnostics/smoke.

## Phase 1: Setup
- [X] T001 [Setup] Baseline green (mail/integrations/app suites); confirm mail adapter shim + plane.

## Phase 2: Foundational
- [X] T002 [Foundational] mail adapter shim: provider kind + stable failure class + ambiguous
  classification (mirror calendar AdapterFailure); manager passes providerKind to diagnostics.
- [X] T003 [Foundational] feishulark mail provider skeleton: adapterprovider.Handler for "mail",
  reuse existing client + token + error mapping; route all mail ops.

## Phase 3: US1 — read closure
- [X] T004 [US1] mail.ProjectAccount + ListThreads/GetThread/GetMessage + ListDrafts/GetDraft
  mapping (Feishu Mail API -> mail snapshots; identity preserved).
- [X] T005 [P] [US1] tests: read closure maps onto existing resources; expired/revoked read fails
  with stable diagnostic.

## Phase 4: US2 — send closure
- [X] T006 [US2] CreateDraft/UpdateDraft + SendMessage/SendDraft/ReplyMessage/ForwardMessage
  mapping; ambiguous send -> ambiguity channel; idempotency preserved.
- [X] T007 [P] [US2] tests: draft+send distinct operations preserve identity; ambiguous send
  recorded ambiguous; retried send no duplicate.

## Phase 5: US3 — diagnostics, wiring, smoke
- [X] T008 [US3] app wiring: mail adapter WithProviderKind(feishu_lark) when configured;
  cmd DOPE_ADAPTER_DOMAIN=mail serves real mail provider.
- [X] T009 [US3] diagnostics mapping test: auth/scope/token/rate failures -> stable reasons;
  no raw provider message/content leaks.
- [X] T010 [US3] opsreadiness mail real-account smoke with structured skip; never exposes
  credential/message content beyond redacted evidence.

## Phase 6: Polish & verification
- [X] T011 [Polish] schemas: additive if any (expect none); make daemon-contract-test.
- [X] T012 [Polish] docs note (real-account-smoke / integration-adapter-plane).
- [X] T013 [Polish] verify: build/vet/test mail, feishulark, integrations, app, opsreadiness,
  contracts; fake mail suite green.

## Dependencies
T002/T003 block provider work. T004 before T005. T006 before T007. Smoke (T010) after T006.
</content>
