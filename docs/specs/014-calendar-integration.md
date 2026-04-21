# Calendar Integration

Status: proposed

Authority: This document is the authoritative upstream spec for the first personal calendar domain slice built on the shared integrations platform.

Primary source documents:
- `docs/specs/012-personal-integrations-platform.md`
- `docs/specs/010-scheduled-tasks-and-wakeups.md`
- `docs/specs/013-delivery-and-notifications.md`

## Background

Calendar is one of the highest-value personal-agent domains because it combines passive context, active scheduling, conflict handling, and outbound coordination.

## Goal

Add a production-grade calendar domain that supports inspectable availability, event creation and updates, and schedule-triggered workflows.

## Fixed Decisions

- calendar is a separate roadmap from mail and reminders
- calendar actions must preserve account identity, event identity, and audit truth
- busy/free lookup and event mutation are separate operation classes

## In Scope

- calendar account binding through the shared integration model
- event list/detail inspection
- busy/free lookup
- create, update, and cancel event flows
- operator-visible meeting or event artifacts when applicable

## Out Of Scope

- CRM-style relationship modeling
- generalized travel booking
- memory-driven meeting summarization

## User Stories

- As a user, I can ask the agent to create or move an event and get a truthful result back.
- As an operator, I can distinguish availability lookup from event mutation.
- As a user, I can rely on scheduled routines that inspect upcoming events and deliver summaries.

## Functional Requirements

- calendar integration MUST expose inspectable calendar account readiness
- busy/free, read, create, update, and cancel actions MUST be distinct operation types
- calendar actions MUST preserve event identity and downstream delivery truth
- scheduled workflows MUST be able to invoke calendar reads and writes through normal runtime workflows

## Verification Expectations

- domain tests for availability lookup and event mutation
- contract coverage for calendar resource projections if added
- one manual or repo-owned local fixture verification path

## Definition Of Done

- the agent can read and mutate calendar state through a stable, inspectable, and auditable domain layer

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/014-calendar-integration.md 完成 phase 29 的工作`
