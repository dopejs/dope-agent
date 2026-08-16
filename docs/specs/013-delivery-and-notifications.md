# Delivery And Notifications

Status: proposed

Authority: This document is the authoritative upstream spec for the personal-agent delivery plane that returns outcomes, alerts, and summaries back to the user.

Primary source documents:
- `docs/product/personal-agent-non-knowledge-roadmap-split.md` (removed 2026-08, in git history)
- `docs/product/feature-phasing.md`
- `docs/harness/harness-architecture.md`

## Background

An agent that can schedule and act still does not feel ambient if it cannot reliably deliver results, alerts, and summaries through user-visible channels.

## Goal

Add a first-class delivery plane for notifications, summaries, and routed result messages that can be attached to schedules, workflows, and integration outcomes.

## Fixed Decisions

- delivery is not just connector chat reply behavior
- scheduled and autonomous work must be able to emit routed results without an active chat request
- delivery failures are distinct from execution failures
- integration readiness and delivery outcome remain separate truth domains; an
  integration-backed action may succeed while delivery fails, or fail before delivery is
  attempted

## In Scope

- delivery targets and preferences
- workflow-result delivery
- failure and retry semantics for notifications
- digest or summary delivery scaffolding

## Out Of Scope

- full mobile-app push stack
- rich social-product feed design
- generalized marketing or campaign messaging

## User Stories

- As a user, I receive a result or failure summary after a scheduled task finishes.
- As an operator, I can tell whether work succeeded but delivery failed.
- As an operator, I can inspect which delivery targets are active for a user or integration.

## Functional Requirements

- the daemon MUST support delivery targets that are separate from the immediate request channel
- schedule- and workflow-originated work MUST be able to emit result notifications without an active foreground session
- delivery success, retry, suppression, and failure MUST be projected explicitly
- delivery configuration MUST remain operator-visible and environment-scoped
- delivery state MUST NOT redefine or overwrite integration readiness; phases that depend
  on both planes must preserve execution truth and delivery truth separately

## Compatibility And Operational Notes

- existing chat reply flows remain valid for foreground requests
- delivery targets should be reusable by schedules, integration events, and later memory-driven summaries
- channel expansion may add targets without redefining the delivery resource model

## Verification Expectations

- targeted routing and retry tests
- contract coverage for delivery resources or events
- one manual `DOPE_ENV=test` notification or summary flow

## Definition Of Done

- the agent can deliver background-work results through a durable, inspectable plane
- delivery truth is separated cleanly from execution truth

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/013-delivery-and-notifications.md 完成 phase 28 的工作`
