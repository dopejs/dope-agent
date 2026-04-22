# Mail Integration

Status: proposed

Authority: This document is the authoritative upstream spec for the first personal mail domain slice built on the shared integrations platform.

Primary source documents:
- `docs/specs/012-personal-integrations-platform.md`
- `docs/specs/013-delivery-and-notifications.md`

## Background

Mail is not just another messaging channel. It has threads, drafts, replies, forwarding, attachments, and higher risk around unintended side effects.

## Goal

Add a production-grade mail domain that supports inspectable mail state, draft and send flows, and operator-visible side effects.

## Fixed Decisions

- read and send paths must remain distinguishable
- draft creation and final send are separate operation classes
- attachment handling must remain explicit and auditable
- shared integration readiness, account binding, canonical-default selection, and
  redacted provenance semantics come from roadmap 27 and are not redefined here
- background mail results should reuse roadmap 28 delivery targets, preferences, and
  outcome history instead of a mail-specific notification subsystem

## In Scope

- mailbox readiness through the shared integration model
- thread list and detail inspection
- draft create and update
- send, reply, and forward
- attachment metadata handling

## Out Of Scope

- full CRM mailbox automation
- autonomous cold outreach
- generalized marketing campaign tooling

## User Stories

- As a user, I can ask the agent to draft or send a message and see what actually happened.
- As an operator, I can distinguish draft creation from final send.
- As a user, I can receive results or failures from background mail workflows.

## Functional Requirements

- mail integration MUST expose account readiness and mailbox identity by reusing the
  shared integration resource, readiness vocabulary, and account-binding contract from
  `docs/specs/012-personal-integrations-platform.md`
- read, draft, send, reply, and forward MUST be separate operation classes
- sent-message side effects MUST be distinguishable from draft-only results
- attachment metadata and failures MUST remain operator-visible

## Verification Expectations

- domain tests for draft and send flows
- contract coverage for mail-side projections where applicable
- one manual or local-fixture verification path

## Definition Of Done

- the agent can inspect mail, create drafts, and send messages with truthful audit and delivery behavior

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/015-mail-integration.md 完成 phase 30 的工作`
