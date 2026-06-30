# Tasks: Operator Shell Productization

**Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md) | **Roadmap**: 70

- [X] T001 [Setup] Inspect SDK DopeClient + web App/feature structure.
- [X] T002 [SDK] Add typed types + client methods for triage/routines/webhooks/catalog/execution profiles.
- [X] T003 [SDK] product-surfaces.test.ts: routing assertions for the new methods.
- [X] T004 [Web] operator-shell navigation IA: sections/surfaces for all public surfaces + critical expectations.
- [X] T005 [Web] resolveViewState: stable empty/error/denied/unsupported/ready states; tenant-required denial.
- [X] T006 [Web] operator-shell.test.tsx: navigation coverage, tenant preservation, critical expectations, state mapping.
- [X] T007 [Polish] fix pre-existing R57 web typecheck debt (thread-lifecycle fixture fields); typecheck:web clean.
- [X] T008 [Polish] verify: build:sdk, sdk tests, web tests, typecheck:web.

## Notes
TUI parity not required. Full per-surface React pages wire incrementally against the App shell;
the IA module + SDK methods are the durable foundation. Shell accesses daemon only via the SDK.
