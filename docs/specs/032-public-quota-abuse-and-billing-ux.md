# Public Quota Abuse And Billing UX

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 47, the public
quota, abuse-limit, and billing experience slice.

Primary source documents:
- `docs/specs/023-billing-quotas-and-usage-accounting.md`
- `docs/specs/021-tenant-aware-operator-shell-and-sdk.md`
- `docs/runtime/hosted-billing-quotas.md`

## Background

Quota enforcement exists, but a public product also needs users to understand usage,
limits, denials, and recovery actions. Otherwise quota failures appear as arbitrary agent
failures and increase support load.

## Goal

Expose quota, abuse-limit, billing-plan, denial, and usage-recovery behavior through
tenant-scoped product surfaces.

## Fixed Decisions

- Billing UX is not payment checkout in this slice.
- Quota denial must be product-visible before expensive or side-effecting work starts.
- Abuse limits are separate from normal plan quotas when the distinction matters.
- This roadmap does not change quota accounting semantics unless a UX gap reveals a
  contract bug.

## Dependencies On Completed Phases

- Roadmap 38: Billing, Quotas, And Usage Accounting
- Roadmap 45: Hosted Signup And Tenant Activation

## In Scope

- usage and quota dashboard projection
- quota denial detail views
- abuse-limit and temporary restriction messaging
- plan and quota override visibility
- support evidence for quota-related failures
- SDK and web shell updates

## Out Of Scope

- payment-provider checkout
- invoices, taxes, or revenue recognition
- cross-tenant pooled quota
- model marketplace pricing

## Operator Or User Problems To Solve

- Users need to know why work was denied and what limit was hit.
- Operators need support evidence for quota and abuse-limit disputes.

## User Stories

- As a user, I can see my current plan, important limits, and recent usage.
- As a user, I can inspect why a run, workflow, or live validation was denied.
- As support, I can export redacted quota evidence for a tenant.

## Functional Requirements

- The system MUST expose tenant-scoped quota status and recent usage summaries.
- Quota denials MUST link to the source operation and stable denial reason.
- Abuse restrictions MUST be distinguishable from ordinary quota exhaustion.
- The UI MUST show recovery actions such as wait, reduce scope, request override, or
  contact support.
- Quota and abuse views MUST be permission-gated.

## Compatibility And Operational Notes

Existing billing resources remain authoritative. This roadmap adds product projections,
not a new accounting ledger.

## Verification Expectations

- API, SDK, and web tests for quota dashboard and denial detail.
- Permission tests for billing visibility.
- Regression tests proving denials remain fail-closed before side effects.
- Manual `KURA_ENV=test` walkthrough for exhausted quota and abuse restriction.

## Definition Of Done

- Users and support can understand quota-related failures from product surfaces.
- Public hosted operation can enforce limits without appearing broken.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/032-public-quota-abuse-and-billing-ux.md 完成 phase 47 的工作`
