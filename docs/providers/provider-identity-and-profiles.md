# Provider Identity And Profiles

## Purpose

`Roadmap 9` turns providers into a first-class daemon plane instead of leaving them implicit inside bootstrap config and dispatch behavior.

This roadmap closes four operator-facing concerns:

- provider profile identity
- provider inventory and introspection
- daemon-managed provider checks
- explicit provider and model resolution policy

It does **not** add managed login providers. That remains `Roadmap 10`.

## Current Provider Resource Model

Provider profiles are now exposed as daemon resources.

Current fields include:

- `providerId`
- `title`
- `family`
- `authMode`
- `source`
- `modelSelectionMode`
- `knownModels`
- `registered`
- `configured`
- `ready`
- `default`
- `baseURL`
- `requestURL`
- `defaultModel`
- `effectiveModel`
- `effectiveTimeoutMs`
- `effectiveMaxRetries`
- `secretConfigured`
- `secretRef`
- `capabilities`
- `issues`

Secrets are never returned. The provider plane only exposes whether a secret is configured and, when relevant, the secret reference name.

## Current Supported Provider Families

`Roadmap 9` closes the substrate for provider profiles, but the concrete provider set is still intentionally small:

- `echo`
  - family: `builtin_echo`
  - auth mode: `none`
  - model selection: `fixed`
- `openai_compatible`
  - family: `openai_compatible`
  - auth mode: `api_key`
  - model selection: `open`

This is enough to validate the profile system without prematurely baking Claude or Codex login behavior into the wrong abstraction.

## API Surface

### Provider Inventory

- `GET /v1/providers`
- `GET /v1/providers/{providerId}`

These routes expose effective provider metadata and readiness without leaking credentials.

### Provider Checks

- `POST /v1/providers/{providerId}/checks`
- `GET /v1/providers/{providerId}/checks`
- `GET /v1/providers/{providerId}/checks/{checkId}`

Provider checks are durable resources. A failed check still returns `201` with a failed check resource; the failure is part of the operator result, not an internal server error.

## Provider Check Semantics

A provider check validates:

- provider profile existence
- config completeness
- model resolution
- a real execution path through the underlying provider

Failure classes are normalized into:

- `config_error`
- `auth_error`
- `transport_error`
- `upstream_error`
- `timeout`

Check results are:

- persisted in SQLite
- queryable through provider check routes
- emitted as durable events

Current provider events:

- `provider.check_completed`
- `provider.check_failed`

## Resolution Policy

Provider resolution is now explicit and shared by both:

- `/v1/llm/dispatches`
- `/v1/chat/query`

Resolution order is:

### Provider

1. request `provider`
2. configured `llm.defaultProvider`
3. implicit ready configured provider
4. builtin fallback provider

Today, the implicit ready configured provider path preserves the existing `openai_compatible` behavior when it is configured but not explicitly set as the default provider.

### Model

1. request `model`
2. configured `llm.defaultModel` when the request did not force a different provider
3. profile default model
4. provider default model

### Validation

- fixed-model providers reject incompatible models early
- dispatch and chat routes share the same resolution policy
- provider readiness and compatibility issues are reflected in profile `issues`

## Operator Workflow

Current operator loop is:

1. configure provider in the active environment config file or `DOPE_*`
2. inspect effective provider state with `GET /v1/providers`
3. run `POST /v1/providers/{providerId}/checks`
4. inspect the durable result and failure class
5. use `/v1/chat/query` or `/v1/llm/dispatches`

This is the intended substrate for the next roadmap, where managed coding providers will add login-backed profiles instead of `baseURL + apiKey`.

## Tenant-Owned Provider Auth

Hosted credential isolation makes provider auth state tenant-owned. Provider auth
resources include `tenantId`, lifecycle status, disabled reason, and redacted secret
reference summaries. They do not expose OAuth codes, access tokens, refresh tokens,
CLI auth material, or derived credential values.

Provider auth lifecycle routes must run in the active tenant:

- connect or complete auth creates/updates only that tenant's provider auth state
- refresh and checks resolve credential material only from the active tenant
- revoke/disconnect disables dependent use for that tenant without deleting redacted
  operator-visible metadata
- another tenant with the same provider account label or external account id must
  connect independently

Mutation requires the provider/integration management permission. A tenant-scoped
operator with `credentials.inspect` can inspect redacted auth status for that tenant;
viewers cannot inspect provider credential state.

## Local Bridge Behavior

On startup, the hosted credential bridge imports legacy local credential files into
the default personal tenant. Safe values become tenant secrets. Ambiguous values, such
as the same secret ref with conflicting legacy values, become disabled
`pending_remediation` metadata and cannot be used until an operator rotates or
reconnects the credential.

Provider readiness and auth status must treat bridged values like any other tenant
secret: only status, account labels, and redacted secret summaries are visible.
Bridge progress and provider auth events must not log raw credential material.

## Out Of Scope

This roadmap does not include:

- Claude login flows
- Codex / ChatGPT login flows
- provider profile creation UI
- multi-provider fallback
- remote provider registry
