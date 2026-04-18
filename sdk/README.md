# SDK

This directory contains client-facing libraries.

Current package:

- `sdk/ts`: shared TypeScript chat client used by both Web and TUI

Design rule:

- clients must share the same daemon contract layer
- Web and TUI must not each implement ad-hoc HTTP/SSE logic

Reserved for generated or maintained client SDKs.

Planned targets:

- TypeScript
- Go
