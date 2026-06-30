# Feature Specification: Operator Shell Productization

**Feature Branch**: `main`
**Created**: 2026-06-30
**Status**: Draft
**Phase / Roadmap**: Phase 70 — Roadmap 70
**Upstream authority**: [docs/specs/055-operator-shell-productization.md](../../docs/specs/055-operator-shell-productization.md)

## Overview
Turn the web shell into a cohesive product control console: a single information architecture
organizing all public non-knowledge surfaces (setup, channels, sessions, profiles, routines,
providers/capabilities, quota, diagnostics, evaluation, support), preserving tenant selection,
showing permission/approval/quota/side-effect expectations on critical actions, and rendering
stable empty/error/denied/unsupported states. The shell remains a pure client of daemon APIs
(consumed through the @dope/client SDK) and owns no runtime truth.

## User Scenarios & Testing *(mandatory)*
### US1 - Operate the product from one shell (P1)
1. A user navigates setup, channels, sessions, profiles, routines, providers, and quota from one
   coherent navigation; the Roadmap 65-69 surfaces (routines, webhooks, catalog, execution
   profiles) are reachable.
2. Critical actions show permission/approval/quota/side-effect expectations before running.
### US2 - Support navigates failure -> evidence (P2)
1. Every failure view exposes a stable reason and next step where available; denied/unsupported/
   empty states render explicitly.

## Requirements *(mandatory)*
- **FR-001**: Organize all public non-knowledge surfaces into coherent navigation with explicit
  empty/error/denied/unsupported states.
- **FR-002**: Preserve tenant selection across views (tenant-required surfaces deny without it).
- **FR-003**: Critical actions MUST show permission, approval, quota, and side-effect expectations.
- **FR-004**: Every failure view MUST expose a stable reason / next step where available.
- **FR-005**: The shell MUST NOT bypass daemon APIs (all access via the SDK client).

### Key Entities
- Shell Section / Surface (IA), Surface critical expectations, View state resolver.

## Compatibility & Operational Impact *(mandatory)*
- **Compatibility**: Consolidates UI surfaces + adds SDK client methods for the new product
  surfaces; no backend refactor; existing API contracts authoritative.
- **Migration / Rollback**: None; additive.
- **Verification**: SDK build + tests (new product-surface methods); web tests for the IA module
  (navigation coverage, tenant preservation, critical-action expectations, state resolution).
- **Observability**: shell consumes daemon events/projections; no new runtime truth.

## Success Criteria *(mandatory)*
- **SC-001**: All public surfaces (incl. Roadmap 65-69) are in the shell IA.
- **SC-002**: Tenant-required surfaces deny without a selected tenant.
- **SC-003**: Critical-action surfaces declare permission/approval/quota/side-effect.
- **SC-004**: Load results map to stable empty/error/denied/unsupported/ready states.
- **SC-005**: SDK exposes typed methods for triage/routines/webhooks/catalog/execution profiles.

## Assumptions
- TUI parity is not required this roadmap. The IA module (web/src/features/operator-shell) is the
  navigation source of truth; full per-surface React pages are wired incrementally against the
  existing App shell. SDK methods route through daemon APIs (no bypass).
