# Implementation Plan: Mail Integration

**Branch**: `015-mail-integration` | **Date**: 2026-04-23 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/015-mail-integration/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Add a first-class daemon-owned mail domain on top of the shared integrations, runtime,
workflow, scheduler, and delivery planes. The design closes roadmap 30 by introducing a
dedicated `daemon/internal/mail` package that can project mailbox identity from the
shared integration substrate, inspect mailbox threads and message detail, create and
update drafts, send either direct messages or existing drafts, and execute reply and
forward flows with truthful separation between draft-only and sent-message outcomes.
Verification stays in `DOPE_ENV=test` by reusing the shared `fake_local` backend kind
while keeping deterministic mailbox state in a dedicated daemon-owned fake mail backend,
rather than requiring live third-party mail accounts.

## Technical Context

**Language/Version**: Go 1.24.0; Markdown docs; JSON Schema contracts  
**Primary Dependencies**: `daemon/internal/api`, new `daemon/internal/mail`,
`daemon/internal/app`, `daemon/internal/runtime`, `daemon/internal/orchestration`,
`daemon/internal/scheduler`, `daemon/internal/integrations`,
`daemon/internal/delivery`, `daemon/internal/policy`, `daemon/internal/events`,
`daemon/internal/store`, `daemon/internal/contracts`, existing auth wiring, and the
shared repo-owned `fake_local` backend contract plus a dedicated mail fake backend for
mail-domain verification  
**Storage**: SQLite daemon state with additive `mail_accounts`, `mail_operations`, and
`mail_artifacts` persistence plus additive workflow-step, tool-call, schedule-attempt,
and delivery linkage for mail-operation summaries; full binary attachment storage is out
of scope in phase 30  
**Testing**: `go test ./internal/mail ./internal/api ./internal/store ./internal/app ./internal/runtime ./internal/orchestration ./internal/scheduler ./internal/integrations ./internal/delivery ./internal/policy ./internal/contracts`,
`make daemon-contract-test`, targeted mail-route and workflow regressions, and one manual
`DOPE_ENV=test` walkthrough using the fake mail backend plus `test_sink` delivery  
**Target Platform**: macOS/Linux local daemon in `DOPE_ENV=test` by default, using the
existing localhost HTTP API, SQLite store, and operator-authenticated `/v1/*` control
plane  
**Project Type**: Go daemon and harness control-plane service with schema-backed HTTP
and event contracts  
**Performance Goals**: inspect mailbox projection, thread detail, message detail, or
draft detail from persisted local state in `<=500 ms`; persist and project one
draft/send/reply/forward result in `<=1 s` after backend completion on local test
hardware; deliver a background mail result through the shared delivery plane in `<=2 s`
after the operation reaches a terminal state excluding connector latency  
**Constraints**: roadmap 30 MUST reuse phase 27 integration readiness and canonical
default semantics plus phase 28 delivery targets and outcome history; read, draft,
direct send, send-existing-draft, reply, and forward remain distinct operation classes;
new outbound mail requires recipients explicitly provided in the current request; full
attachment upload/download stays out of scope; unresolved attachment references block
final send; background workflows may finalize send only when they explicitly declare
send-side-effect permission; existing non-mail behavior remains backward compatible  
**Scale/Scope**: one operator-managed daemon, low tens of mail integrations and mailbox
projections, low hundreds of mailbox inspections or outbound actions per day, one
repo-owned fake mail backend plus one shared delivery sink sufficient to close roadmap
30 without live external dependencies

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Roadmap closure: PASS. This plan closes roadmap 30 only: inspectable mailbox account
  projection, thread and message inspection, draft create and update, direct send,
  send-existing-draft, reply, forward, truthful attachment metadata and failure truth,
  normal schedule/workflow execution, and shared delivery reuse. Tasks and reminders,
  CRM automation, campaign tooling, and full attachment transfer remain out of scope.
- Production-grade change control: PASS. The design adds a dedicated
  `daemon/internal/mail` plane and additive API, schema, event, runtime-projection, and
  store changes rather than refactoring integrations, delivery, or scheduler broadly.
  Rollback is a single change-set revert of mail-specific routes, persistence,
  projections, and docs while leaving phases 27, 28, and 29 behavior intact.
- Contracts and auditability: PASS. The plan names additive mail account, thread,
  message, draft, operation, artifact, and background-delivery surfaces together with
  the schema, event, and doc updates required to keep mailbox selection, send path,
  attachment failure truth, and delivery linkage inspectable.
