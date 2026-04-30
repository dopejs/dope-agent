# Research: Integration Health And Permission Diagnostics

## Decision: Keep Diagnostics Additive To The Integration Boundary

**Decision**: Add diagnostic state and provider classification under the existing
`daemon/internal/integrations` ownership boundary, with domain packages contributing
limited diagnostic summaries and `opsreadiness` owning real-account smoke publication.

**Rationale**: Integration resources, credential provenance, account binding, and
readiness already live in the integration platform. Keeping diagnostics there avoids a
parallel source of truth while allowing calendar, mail, reminders, delivery, and
connectors to project limited or unsupported diagnostic state through explicit contracts.

**Alternatives considered**:

- A separate `daemon/internal/diagnostics` service. Rejected because it would duplicate
  integration ownership and make tenant-scoped credential provenance harder to audit.
- Embedding diagnostic logic only in API handlers. Rejected because provider
  classification, redaction, freshness, and smoke behavior need unit-testable domain
  logic.

## Decision: Feishu/Lark Is The Full Proof Domain

**Decision**: Implement full diagnostic coverage for Feishu/Lark in Roadmap 42. Other
supported domains expose either limited structured diagnostic state or a deliberate
unsupported-diagnostic classification.

**Rationale**: Feishu/Lark exercises app or bot authorization, user OAuth, tenant
approval, scopes, token freshness, calendar/task/document-like API surfaces, provider
rate limits, and CLI-backed operator workflows. This closes the roadmap without
incorrectly claiming full provider coverage across every domain.

**Alternatives considered**:

- Full coverage for all domains in one phase. Rejected because it expands Roadmap 42
  beyond the agreed proof domain and increases provider-specific risk.
- Feishu/Lark only with no projection elsewhere. Rejected because release reviewers need
  to distinguish unsupported domains from missing evidence.

## Decision: Use Stable Reason Codes With Provider Evidence Adapters

**Decision**: Map provider-specific evidence into a stable reason-code catalog covering
authorization, tenant approval, scopes, token states, tenant mismatch, provider
availability, rate limits, network failures, transient failures, retry safety,
unsupported diagnostics, and unknown provider errors.

**Rationale**: User remediation, SDK handling, smoke reports, audit events, and
release-readiness evidence must agree on meaning even when providers return different
error payloads. Stable codes also make fake-backend coverage representative when
real-account smoke is skipped.

**Alternatives considered**:

- Preserve raw provider errors as the primary contract. Rejected because it leaks
  provider-specific behavior into clients and increases secret exposure risk.
- Collapse all permission problems into one code. Rejected because operators need to
  distinguish user action, tenant administrator action, app scope configuration, and
  provider issues.

## Decision: Cached State Is Allowed But Stales After 15 Minutes

**Decision**: Operator inspection may show cached diagnostic state, but it is marked
stale after 15 minutes. User-facing integration failures must produce current diagnostic
truth before presenting remediation.

**Rationale**: External diagnostics can be slow or rate-limited, so cached state is
useful for operator inspection. User remediation must still reflect current auth and
provider state at failure time.

**Alternatives considered**:

- Live diagnostics for every inspection. Rejected because it increases provider load and
  makes operator views dependent on external availability.
- Manual refresh only. Rejected because stale auth or token state would be too easy to
  present as current truth.

## Decision: Redaction Fails Closed

**Decision**: If diagnostic evidence cannot be confidently redacted, suppress the
diagnostic detail, emit redaction-failure audit evidence, and show only a generic safe
classification.

**Rationale**: Diagnostics are a convenience and operability feature; they must not
create a path for raw OAuth tokens, app secrets, authorization headers, or
credential-bearing payloads to reach product surfaces, reports, logs, fixtures, or
events.

**Alternatives considered**:

- Show detail only to operators. Rejected because operators are still product-surface
  readers and diagnostic evidence may include high-risk credential material.
- Store detail internally but hide it from users. Rejected because persisted raw detail
  increases breach and deletion risk.

## Decision: Diagnostic And Smoke Evidence Retains For 90 Days

**Decision**: Diagnostic runs and smoke report evidence use a 90-day default retention
period unless an authorized longer retention policy applies.

**Rationale**: Ninety days is long enough for release review, incident follow-up, and
regression investigation while limiting long-lived operational metadata and provider
evidence exposure.

**Alternatives considered**:

- Thirty days. Rejected because release and incident windows may span multiple cycles.
- Manual deletion only. Rejected because it creates unbounded operational evidence growth.

## Decision: Real-Account Smoke Defaults To Safe Probes

**Decision**: Real-account smoke defaults to read-only or reversible probes. Non-
idempotent or externally visible probes require both tenant administrator approval and
authorized operator approval.

**Rationale**: Real external systems may send messages, create records, or mutate tenant
state. Dual approval keeps tenant authority and operator accountability explicit for any
probe that can create observable or irreversible effects.

**Alternatives considered**:

- Operator-only approval. Rejected because tenant administrators own provider-side
  authorization and external side effects.
- Always skip risky probes. Rejected because some release reviews need explicit approved
  coverage for side-effecting domains.

## Decision: Reuse Live-Validation Retry Safety Evidence

**Decision**: Side-effecting integration failures reuse Roadmap 40 live-validation
concepts for retryable, unsafe-to-retry, and operator-action-needed classifications when
commit evidence or idempotency determines retry safety.

**Rationale**: Diagnostics should not invent a second retry-safety model. Reusing the
live-validation ledger and ambiguous commit vocabulary keeps user remediation, smoke
reports, and release evidence consistent.

**Alternatives considered**:

- Provider-specific retry strings. Rejected because they cannot drive cross-domain
  product behavior.
- Always mark side-effect failures as unsafe. Rejected because idempotent and clearly
  uncommitted failures should remain safely retryable.

## Decision: Release Readiness Consumes Diagnostic Evidence

**Decision**: Roadmap 42 smoke reports and diagnostic summaries feed release-readiness
evidence with pass, fail, blocked, skipped, limited, and unsupported states.

**Rationale**: Roadmap 39 already requires real-account smoke evidence and explicit skip
reasons. Roadmap 42 strengthens that evidence with stable reason codes, remediation
owners, and domain support status.

**Alternatives considered**:

- Keep diagnostic evidence outside release readiness. Rejected because release reviewers
  need to see which external systems are healthy, blocked, skipped, or unsupported.
