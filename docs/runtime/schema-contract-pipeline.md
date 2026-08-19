# Schema Contract Pipeline

## Purpose

This document defines how Kura keeps daemon contracts enforceable.

P0 uses committed JSON Schema files as the durable contract source for:

- HTTP request payloads
- HTTP JSON response payloads
- persisted runtime events

The enforcement path is test-driven:

- schemas live under `schemas/api` and `schemas/events`
- contract tests live under `crates/foundation/contracts`
- `make daemon-contract-test` runs the contract suite directly
- `cargo test --workspace` in `crates/` also executes the contract suite

This is the minimum acceptable P0 contract gate. Contract drift must fail in test, not be discovered by operator clients.

## What The Pipeline Checks

The contract suite currently verifies:

- canonical request fixtures against request schemas
- live HTTP responses from daemon handlers against response schemas
- persisted runtime events against event schemas
- `$ref` resolution across files and local `$defs`
- additive drift and accidental response-shape changes

The schema validator intentionally supports the subset of JSON Schema currently used by this repository:

- `type`
- `required`
- `properties`
- `items`
- `enum`
- `const`
- `minimum`
- `minLength`
- `minItems`
- `format: date-time`
- `additionalProperties`
- `$ref`
- `allOf`

## Update Process

Any contract change must update all of the following in one change:

1. daemon implementation
2. affected schema files
3. contract test fixture or live-route test coverage
4. release or migration notes if the change affects compatibility or persisted state

The required operator workflow is:

1. change the handler, resource type, or event payload
2. update the schema file in `schemas/api` or `schemas/events`
3. update or add a contract test in `crates/foundation/contracts`
4. run `make daemon-contract-test`
5. run `cargo test --workspace` under `crates/`

## Contract Rules

- Request and response shapes are additive-first. Breaking field removal or rename requires an explicit migration decision.
- Event payloads are durable contracts because they are persisted and replayed after restart.
- Wrapper responses are not exempt. If a route returns an envelope, the envelope gets its own schema.
- A resource schema is not enough when the route returns a list or multi-object wrapper. Those responses need their own committed schema.

## Current Boundary

The pipeline is intentionally scoped to JSON contracts.

It does not currently cover:

- SSE event-stream framing
- non-JSON error response policy beyond existing handler tests
- external SDK generation

Those are release-review items, not hidden assumptions.
