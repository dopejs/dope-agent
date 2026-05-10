# Schemas

This directory is the planned source of truth for shared contracts.

It will hold schemas for:

- daemon APIs
- event envelopes
- config
- capability RPC
- plugin manifests

Generated code for Go or TypeScript should be derived from these schemas.

The first concrete P0 schemas should cover:

- system info responses
- run and step resources
- event envelope and early event shapes

Current additive contract groups include:

- operator-shell projection responses
- schedule, delivery, calendar, mail, reminder, computer-use, integration, provider, MCP,
  sandbox, and workflow resources
- evaluation and replay resources, including replay candidates, replay attempts,
  comparisons, drift findings, fixtures, and `evaluation.*` events
- Matrix channel connector resources for hosted setup, route policy, smoke evidence, and
  setup validation events. Matrix schemas intentionally keep tenant id additive and keep
  token, raw payload, event body, and room content fields out of public fixtures.
