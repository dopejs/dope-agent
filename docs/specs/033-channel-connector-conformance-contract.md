# Channel Connector Conformance Contract

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 48, the shared
contract every hosted channel connector must satisfy.

Primary source documents:
- `docs/channels/first-im-channel-loop.md`
- `docs/channels/discord-channel-loop.md`
- `docs/channels/channel-reply-progression.md`
- `docs/specs/013-delivery-and-notifications.md`
- `docs/specs/022-hosted-secrets-integrations-and-connector-isolation.md`

## Background

Kura has a connector supervisor, IM loop, Discord implementation, and delivery plane.
Public parity requires more channels, but adding them one by one without a conformance
contract would create inconsistent routing, dedupe, reply, repair, and tenant behavior.

## Goal

Define a channel connector conformance suite covering tenant scope, account binding,
inbound routing, dedupe, reply progression, delivery reuse, diagnostics, and failure
visibility.

## Fixed Decisions

- Connectors must not invent routing semantics locally.
- Foreground replies and background delivery remain separate product truths.
- Every hosted connector must be tenant-owned, permission-gated, and redacted.
- Rich media and channel-specific enhancements may be optional, but unsupported behavior
  must be explicit.

## Dependencies On Completed Phases

- Roadmap 28: Delivery And Notifications
- Roadmap 37: Hosted Secrets, Integrations, And Connector Isolation
- Roadmap 42: Integration Health And Permission Diagnostics

## In Scope

- connector conformance test matrix
- required connector lifecycle, health, and diagnostic states
- DM, group, mention, room, and thread routing expectations
- inbound dedupe and retry requirements
- reply progression capability vocabulary
- foreground/background delivery separation
- secret and account-binding redaction rules

## Out Of Scope

- implementing a new channel connector
- mobile app nodes
- voice and media workers
- memory-based conversation continuity

## Operator Or User Problems To Solve

- Users need channel behavior to feel predictable across Discord, Telegram, Slack, and
  future connectors.
- Engineers need a proving suite before adding more connectors.

## User Stories

- As an engineer, I can run one connector conformance suite against any channel adapter.
- As a user, I see consistent handling of duplicates, blocked channels, group mentions,
  and failed replies.

## Functional Requirements

- The contract MUST define required connector states and event families.
- The contract MUST define inbound message identity and durable dedupe behavior.
- The contract MUST define reply progression levels: final-only, thinking, incremental
  edit, and unsupported.
- The contract MUST define required diagnostics for auth missing, permission missing,
  rate limited, provider unavailable, and network failed.
- The contract MUST require tenant-scoped secret and account binding.

## Compatibility And Operational Notes

Existing Discord behavior should be adapted to the contract rather than treated as a
special case. Connector-specific capabilities remain additive.

## Verification Expectations

- Shared fake connector conformance tests.
- Discord regression showing the existing connector satisfies the contract or records
  explicit unsupported capabilities.
- Schema and docs updates for connector capabilities and diagnostics.

## Definition Of Done

- Future channel specs can focus on provider mechanics because shared connector behavior is
  already defined and testable.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/033-channel-connector-conformance-contract.md 完成 phase 48 的工作`