- Verification and observability: PASS. The design requires targeted package, contract,
  workflow, schedule, and fake-backend regressions plus one manual `DOPE_ENV=test`
  walkthrough. Operator-visible account, operation, artifact, and delivery resources
  replace backend guesswork or raw provider logs as the source of truth.
- Environment and secrets: PASS. Local planning and later verification stay in
  `DOPE_ENV=test`; the repo-owned fake mail backend avoids live mail credentials; any
  real connector or token use remains optional, operator-owned, redacted, and
  environment-scoped.

Post-design re-check:

- PASS. The design remains roadmap-closed to the first mail slice and does not drift
  into reminders, CRM automation, campaigns, or broad attachment-transfer products.
- PASS. Mail execution remains on the existing runtime, workflow, schedule, and
  delivery planes with additive domain records rather than a second mail-only execution
  ledger.
- PASS. Integration readiness, mail execution, and delivery outcomes remain distinct
  truths, which preserves operator auditability under partial failure.

## Project Structure

### Documentation (this feature)

```text
specs/015-mail-integration/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── mail-domain-surfaces.md
└── tasks.md
```

### Source Code (repository root)

```text
daemon/
├── internal/
│   ├── api/
│   ├── app/
│   ├── contracts/
│   ├── delivery/
│   ├── events/
│   ├── integrations/
│   ├── mail/
│   ├── orchestration/
│   ├── policy/
│   ├── runtime/
│   ├── scheduler/
│   └── store/
└── go.mod

schemas/
├── api/
└── events/

docs/
├── harness/
└── runtime/

AGENTS.md
```

**Structure Decision**: Keep mailbox account projection, backend abstraction, thread and
message inspection, draft handling, outbound send paths, attachment metadata truth, and
fake-backend verification support in a new `daemon/internal/mail` package.
`daemon/internal/api` exposes additive mail account, thread, message, draft, and
operation routes. `daemon/internal/runtime`, `daemon/internal/orchestration`, and
`daemon/internal/scheduler` remain the owners of run, workflow, and schedule truth while
gaining additive mail-operation linkage. `daemon/internal/integrations` continues to own
readiness and canonical-default mailbox binding; `daemon/internal/delivery` remains the
owner of background-result routing. `daemon/internal/store` persists mail-domain
documents and linkage indexes. `schemas/` and `docs/` carry the new contract and
operator-guidance surfaces. `AGENTS.md` should point at this plan for downstream task
generation.

## Complexity Tracking

No constitution violations remain. The design avoids reusing integration probes as the
mail domain API, avoids collapsing delivery truth into mail operation results, avoids
live third-party mail dependencies for roadmap closure, and avoids premature support for
CRM automation, marketing campaigns, broad attachment transfer, or inferred recipients
for new outbound mail.

## Implementation Notes

- Add daemon-owned mail resources rather than deriving mail truth ad hoc from
  integration probes or live backend reads at response time.
- Reuse an explicit request-scoped `integrationId` when provided on mail reads or
  writes; otherwise resolve the healthy or degraded canonical default integration for
  the mailbox account and surface that choice in mail-operation truth.
- Persist mailbox account projection separately from raw integration readiness so the
  domain can expose mailbox identity and capability metadata without redefining phase 27
  resource ownership.
- Treat mail work as explicit operation classes: `list_threads`, `get_thread`,
  `get_message`, `list_drafts`, `get_draft`, `create_draft`, `update_draft`,
  `send_message`, `send_draft`, `reply_message`, and `forward_message`. Each operation
  stores source linkage, mailbox selection, thread or message or draft identity when
  present, send path when present, and terminal truth.
- For new outbound mail that is not reply or forward, require recipients explicitly
  provided in the current request. Do not infer new recipients from unrelated mailbox
  context, prior drafts, or background workflow state.
- Keep attachment handling intentionally narrow in phase 30: attachment references,
  metadata, and failure truth are operator-visible, but full attachment upload/download
  behavior and generalized transfer workflows stay out of scope.
- Block final send when required attachment references cannot be resolved or validated;
  preserve explicit failure or draft-only truth instead of degraded send.
- Gate background final-send behavior behind explicit workflow declaration such as
  `allowSendSideEffects`; absent that declaration, background mail work may inspect
  mailbox state or manage drafts but must not finalize send.
