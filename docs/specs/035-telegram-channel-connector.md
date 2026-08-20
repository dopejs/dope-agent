# Telegram Channel Connector

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 50, the Telegram
channel connector slice.

Primary source documents:
- `docs/specs/033-channel-connector-conformance-contract.md`
- `docs/specs/031-hosted-credential-and-oauth-setup-wizard.md`
- `docs/specs/013-delivery-and-notifications.md`

## Background

Telegram is a high-value personal-agent entry point because it supports direct mobile
access, bot messaging, groups, and broad international usage. It should be added only
after the channel conformance contract exists.

## Goal

Add a tenant-owned Telegram connector that supports hosted setup, DM and group routing,
foreground replies, diagnostics, and background delivery target reuse.

## Fixed Decisions

- Telegram must use the shared connector conformance contract.
- Bot API behavior is sufficient for the first Telegram slice.
- Unsupported Telegram features must fail explicitly.
- This roadmap does not add memory or voice.

## Dependencies On Completed Phases

- Roadmap 48: Channel Connector Conformance Contract
- Roadmap 46: Hosted Credential And OAuth Setup Wizard

## In Scope

- Telegram connector resource and setup path
- bot token validation and redaction
- DM and group message routing
- mention or command gating where applicable
- durable inbound dedupe by Telegram update/message identity
- outbound reply and delivery adapter
- diagnostics for auth, permission, rate-limit, provider, and network failures

## Out Of Scope

- Telegram voice
- payments
- mini apps
- broad media transfer beyond explicit text and basic attachment handling if feasible

## Operator Or User Problems To Solve

- Users need a mobile-first channel to reach their hosted agent.
- Operators need Telegram failures to be diagnosed without raw bot logs.

## User Stories

- As a user, I can connect a Telegram bot and message the agent in DM.
- As a user, I can allow or block group behavior.
- As support, I can inspect Telegram connector health and redacted setup evidence.

## Functional Requirements

- Telegram MUST satisfy the shared channel conformance suite.
- Telegram MUST reference `docs/channels/channel-connector-conformance.md` for shared
  routing, dedupe, diagnostics, redaction, and delivery-boundary behavior instead of
  redefining those contracts.
- The connector MUST preserve tenant, chat, message, and thread identity.
- Duplicate updates MUST not create duplicate runs or replies.
- Background delivery through Telegram MUST be recorded separately from foreground replies.

## Compatibility And Operational Notes

Telegram should reuse the IM loop and connector supervisor rather than creating a separate
execution path.

## Verification Expectations

- Fake Telegram transport tests for routing, dedupe, reply, failure, and reconnect.
- Contract tests for connector resource and event schemas.
- Manual `KURA_ENV=test` smoke using safe Telegram credentials or structured skip.

## Definition Of Done

- Telegram is a hosted-ready second real channel with conformance, diagnostics, setup, and
  delivery evidence.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/035-telegram-channel-connector.md 完成 phase 50 的工作`
