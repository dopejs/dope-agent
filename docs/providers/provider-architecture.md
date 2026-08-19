# Provider Architecture

## Purpose

This document redefines the provider direction after `Roadmap 8`.

The previous provider plan was too narrow. It assumed provider work was mainly:

- inventory
- health check
- resolution policy

That is enough for API-key based providers, but not for the actual product goal.

The real provider scope must also cover:

- login-based managed providers
- provider profiles
- model catalogs
- compatibility layers across different upstream APIs

Examples:

- `OpenAI-compatible` providers using `baseURL + apiKey`
- `Claude` login or plan-backed access
- `Codex` / ChatGPT-style login or plan-backed access

Those should not be forced into a fake `baseURL + apiKey` model.

## Core Design Principle

Provider handling must be split into three planes:

### 1. Provider Family

This is the protocol or upstream API shape.

Examples:

- `openai_compatible`
- `anthropic_messages`
- future custom families

This layer answers:

- how requests are encoded
- how streams are decoded
- how errors are mapped
- how model capabilities are described

### 2. Auth Mode

This is how Kura obtains usable access for that provider profile.

Examples:

- `api_key`
- `oauth_device`
- `session_token`
- `local_cli_bridge`

This layer answers:

- how credentials are acquired
- how credentials are refreshed
- whether credentials are user-managed or daemon-managed
- whether secrets live in config, env, keychain, or daemon store

### 3. Provider Profile

This is the operator-facing configured object that daemon actually uses.

A profile combines:

- provider family
- auth mode
- endpoint or account identity
- default model
- model allowlist or overrides
- timeout and retry policy
- capability flags

Profiles are what operator clients should see and select.

The operator should not have to think in terms of raw transport implementation details.

## Product Requirement Shift

The provider work is not just:

- "support more providers"

It is:

- support multiple provider families
- support multiple auth modes
- support both API-style and managed-login-style providers
- expose model compatibility in a way operator clients can understand

That means `Claude` or `Codex` style login support is a first-class product requirement, not an afterthought.

## Revised Roadmap Split

This work should be split into two closed roadmaps.

## Roadmap 9: Provider Identity And Profiles

Status: `planned`

This roadmap should close the provider management substrate.

### Scope

- provider profile resource model
- provider family metadata
- auth mode metadata
- provider inventory APIs
- provider profile CRUD or managed creation flows
- provider inspection APIs
- provider preflight/check APIs
- provider resolution rules
- provider check events and durable results

### Examples In Scope

- create an `OpenAI-compatible` profile with `baseURL + apiKey`
- inspect whether a profile is configured
- run a provider check and get auth/transport/upstream failure class
- expose effective default model and supported capabilities

### Explicitly Not In Scope

- actual Claude login flow
- actual Codex login flow
- new upstream protocol families beyond what is needed for the profile system

## Roadmap 10: Managed Coding Providers

Status: `planned`

This roadmap should close the first login-based provider integrations.

### Scope

- `Claude` managed provider flow
- `Codex` / ChatGPT managed provider flow if technically viable
- provider-specific auth adapters
- provider-specific model catalogs
- compatibility mapping into daemon dispatch plane
- operator flows for login, logout, refresh, inspect, and select model

### Examples In Scope

- operator can log into a managed provider without entering `baseURL + apiKey`
- daemon stores enough provider identity state to reuse the session
- daemon exposes available models and selection defaults
- daemon can dispatch through the managed provider using the same high-level chat contract

### Explicitly Not In Scope

- every provider family in the market
- automatic multi-provider fallback
- billing optimization or smart routing

## Why The Split Matters

If we try to do all of this in one roadmap, we will either:

- underbuild the profile/auth substrate
- or fake a Claude/Codex integration on top of the wrong abstraction

That would create exactly the kind of narrow implementation you do not want.

The correct order is:

1. build provider identity and profile substrate
2. then build managed coding providers on top of it

## Operator UX Direction

The eventual provider settings surface should let users do things like:

- add API provider
- login Claude
- login Codex
- inspect provider health
- choose default provider
- choose default model
- see which models are chat-only, coding-first, streaming-capable, or tool-capable

That UX should sit above provider profiles, not above raw transport implementations.

## Completion Standard

This document is only planning, but it changes the standard:

- provider work is no longer treated as only `baseURL + apiKey`
- managed login providers are first-class in the product plan
- provider families, auth modes, and provider profiles are separate concepts
