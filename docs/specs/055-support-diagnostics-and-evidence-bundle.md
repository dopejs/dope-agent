# Support Diagnostics And Evidence Bundle

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 70, the support
diagnostics and redacted evidence bundle slice.

Primary source documents:
- `docs/specs/027-integration-health-and-permission-diagnostics.md`
- `docs/specs/028-hosted-operational-profile-and-recovery.md`
- `docs/specs/026-evaluation-product-expansion.md`
- `docs/runtime/release-readiness.md`

## Background

Public hosted operation creates support obligations. Users and support staff need a safe
way to gather relevant tenant-scoped evidence for failed channels, providers, routines,
quota denials, approvals, live validation, and delivery without exposing secrets or
unrelated tenant data.

## Goal

Provide a permission-gated redacted evidence bundle for support and incident triage.

## Fixed Decisions

- Evidence bundles are tenant-scoped and redacted by default.
- Bundle generation must be auditable.
- Bundles collect links and summaries, not unbounded raw logs.
- This roadmap does not create autonomous remediation.

## Dependencies On Completed Phases

- Roadmap 42: Integration Health And Permission Diagnostics
- Roadmap 43: Hosted Operational Profile And Recovery
- Roadmap 69: Operator Shell Productization

## In Scope

- evidence bundle request resource
- selectable scope: run, workflow, thread, connector, provider, routine, quota denial, or
  time window
- redaction policy and validation
- downloadable or inspectable structured report
- support role permission checks
- audit events and retention policy

## Out Of Scope

- full log archive export by default
- cross-tenant support browsing without explicit permission
- external ticketing integration unless clarified
- memory export

## Operator Or User Problems To Solve

- Users need to share diagnostic evidence without leaking secrets.
- Support needs enough context to triage failures quickly.

## User Stories

- As a user, I can generate a support bundle for a failed routine.
- As support, I can request a redacted bundle for an authorized tenant.
- As an auditor, I can see who generated or accessed a bundle.

## Functional Requirements

- The system MUST generate bundles with tenant, actor, scope, created time, retention, and
  redaction status.
- Bundles MUST include relevant resource summaries and links.
- Bundles MUST exclude raw secrets, OAuth tokens, credential-bearing payloads, and
  inaccessible tenant data.
- Bundle access MUST be permission-gated and audited.
- Redaction failure MUST fail closed.

## Compatibility And Operational Notes

Bundles should reuse existing diagnostic, evaluation, hosted evidence, audit, and event
records rather than duplicating raw data.

## Verification Expectations

- Redaction tests across credential, connector, integration, mail, calendar, delivery,
  quota, and evaluation records.
- Permission and tenant isolation tests.
- API/SDK/web tests for bundle generation and inspection.
- Fixture tests for pass and fail redaction outcomes.

## Definition Of Done

- Support can triage public hosted failures from safe evidence bundles without direct
  database access or secret exposure.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/055-support-diagnostics-and-evidence-bundle.md 完成 phase 70 的工作`
