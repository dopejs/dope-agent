# Implementation Plan: Operator Shell Productization

**Branch**: `main` | **Spec**: [spec.md](./spec.md) | **Upstream**: [docs/specs/055-operator-shell-productization.md](../../docs/specs/055-operator-shell-productization.md)
**Phase / Roadmap**: Phase 70 — Roadmap 70

## Summary
Consolidate the web shell IA and extend the SDK to cover the Roadmap 65-69 product surfaces.
Add a single navigation/state module (web/src/features/operator-shell) that organizes all public
surfaces, declares critical-action expectations, preserves tenant selection, and resolves stable
view states. Add typed @dope/client methods for triage, routines, webhooks, catalog, and
execution profiles so the shell consumes daemon APIs without bypass.

## Constitution Check
- Roadmap closure: a hosted user can operate the non-knowledge product from the shell.
- Production-grade: tenant preservation, critical-action expectations, explicit failure states,
  SDK-only daemon access.
- Contracts first: SDK types mirror daemon resources; web IA references SDK routes.
- Verification: SDK build + tests; web IA tests; web typecheck clean.

## Project Structure
```
specs/055-operator-shell-productization/  spec.md plan.md tasks.md checklists/
sdk/ts/src/index.ts                  # typed methods + types for Roadmap 65-69 surfaces
sdk/ts/src/product-surfaces.test.ts  # SDK routing test
web/src/features/operator-shell/navigation.ts          # IA + state resolver
web/src/features/operator-shell/operator-shell.test.tsx
web/src/features/thread-lifecycle.test.tsx             # fix pre-existing typecheck debt
```

## Complexity Tracking
No backend refactor. SDK + web-IA additive. Full per-surface React pages are incremental against
the existing App shell; the IA module + SDK are the durable foundation this roadmap lands.
