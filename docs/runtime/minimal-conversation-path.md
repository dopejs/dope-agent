# Minimal Conversation Path

## Purpose

This document defines the single-turn conversation path after the P0 daemon core.

The work is intentionally split into two closed roadmaps:

- `Roadmap 6: Real Conversation Core`
- `Roadmap 7: Minimal Chat Clients`

This split is structural, not optional. Provider integration and two client surfaces are too large to close safely in one implementation round without lowering standards.

## Current State

- `Roadmap 6` is complete
- `Roadmap 7` is complete

That means Kura daemon can already:

- load a real provider from config
- accept a single-turn query
- return a real assistant reply
- stream a single-turn reply over SSE

The overall minimal conversation goal is now closed:

- daemon-side real provider integration is complete
- Web and TUI client delivery is complete
- cross-client verification is complete

## Phase Boundary

This path is deliberately narrow:

- one user query in
- one assistant reply out
- one real configured provider
- no daemon-side multi-turn state
- no context engineering
- no memory engineering
- no tool orchestration during chat

The daemon remains stateless with respect to conversation history. Any future history retention belongs either to the client or to later context/memory subsystems, not to `Roadmap 6`.

## Implemented Daemon Contract

Roadmap 6 added a query-first contract above raw LLM dispatch:

- `POST /v1/chat/query`
- `POST /v1/chat/query/stream`

Request shape:

```json
{
  "provider": "openai_compatible",
  "model": "gpt-4.1-mini",
  "query": "Explain the architecture gaps in OpenClaw."
}
```

Response shape:

```json
{
  "dispatchId": "dispatch_123",
  "provider": "openai_compatible",
  "model": "gpt-4.1-mini",
  "query": "Explain the architecture gaps in OpenClaw.",
  "status": "completed",
  "reply": "...",
  "finishReason": "stop",
  "usage": {
    "inputTokens": 12,
    "outputTokens": 34,
    "totalTokens": 46
  }
}
```

Streaming uses SSE with these event names:

- `chat.query.started`
- `chat.query.delta`
- `chat.query.completed`
- `chat.query.failed`
- `chat.query.cancelled`

## Provider Strategy

The first real provider is `OpenAI-compatible`.

Current implementation supports:

- base URL
- API key
- default provider selection
- default model selection
- default timeout
- retry compatibility with the existing dispatch plane
- non-stream response
- stream response
- upstream auth and failure mapping

For `OpenAI-compatible` endpoints, the daemon now accepts these `baseURL` forms:

- provider root, for example `https://code.b886.top`
- `/v1`, for example `https://code.b886.top/v1`
- full chat completions path, for example `https://code.b886.top/v1/chat/completions`

That normalization is intentionally limited to the `OpenAI-compatible` provider only. It is not treated as a cross-provider URL rule.

The implementation uses the existing daemon LLM dispatch plane rather than creating a second hidden provider stack.

`echo` still exists only as an explicit dev fallback. It is no longer the hidden default provider path.

## Config Model

Daemon config now includes an `llm` section in the active environment config file.

- development/test default: `~/.kura-test/config.json`
- production explicit env: `~/.kura/config.json`

Example:

```json
{
  "bindAddr": "127.0.0.1:19192",
  "logLevel": "info",
  "llm": {
    "defaultProvider": "openai_compatible",
    "defaultModel": "gpt-4.1-mini",
    "defaultTimeoutMs": 30000,
    "defaultMaxRetries": 0,
    "openaiCompatible": {
      "baseURL": "https://api.openai.com/v1",
      "apiKeyEnv": "OPENAI_API_KEY",
      "model": "gpt-4.1-mini"
    }
  }
}
```

Supported secret paths:

- `llm.openaiCompatible.apiKey` in config file
- `llm.openaiCompatible.apiKeyEnv` pointing to an environment variable
- direct env override via `KURA_LLM_OPENAI_COMPATIBLE_API_KEY`

Relevant overrides:

- `KURA_LLM_DEFAULT_PROVIDER`
- `KURA_LLM_DEFAULT_MODEL`
- `KURA_LLM_DEFAULT_TIMEOUT_MS`
- `KURA_LLM_DEFAULT_MAX_RETRIES`
- `KURA_LLM_OPENAI_COMPATIBLE_BASE_URL`
- `KURA_LLM_OPENAI_COMPATIBLE_API_KEY`
- `KURA_LLM_OPENAI_COMPATIBLE_API_KEY_ENV`
- `KURA_LLM_OPENAI_COMPATIBLE_MODEL`
- `KURA_LLM_OPENAI_COMPATIBLE_TIMEOUT_MS`

`GET /v1/config` now redacts provider secrets and exposes only redaction metadata.

## First-Run Operator Flow

1. Write the active environment config file with an `llm.openaiCompatible` section.
2. Export the provider secret if `apiKeyEnv` is used.
3. Start the daemon.
4. Pair or authenticate as usual.
5. Issue `POST /v1/chat/query` or `POST /v1/chat/query/stream`.

Minimal single-turn request:

```json
{
  "query": "Hello"
}
```

If `defaultProvider` and `defaultModel` are configured, the daemon can resolve the rest. Otherwise the client must pass `provider` and `model` explicitly.

## Failure Modes

The daemon now makes these failure classes explicit:

- invalid provider config at startup
  - daemon startup fails
- upstream auth failure
  - chat returns a stable failed response with `errorCode: "upstream_auth_failed"`
- upstream invalid request
  - chat returns a stable failed response with `errorCode: "upstream_invalid_request"`
- upstream rate limit or upstream unavailable
  - retry behavior follows the existing dispatch plane rules
- timeout
  - dispatch fails with `errorCode: "timeout"`
- missing provider/model at request time
  - request fails early with a `400`

## Rollback And Disable Path

If provider integration must be disabled:

1. remove or comment out the `llm.openaiCompatible` config section
2. clear `llm.defaultProvider`
3. optionally use explicit `provider: "echo"` for local-only debugging

This rolls the daemon back to a no-real-provider state without touching runtime, auth, supervision, or persistence subsystems.

## Roadmap 6 Closure

`Roadmap 6` is only considered complete because all of these are now true:

- provider config is file/env driven
- provider secrets are redacted from config inspection
- invalid provider config fails clearly at startup
- one real OpenAI-compatible provider is integrated
- non-stream and stream execution both work through the existing dispatch plane
- query-first chat routes exist and are schema-backed
- daemon remains stateless for conversation history
- config, provider, API, contract, and app-level restart wiring are covered by tests
- operator setup and rollback are documented

## Roadmap 7 Boundary

What `Roadmap 7` closed for the overall minimal conversation goal:

- Web chat surface
- TUI chat surface
- cross-client smoke verification
- operator docs for starting each client against daemon

That means the minimal single-turn conversation path is now complete even though richer conversation work remains out of scope.
