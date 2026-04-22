# Research: Personal Integrations Platform

## Decisions

### Decision: Introduce a dedicated daemon-owned integrations plane instead of reusing the existing connector supervisor

- Rationale: The current `connectors` plane is channel-ingress supervision for IM
  transports. It tracks liveness, backoff, restart, and ingress acceptance, but it does
  not model account binding, auth readiness, canonical-default selection, or redacted
  secret-scope provenance. Roadmap 27 needs those semantics as first-class operator truth.
- Alternatives considered:
  - Extend `daemon/internal/connectors` with account-binding and auth fields.
    - Rejected because it would blur channel-ingress and personal-system identity into one
      resource model and make connector-only routes carry personal-integration semantics
      they do not need.
  - Store integration truth only in config inspection responses.
    - Rejected because the roadmap requires daemon-managed resources with durable status,
      readiness, and provenance, not derived config snapshots.

### Decision: Model readiness as explicit integration resource state with supporting auth and health context

- Rationale: The spec fixes the operator-visible readiness vocabulary at `not_configured`,
  `auth_pending`, `healthy`, `degraded`, and `unavailable`. Persisting that readiness
  explicitly on the integration resource keeps later domains from recomputing state
  differently while still allowing auth and health details to explain why a resource is in
  that state.
- Alternatives considered:
  - Derive readiness lazily from backend type, secrets, and heartbeat data on every read.
    - Rejected because operators need durable truth across restart and consistent behavior
      across backend styles.
  - Collapse readiness into only auth state plus health state fields.
    - Rejected because downstream domains and operators need one canonical readiness term,
      not a matrix they must interpret themselves.

### Decision: Allow multiple integration records per domain/account/environment while enforcing one canonical default

- Rationale: Clarification established that multiple backend paths may intentionally point
  at the same account in the same environment, but downstream work still needs one
  unambiguous default binding. The platform therefore needs group-aware uniqueness rules
  without forcing single-record ownership.
- Alternatives considered:
  - Enforce exactly one integration per domain/account/environment.
    - Rejected because it blocks safe migration, dual-backend comparison, and operator
      choice between backend styles.
  - Allow unlimited duplicate records with no canonical-default field.
    - Rejected because later domains would have to invent their own tie-break logic and
      operator inspection would stay ambiguous.

### Decision: Treat `unavailable` as hard-blocking and `degraded` as inspectable with operation-specific gating

- Rationale: Clarification established that only `unavailable` should blanket-block
  integration-backed work. `Degraded` needs to remain visible and truthful without
  preemptively deciding every downstream operation policy in roadmap 27.
- Alternatives considered:
  - Block all work whenever an integration is `degraded`.
    - Rejected because some later read-only operations may still be safe or useful.
  - Treat `degraded` as fully healthy for all work.
    - Rejected because it would hide meaningful operator risk and weaken later policy
      decisions.

### Decision: Reuse the existing runtime, workflow, approval, and secret-provenance planes by attaching additive integration-binding summaries

- Rationale: The upstream spec explicitly requires one shared run, workflow, approval,
  event, and artifact model. The platform should therefore add `integrationBindings`
  summaries to tool-call, workflow-step, and approval surfaces rather than creating a
  second execution ledger for integration-backed work.
- Alternatives considered:
  - Add a standalone integration-invocation ledger beside tool calls.
    - Rejected because it would split execution truth across two systems and repeat the
      mistake earlier roadmaps were avoiding.
  - Defer integration provenance until the first calendar or mail implementation.
    - Rejected because roadmap 27 is supposed to close the shared substrate those domains
      build on.

### Decision: Provide one repo-owned fake integration backend and run-scoped probe route for verification

- Rationale: The spec requires a local or fake verification path in `DOPE_ENV=test`. A
  deterministic fake backend with one read-only probe and one approval-gated mutation
  probe lets the repository validate readiness transitions, canonical-default behavior,
  approval reuse, runtime linkage, and redacted provenance without touching live personal
  systems.
- Alternatives considered:
  - Require a live calendar or mail sandbox account to close roadmap 27.
    - Rejected because roadmap 27 is the shared substrate and must remain verifiable
      without live external dependencies.
  - Skip probe execution and validate only resource inspection.
    - Rejected because the spec also requires integration-backed execution to preserve one
      shared runtime and approval model.

## Implementation Notes

- New resource ownership lives in `daemon/internal/integrations`; backend adapters may be
  fake, managed-provider-backed, MCP-backed, or native later, but the operator-facing
  resource shape converges here.
- Runtime, workflow, and approval projections should store integration-binding snapshots
  rather than dereferencing live integration state at read time; operators need to see
  the readiness and provenance truth that existed when the work executed.
- Existing redacted secret-scope summaries should be reused where available instead of
  inventing a second secret-provenance format for integrations.
