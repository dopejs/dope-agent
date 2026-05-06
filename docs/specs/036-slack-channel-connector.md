# Slack Channel Connector

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 51, the Slack
channel connector slice.

Primary source documents:
- `docs/specs/033-channel-connector-conformance-contract.md`
- `docs/specs/031-hosted-credential-and-oauth-setup-wizard.md`
- `docs/specs/027-integration-health-and-permission-diagnostics.md`

## Background

Slack covers workgroup and team-channel usage that personal messaging connectors do not.
It exercises workspace authorization, channel membership, app scopes, direct messages,
mentions, and enterprise-style permission failures.

## Goal

Add a hosted Slack connector with workspace-aware setup, DM/channel routing, mention
gating, diagnostics, and conformance evidence.

## Fixed Decisions

- Slack must reuse the shared connector conformance contract.
- Slack workspace identity must be explicit and tenant-owned.
- App scope and workspace installation failures must map to stable diagnostics.
- This roadmap does not implement a full enterprise admin suite.

## Dependencies On Completed Phases

- Roadmap 48: Channel Connector Conformance Contract
- Roadmap 46: Hosted Credential And OAuth Setup Wizard

## In Scope

- Slack app or bot setup flow
- workspace, channel, and user identity mapping
- DM and channel mention routing
- inbound dedupe by Slack event/message identity
- reply and optional thread reply behavior
- scope, installation, rate-limit, and event-delivery diagnostics
- delivery target reuse for Slack

## Out Of Scope

- Slack marketplace publication
- enterprise grid administration
- voice huddles
- memory-based team context

## Operator Or User Problems To Solve

- Work users need to reach the agent from Slack without leaking organization data across
  tenants.
- Operators need app installation and scope failures to be actionable.

## User Stories

- As a user, I can connect a Slack workspace and allow selected channels.
- As a user, I can message the agent in DM or mention it in a channel.
- As an operator, I can distinguish missing app scopes from event-delivery failures.

## Functional Requirements

- Slack MUST satisfy shared channel conformance.
- Slack workspace and channel identity MUST be tenant-scoped.
- The connector MUST support durable dedupe and retry-safe reply truth.
- Diagnostics MUST classify missing scopes, installation missing, rate limits, and event
  delivery failures.

## Compatibility And Operational Notes

Slack should reuse integration diagnostics where app scope evidence overlaps provider
authorization evidence.

## Verification Expectations

- Fake Slack transport tests for DM, channel mention, duplicate event, reply, and rate
  limit.
- Setup and OAuth tests with redacted credential evidence.
- Manual real Slack smoke where safe workspace credentials exist, or structured skip.

## Definition Of Done

- Slack is a hosted-ready work-channel connector with setup, routing, diagnostics, and
  conformance evidence.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/036-slack-channel-connector.md 完成 phase 51 的工作`
