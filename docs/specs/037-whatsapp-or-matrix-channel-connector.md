# WhatsApp Or Matrix Channel Connector

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 52, the fourth
real channel connector slice. The implementation must choose WhatsApp or Matrix during
speckit clarification based on provider risk and hosted viability.

Primary source documents:
- `docs/specs/033-channel-connector-conformance-contract.md`
- `docs/specs/031-hosted-credential-and-oauth-setup-wizard.md`
- `docs/product/feature-phasing.md`

## Background

OpenClaw-style parity depends on multi-channel reach. After Discord, Telegram, and Slack,
DopeAgent needs one more materially different connector: WhatsApp for personal mobile
messaging reach, or Matrix for open federated rooms with lower platform risk.

## Goal

Add one additional hosted-ready channel connector that broadens channel coverage while
preserving conformance, tenant isolation, and diagnostics.

## Fixed Decisions

- The chosen channel must satisfy the shared conformance contract.
- Provider risk must be made explicit before implementation.
- Unsupported or unsafe provider behavior must block the connector rather than creating
  brittle hidden automation.
- This roadmap implements one channel, not both.

## Dependencies On Completed Phases

- Roadmap 48: Channel Connector Conformance Contract
- Roadmaps 49-51: First three production channels

## In Scope

- provider selection clarification between WhatsApp and Matrix
- hosted setup and credential model for the chosen provider
- DM/room routing and durable dedupe
- outbound reply and delivery adapter
- provider-specific diagnostics and redaction
- conformance tests and safe smoke path

## Out Of Scope

- implementing both WhatsApp and Matrix
- unsupported unofficial automation that violates provider policy
- voice, calls, or media-rich workflows unless the provider path is straightforward
- memory-based channel personalization

## Operator Or User Problems To Solve

- Users need more than work-chat coverage to treat the agent as reachable.
- Operators need provider-risk decisions recorded before committing to a connector.

## User Stories

- As a product owner, I can choose the fourth channel based on hosted safety and user value.
- As a user, I can reach the agent from one more common messaging surface.

## Functional Requirements

- The spec implementation MUST record the provider choice and rejected alternative.
- The chosen connector MUST satisfy shared conformance.
- The chosen connector MUST reference `docs/channels/channel-connector-conformance.md`
  for shared routing, dedupe, diagnostics, redaction, and delivery-boundary behavior
  instead of redefining those contracts.
- The setup path MUST be tenant-scoped and redacted.
- Diagnostics MUST classify auth, permission, rate-limit, provider, and network failures.

## Compatibility And Operational Notes

If WhatsApp requires provider-hosted APIs, the implementation must avoid brittle personal
web automation. If Matrix is chosen, federation and room identity semantics must be
explicit.

## Verification Expectations

- Provider-risk research artifact.
- Fake transport conformance tests.
- Safe real-account smoke where credentials are available, or structured skip.

## Definition Of Done

- DopeAgent has a fourth real channel option and a documented provider-risk decision.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/037-whatsapp-or-matrix-channel-connector.md 完成 phase 52 的工作`
