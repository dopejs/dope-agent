# Channel Management And Repair UX

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 53, the product
management and repair surface for hosted channel connectors.

Primary source documents:
- `docs/specs/033-channel-connector-conformance-contract.md`
- `docs/specs/034-discord-production-channel-hardening.md`
- `docs/specs/035-telegram-channel-connector.md`
- `docs/specs/036-slack-channel-connector.md`
- `docs/specs/027-integration-health-and-permission-diagnostics.md`

## Background

Multiple connectors are not product-ready unless users can inspect, enable, disable,
repair, and understand them without logs. Channel repair must be product behavior, not
operator archaeology.

## Goal

Add channel management and repair UX for all production connectors.

## Fixed Decisions

- Channel state must be tenant-scoped and permission-gated.
- Repair flows reuse diagnostic reason codes and setup sessions.
- Disabling a channel must preserve history and prevent new inbound work.
- This roadmap does not add a new connector.

## Dependencies On Completed Phases

- Roadmaps 48-52: Channel conformance and production connectors
- Roadmap 46: Hosted Credential And OAuth Setup Wizard

## In Scope

- channel list/detail product views
- enable, disable, reconnect, repair, and rotate credential actions
- connector health and diagnostic summaries
- allowlist and routing configuration UX
- foreground reply and background delivery status views
- support evidence for channel incidents

## Out Of Scope

- channel marketplace
- mobile push apps
- memory-driven channel ranking

## Operator Or User Problems To Solve

- Users need to know which channels are active and why one stopped working.
- Support needs redacted evidence for connector incidents.

## User Stories

- As a user, I can disable a channel without deleting its history.
- As a user, I can repair a broken connector from a diagnostic next step.
- As support, I can inspect redacted connector evidence.

## Functional Requirements

- The UI MUST list all tenant connectors with status, health, setup state, and diagnostics.
- Users MUST be able to disable and re-enable connectors according to permission.
- Repair actions MUST link to setup sessions and diagnostic evidence.
- Connector configuration changes MUST be auditable.
- Disabled connectors MUST reject or ignore new inbound work safely.

## Compatibility And Operational Notes

Existing connector APIs remain authoritative. This roadmap adds product projections and
repair actions across the connector fleet.

## Verification Expectations

- Web, SDK, and API tests for list/detail/disable/repair flows.
- Connector conformance regression after disable and re-enable.
- Manual `DOPE_ENV=test` walkthrough with at least two connector kinds.

## Definition Of Done

- A user can manage and repair hosted channels without raw config edits or log access.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/038-channel-management-and-repair-ux.md 完成 phase 53 的工作`
