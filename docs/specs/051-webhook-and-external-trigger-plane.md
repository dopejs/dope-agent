# Webhook And External Trigger Plane

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 66, the webhook
and external event trigger plane.

Primary source documents:
- `docs/specs/010-scheduled-tasks-and-wakeups.md`
- `docs/specs/050-routine-builder.md`
- `docs/runtime/operator-trust-model.md`

## Background

Scheduled wakeups exist, but public personal agents also need external systems to trigger
bounded workflows. Webhooks are trigger-plane work and must stay separate from chat
connectors, provider integrations, and memory.

## Goal

Add tenant-scoped webhook endpoints that can safely trigger runs, workflows, or routines
with authentication, replay protection, quota, audit, and delivery linkage.

## Fixed Decisions

- Webhooks are trigger resources, not channel connectors.
- Every webhook must authenticate and resolve tenant context.
- Payload handling must be bounded and redacted.
- This roadmap does not add arbitrary public unauthenticated automation.

## Dependencies On Completed Phases

- Roadmap 25: Scheduled Tasks And Wakeups
- Roadmap 65: Routine Builder
- Roadmap 38: Billing, Quotas, And Usage Accounting

## In Scope

- webhook endpoint resource
- signing secret or token authentication
- payload schema or sample projection
- idempotency and replay protection
- trigger-to-run/workflow/routine linkage
- audit, quota, and delivery behavior
- SDK and web management

## Out Of Scope

- generic third-party app marketplace
- inbound chat message connectors
- unbounded payload storage
- memory ingestion from webhook payloads

## Operator Or User Problems To Solve

- Users need external events to trigger routines safely.
- Operators need to inspect webhook failures, replays, and quota denials.

## User Stories

- As a user, I can create a webhook that triggers a routine.
- As an operator, I can rotate a webhook secret and inspect failed deliveries.
- As support, I can confirm whether duplicate payloads were suppressed.

## Functional Requirements

- Webhook requests MUST authenticate and resolve tenant context.
- The system MUST enforce idempotency or replay protection.
- Payloads MUST be size-bounded and redacted in logs and events.
- Webhook triggers MUST create normal runtime or routine execution records.
- Quota and permission checks MUST run before execution starts.

## Compatibility And Operational Notes

Webhook trigger records should align with schedule dispatch attempts where possible while
preserving external source identity.

## Verification Expectations

- API tests for create, rotate, disable, and inspect.
- Security tests for missing auth, bad signature, replay, oversized payload, and
  cross-tenant attempts.
- Runtime tests proving trigger linkage and quota denial.

## Definition Of Done

- External systems can safely wake the hosted agent without abusing channel connector or
  integration semantics.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/051-webhook-and-external-trigger-plane.md 完成 phase 66 的工作`
