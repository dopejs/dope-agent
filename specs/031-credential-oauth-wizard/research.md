# Research: Hosted Credential And OAuth Setup Wizard

## Decision: Model setup as an orchestration layer, not a replacement store

**Rationale**: Roadmap 46 explicitly depends on existing tenant secrets, provider auth,
connectors, integration diagnostics, and readiness surfaces. A setup session should
explain and coordinate those resources, while the existing resources remain the source of
truth for secret values, provider readiness, and integration diagnostics.

**Alternatives considered**:
- Store complete credential/provider state in setup sessions. Rejected because it would
  duplicate secret/provider truth and increase rollback risk.
- Reuse only the existing secret and provider routes without setup state. Rejected because
  hosted users still need recoverable progress, diagnostics, and wizard-level audit.

## Decision: Add `daemon/internal/setupwizard`

**Rationale**: Setup state, permissions, target catalog, state transitions, redaction
rules, and proof-target orchestration cross existing packages. A small package keeps that
state machine out of API handlers while still delegating authoritative operations to
`secrets`, `providers`, and `integrations`.

**Alternatives considered**:
- Put all logic in `daemon/internal/api`. Rejected because the state machine and
  redaction checks would be hard to test without HTTP.
- Put setup logic in `secrets` or `providers`. Rejected because the wizard spans both
  submitted-secret and OAuth paths plus diagnostics.

## Decision: V1 proof targets are OpenAI-compatible credentials and Feishu/Lark OAuth

**Rationale**: OpenAI-compatible provider setup proves the submitted-secret path against
an existing provider family with real tenant secret requirements. Feishu/Lark is already
the full proof domain for integration diagnostics and exercises OAuth, tenant approval,
scopes, and provider-specific remediation.

**Alternatives considered**:
- Generic fake targets only. Rejected because Roadmap 46 requires product-visible setup,
  not only a synthetic model.
- All existing targets. Rejected because it would expand scope beyond one roadmap slice
  and delay closure.

## Decision: Mutating setup requires both secret and integration management permission

**Rationale**: Submitted-secret setup writes tenant secret material, while OAuth and
provider setup changes integration/provider authorization state. Requiring both
`secrets.manage` and `integrations.manage` avoids half-authorized setup paths. Redacted
inspection remains separately gated by `credentials.inspect`.

**Alternatives considered**:
- Require only `integrations.manage`. Rejected because submitted-secret setup could write
  or rotate secret state without explicit secret permission.
- Split permissions by setup style. Rejected because replacement and disable flows can
  cross secret and integration boundaries.

## Decision: No terminal failed setup-session state

**Rationale**: The upstream Roadmap 46 final states are ready, degraded, unavailable,
cancelled, and action-required. Recoverable failures should remain actionable through
state plus reason code, not a generic failed terminal state. This makes retry and
remediation behavior unambiguous.

**Alternatives considered**:
- Add `failed` as a terminal setup state. Rejected because it conflicts with upstream
  final-state semantics and creates overlap with diagnostic run failures.
- Use failed only for system errors. Rejected because system errors are better represented
  as unavailable/action-required with stable reason codes and operator remediation.

## Decision: Setup state gates dependent credential-bearing use

**Rationale**: A guided setup wizard is only meaningful if dependent use respects its
current state. Ready allows normal use, degraded allows only explicitly marked limited
safe capabilities, and action-required/unavailable/cancelled/disabled block dependent
credential-bearing use until repair.

**Alternatives considered**:
- Let downstream integrations decide independently. Rejected because users would see
  inconsistent wizard readiness and runtime behavior.
- Block all degraded use. Rejected because Roadmap 42 diagnostics already distinguish
  degraded state from unavailable and some capabilities can remain safe.

## Decision: Default verification uses fake secrets and safe OAuth fixtures

**Rationale**: The constitution requires test-environment default and secret discipline.
Roadmap 46 must prove redaction, state transitions, diagnostics, and tenant isolation
without requiring live provider credentials. Real-account OAuth evidence can be recorded
later as release-readiness evidence if explicitly approved.

**Alternatives considered**:
- Require live OAuth for completion. Rejected because it would make default local
  verification dependent on external approval and production-like secrets.
- Skip OAuth behavior until live credentials exist. Rejected because OAuth setup is a v1
  proof target.

## Decision: Add setup contracts before implementation tasks

**Rationale**: The feature changes API, schema, SDK, web, persistence, audit, diagnostic,
and rollback behavior. Planning contracts reduce implementation ambiguity and ensure
`/speckit.tasks` can generate dependency-ordered work with testable gates.

**Alternatives considered**:
- Rely on the feature spec alone. Rejected because the state machine and redaction rules
  cross multiple packages and client surfaces.
