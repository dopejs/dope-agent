# Managed Coding Providers

## Purpose

This document records the completion boundary for `Roadmap 10: Managed Coding Providers`.

The goal of this roadmap was not to add more `baseURL + apiKey` providers. The goal was to make Kura understand provider profiles backed by managed login state and local coding-plan tooling.

## What Is Implemented

### 1. Managed Provider Auth Surface

Managed providers now expose a first-class auth surface through daemon APIs:

- `GET /v1/providers/{providerId}/auth`
- `POST /v1/providers/{providerId}/auth/start`
- `POST /v1/providers/{providerId}/auth/complete`
- `POST /v1/providers/{providerId}/auth/refresh`
- `POST /v1/providers/{providerId}/auth/revoke`

These routes operate on durable auth state, not on ephemeral UI state.

The auth model is based on `local_cli_bridge`, which is the correct abstraction for providers such as Claude Code and Codex CLI that do not fit a raw API-key configuration model.

### 2. Claude Managed Provider

Claude is now represented as a managed provider profile:

- provider id: `claude_managed`
- family: `claude_code_cli`
- auth mode: `local_cli_bridge`

The daemon can:

- inspect Claude login state
- expose a Claude model catalog
- dispatch single-turn requests through the daemon LLM plane
- classify auth failures from the local CLI bridge

### 3. Codex Managed Provider

Codex is now represented as a managed provider profile:

- provider id: `codex_managed`
- family: `codex_cli`
- auth mode: `local_cli_bridge`

The daemon can:

- inspect Codex / ChatGPT CLI login state
- load model availability from local Codex metadata
- expose model compatibility metadata
- dispatch through the same high-level chat and dispatch contracts

### 4. Model Catalog And Default Model Control

Managed providers now expose:

- `GET /v1/providers/{providerId}/models`
- `POST /v1/providers/{providerId}/default-model`

The daemon persists:

- provider auth state
- provider model catalogs
- provider default-model preferences

These are restored after daemon restart.

## Persistence And Recovery

SQLite schema version `4` adds:

- `provider_auth_states`
- `provider_models`
- `provider_preferences`

On startup, the daemon:

1. restores persisted provider state
2. restores persisted model catalogs
3. restores persisted preferences
4. refreshes managed providers from local CLI bridges

This means operator state is durable, but still converges back to the local CLI truth when the daemon restarts.

## Dispatch Semantics

Managed providers are not a separate chat stack.

They plug into the existing daemon dispatch plane:

- `/v1/llm/dispatches`
- `/v1/llm/dispatches/stream`
- `/v1/chat/query`
- `/v1/chat/query/stream`

Provider resolution now considers:

1. request provider/model override
2. daemon default provider/model
3. provider preference default model
4. provider profile default model

For managed providers, model selection is validated against the provider model catalog.

## Operator Implications

The daemon now supports two distinct provider classes:

- API-style providers such as `openai_compatible`
- managed coding providers such as `claude_managed` and `codex_managed`

This is the minimum substrate needed for a settings surface similar in spirit to OpenClaw:

- inspect provider inventory
- inspect login state
- trigger login workflow state transitions
- inspect available models
- choose profile default model

## Validation

This roadmap is backed by:

- provider manager unit tests
- managed CLI bridge unit tests
- store persistence tests
- API route tests
- app recovery tests
- schema contract tests

Verification commands:

- `cargo test --workspace` in `crates/`
- `make daemon-contract-test`
