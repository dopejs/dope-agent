# Agent Profile And Persona Configuration

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 57, the structured
agent profile and persona configuration slice.

Primary source documents:
- `docs/product/openclaw-architecture-gaps.md`
- `docs/product/feature-phasing.md`
- `docs/specs/039-daemon-owned-thread-and-session-lifecycle.md`

## Background

OpenClaw-style systems often use workspace prompt files as runtime inputs. Kura's
direction is to keep editable files as overlays while moving runtime truth into structured
state. Before memory work begins, persona and profile configuration must be structured,
auditable, and bindable to channels and workspaces.

## Goal

Create structured agent profiles for identity, persona, defaults, and operator
preferences without introducing memory or knowledge retrieval.

## Fixed Decisions

- Profiles are structured runtime configuration, not memory.
- Prompt or workspace files may be overlays, not primary truth.
- Profiles must be tenant-owned and auditable.
- Profile changes must not silently rewrite historical run evidence.

## Dependencies On Completed Phases

- Roadmap 54: Daemon-Owned Thread And Session Lifecycle
- Roadmap 45: Hosted Signup And Tenant Activation

## In Scope

- agent profile resource model
- display identity, tone/persona fields, default provider preferences, and safety defaults
- editable overlay references
- profile versioning and audit events
- SDK and web shell profile editor
- runtime projection of active profile ID

## Out Of Scope

- memory or learned preferences
- agent-generated profile mutation
- skill generation
- multi-agent autonomous collaboration

## Operator Or User Problems To Solve

- Users need to configure how their agent presents itself and what defaults it uses.
- Operators need profile changes to be inspectable when behavior changes.

## User Stories

- As a user, I can create and edit an agent profile.
- As an operator, I can see which profile was active for a run.
- As a user, I can roll back a profile change.

## Functional Requirements

- The system MUST expose tenant-scoped profile CRUD with version history.
- Active runs and threads MUST record active profile identity.
- Profile fields MUST be schema-backed and redacted where needed.
- Profile changes MUST emit audit and event records.
- Overlay files MUST be referenced explicitly rather than treated as hidden truth.

## Compatibility And Operational Notes

Existing config and prompt overlay behavior should migrate into explicit references where
possible without breaking local workflows.

## Verification Expectations

- API, store, schema, SDK, and web tests for profile lifecycle.
- Runtime tests proving active profile linkage on runs and threads.
- Audit tests for profile changes and rollback.

## Definition Of Done

- Persona/profile configuration is structured product truth and ready to become an input
  to later context engineering.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/042-agent-profile-and-persona-configuration.md 完成 phase 57 的工作`
