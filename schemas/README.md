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