- Reuse the shared `fake_local` backend kind for readiness and probe semantics while a
  dedicated deterministic mail fake backend owns mailbox projection, seeded inbound
  state, direct-send, send-existing-draft, reply, forward, and unresolved-attachment
  scenarios. One healthy mailbox projection, one seeded inbound thread, one direct-send
  path, one send-existing-draft path, one reply path, one forward path, and one
  unresolved attachment failure are sufficient to close roadmap 30.

## Automated Verification

- `cd daemon && go test ./internal/mail ./internal/api ./internal/store ./internal/app ./internal/runtime ./internal/orchestration ./internal/scheduler ./internal/integrations ./internal/delivery ./internal/policy ./internal/contracts`
- `make daemon-contract-test`
- `cd daemon && go test ./internal/api -run 'TestMailAccountRoutesProjectMailboxIdentity|TestMailRoutesSeparateDraftFromSendAndPreserveSendPath|TestMailRoutesRequireExplicitRecipientsForNewOutbound|TestScheduledMailWorkflowRequiresExplicitSendPermissionAndPreservesDeliveryTruth' -count=1`
- `cd daemon && go test ./internal/mail -run 'TestFakeMailBackendProvidesThreadMessageAndDraftState|TestMailManagerSupportsDirectSendAndSendDraftWithDistinctOperationClasses|TestMailManagerBlocksFinalSendWhenAttachmentReferenceFails|TestMailManagerRejectsBackgroundSendWithoutExplicitPermission' -count=1`

These commands are expected to cover:

- mailbox account projection from integration readiness and canonical-default binding
- thread and message inspection without send side effects
- draft create and update behavior with durable draft identity
- direct send, send-existing-draft, reply, and forward truth with explicit send-path
  distinction
- attachment metadata truth and blocked-send behavior for unresolved attachment
  references
- additive linkage from mail operations to runs, workflow steps, schedules, and delivery
  outcomes
- fake-backend restart safety for persisted mailbox account, operation, and artifact
  truth

Observed on `2026-04-23`:

- `cd daemon && go test ./...` passed
- `cd daemon && go test ./internal/api ./internal/runtime ./internal/store ./internal/mail ./internal/integrations ./internal/app ./internal/contracts` passed
- `make daemon-contract-test` passed

## Manual Verification

- `make daemon-run-test`
- `make daemon-test-status`
- pair or reuse a local bearer token
- register one fake mail integration, promote it as canonical default, and mark it
  healthy
- inspect `/v1/mail/accounts` to confirm mailbox identity, readiness, and selection truth
- inspect `/v1/mail/threads`, `/v1/mail/messages/{messageId}`, and `/v1/mail/drafts`
  without creating any send side effects
- create one draft, update it, send one direct new message, send one existing draft,
  reply to one existing message, and forward one message while verifying operator-visible
  distinction among operation class, send path, and draft-only versus sent outcomes
- attempt a new outbound send with no explicit recipients and confirm the system fails
  truthfully instead of inferring recipients
- attempt a send with an unresolved attachment reference and confirm the system blocks
  final send and records attachment failure truth
- configure one `test_sink` delivery target and preference, then execute one scheduled or
  workflow-driven mail task with send permission disabled and one with send permission
  enabled to confirm background gating and delivery linkage

Observed on `2026-04-23`:

- manual test daemon startup and health check passed on `127.0.0.1:19192`
- fake integration `mail-fake-primary` projected mailbox identity and seeded thread or
  message or draft inspection truth
- draft create or update, direct send, send draft, reply draft, and forward send all
  returned truthful `operationClass`, `resultMode`, and `sendPath` values
- foreground workflow send without `allowSendSideEffects` failed with
  `send_permission_required`
- one-time scheduled background send with `allowSendSideEffects: true` produced linked
  `mailOperationSummaries`, `mailOperationIds`, and delivery outcome

## Residual Risks

- Background delivery emission remains intentionally limited to true background flows.
  Foreground workflows attached to an operator session do not emit delivery outcomes.
- `GET /v1/mail/accounts` assumes callers do not race readiness updates; if operators
  need mixed healthy and not-configured mail integrations listed together, that should be
  handled as a follow-on product decision rather than inferred here.
- Attachment handling is metadata-only in phase 30. There is still no binary attachment
  upload or download path.

## Rollback Notes

- Revert the additive `daemon/internal/mail` package, API handlers, runtime or workflow
  projections, and schema files as one change set.
- The mail SQLite migration is additive. Rolling back code leaves inert tables
  `mail_accounts`, `mail_operations`, and `mail_artifacts`; no data rewrite is required
  for rollback.
