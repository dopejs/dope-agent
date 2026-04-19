# Minimal Chat Clients

## Purpose

This document defines `Roadmap 7: Minimal Chat Clients`.

The daemon-side conversation core was closed in `Roadmap 6`. This roadmap closes the two operator surfaces that sit on top of that contract:

- Web UI
- TUI

Both clients must consume the same daemon chat contract:

- `POST /v1/chat/query`
- `POST /v1/chat/query/stream`

No client-specific provider bypass is allowed.

## Current State

`Roadmap 7` is complete.

The repository now contains:

- a shared TypeScript client package: `@dope/client`
- a minimal Web chat surface
- a minimal TUI chat surface
- cross-client smoke verification

## Shared Client Contract

The shared client package lives in:

- `sdk/ts`

It is the only supported path for Web and TUI to talk to the daemon chat contract.

Responsibilities:

- build authenticated requests
- call the daemon chat routes
- parse SSE chat stream events
- normalize client-visible transport and API failures

This prevents Web and TUI from drifting onto separate request shapes or separate streaming implementations.

## Web Client

The Web client lives in:

- `web`

Current capabilities:

- daemon URL input
- access token input
- provider override input
- model override input
- stream toggle
- single-turn query form
- loading state
- visible error state
- streamed and non-stream reply rendering

The Web client does not attempt to persist or reconstruct daemon-side conversation state.

## TUI Client

The TUI client lives in:

- `tui`

Current capabilities:

- one-shot query from `--query`
- fallback interactive prompt when `--query` is omitted
- same daemon URL / token / provider / model configuration surface as Web
- stream and non-stream modes
- visible error output and non-zero exit on failure

Environment variables:

- `DOPE_DAEMON_URL`
- `DOPE_ACCESS_TOKEN`
- `DOPE_CHAT_PROVIDER`
- `DOPE_CHAT_MODEL`

CLI flags:

- `--daemon-url`
- `--token`
- `--provider`
- `--model`
- `--query`
- `--stream`

## Operator Flow

### Web

1. Start the daemon.
2. Start the Web client.
3. Paste or enter the access token.
4. Enter a single-turn query.
5. Send the query with or without streaming enabled.

### TUI

1. Start the daemon.
2. Build the TUI client.
3. Run the TUI with token and daemon URL, or export them as env vars.
4. Pass `--query` for one-shot mode, or omit it to be prompted.

## Verification Standard

Roadmap 7 is only closed because all of these are true:

- Web and TUI both use `@dope/client`
- neither client bypasses the daemon chat routes
- Web renders success, stream, and error states
- TUI renders success, stream, and error states
- clients build successfully
- client tests pass
- a cross-client smoke test proves the same daemon contract can serve shared-client logic and the terminal client without backend switching

## Failure Modes

The clients now expose these failure classes clearly:

- missing access token
- daemon unavailable
- provider auth failure propagated from daemon
- streaming interruption
- invalid daemon URL or malformed operator input

The clients do not attempt silent retries. They surface the failure so the operator can correct config or rerun.

## Non-Goals

This roadmap still does not add:

- multi-turn chat history in daemon
- memory recall
- context engineering
- tool calling from chat
- rich conversation UI behaviors

Those remain future roadmap work.
