# Ingress Routing Closure

## Purpose

This document records the final daemon closure work after the original seven roadmaps.

The remaining gap was not in runtime, persistence, supervision, or chat. It was in the inbound edge:

- connector ingress was not a first-class daemon API
- session routing existed as an internal model, but not as a closed ingress contract
- runs could bind to sessions through default local behavior, but not through explicit route-aware ingress semantics

`Roadmap 8` closes that gap.

## What Was Added

### Route-Aware Run Creation

`POST /v1/runs` now supports three binding modes:

1. explicit `sessionId`
2. explicit `route`
3. local fallback when neither is supplied

`sessionId` and `route` cannot be supplied together.

Example:

```json
{
  "entrypoint": "connector.message",
  "goal": "Handle inbound connector message",
  "route": {
    "kind": "group",
    "channel": "telegram",
    "accountId": "bot-main",
    "peerId": "chat-1",
    "threadId": "thread-1"
  }
}
```

### Connector Ingress API

The daemon now exposes:

- `POST /v1/connectors/{connectorId}/ingress/messages`

This route:

- validates the connector exists
- rejects ingress when connector state is not accepting traffic
- resolves or creates a session from connector identity + route input
- optionally creates a run bound to that session
- persists resulting session/run state
- emits connector/session/run events

Example request:

```json
{
  "route": {
    "kind": "direct",
    "accountId": "bot-main",
    "peerId": "dm-1"
  },
  "message": {
    "messageId": "msg_1",
    "text": "hello"
  },
  "run": {
    "entrypoint": "connector.message",
    "goal": "Handle inbound message"
  }
}
```

## Event Model

Ingress now emits:

- `session.created` when a new session is created
- `session.routed` when ingress routes into a session
- `run.created` when ingress creates a run
- `connector.ingress_accepted` when the inbound envelope is accepted

This keeps connector ingress visible through the same event history surface as the rest of the daemon.

## Durability And Recovery

Ingress-created state now inherits the same persistence and recovery guarantees as the rest of the daemon:

- session is persisted to SQLite
- run is persisted to SQLite
- run checkpoint is written when a run is created
- restart recovery restores the session and bound run

This means ingress is no longer a side path. It is part of the daemon truth.

## Verification

Roadmap 8 was only closed because all of these are true:

- route-aware run creation is schema-backed and tested
- connector ingress request/response contracts are schema-backed and tested
- ingress emits validated connector/session/run events
- persistence is checked in API tests
- restart recovery is checked in app tests
- the final two historical partial tasks in `Sessions And Routing Foundation` are closed
