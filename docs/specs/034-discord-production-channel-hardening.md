# Discord Production Channel Hardening

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 49, the production
hardening slice for the existing Discord connector.

Primary source documents:
- `docs/specs/033-channel-connector-conformance-contract.md`
- `docs/channels/discord-channel-loop.md`
- `daemon/internal/connectors/discord/`

## Background

Discord is the first real IM channel. It proves the loop, but public hosted use requires
tenant-owned setup, operational diagnostics, repair flows, rate-limit behavior, and
conformance evidence rather than only a working message path.

## Goal

Make Discord a hosted-production-ready channel connector that satisfies the shared channel
conformance contract.

## Fixed Decisions

- Discord remains the first proof channel but not the only strategic channel.
- Hosted setup must use tenant-owned credentials and redacted diagnostics.
- Connector failures must be operator-visible and supportable.
- This roadmap does not add memory or multi-channel abstractions beyond conformance needs.

## Dependencies On Completed Phases

- Roadmap 48: Channel Connector Conformance Contract
- Roadmap 46: Hosted Credential And OAuth Setup Wizard

## In Scope

- Discord setup and repair UX integration
- bot token validation and permission diagnostics
- guild/channel allowlist management
- mention and DM behavior review against conformance contract
- rate-limit and reconnect evidence
- reply progression capability declaration
- live hosted smoke path with safe credentials or structured skip

## Out Of Scope

- Discord voice
- broad rich media support
- custom Discord app marketplace listing
- memory-based thread recall

## Operator Or User Problems To Solve

- Users need to connect Discord and understand whether the bot can see and reply.
- Operators need failed Discord setups to produce stable reason codes.

## User Stories

- As a user, I can connect Discord and choose DM, mention, guild, and channel behavior.
- As an operator, I can diagnose bot token, permission, rate-limit, and gateway failures.

## Functional Requirements

- Discord connector MUST satisfy the channel conformance contract or declare unsupported
  optional capabilities.
- Setup MUST validate credentials without leaking token material.
- Guild/channel filtering MUST be inspectable and tenant-scoped.
- Gateway reconnects and rate limits MUST produce durable diagnostic evidence.

## Compatibility And Operational Notes

Existing Discord config must migrate or project into the hosted connector model without
breaking local test usage.

## Verification Expectations

- Discord conformance tests.
- Fake transport tests for auth, rate limit, gateway disconnect, duplicate inbound, and
  reply failure.
- Manual hosted/test Discord smoke where safe credentials exist, or structured skip.

## Definition Of Done

- Discord can be offered as a public hosted connector with clear setup, health, repair,
  and support evidence.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/034-discord-production-channel-hardening.md 完成 phase 49 的工作`
