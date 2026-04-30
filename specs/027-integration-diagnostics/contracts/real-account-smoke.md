# Contract: Real-Account Smoke

## Goal

Produce repeatable, structured real-account smoke evidence for integration diagnostics
without leaking secrets or creating uncontrolled provider side effects.

## Smoke Matrix Report

Each smoke matrix report includes:

- `smokeReportId`
- `tenantId`
- `reportKind`
- `requestedBy`
- `status`
- `startedAt`
- `completedAt`
- `publishedAt`
- `domainSummary`
- `artifactRefs`
- `retentionExpiresAt`

Allowed report statuses:

- `draft`
- `running`
- `completed`
- `blocked`
- `failed`
- `published`

## Probe Outcome

Each probe outcome includes:

- `probeOutcomeId`
- `smokeReportId`
- `tenantId`
- `integrationId`
- `integrationAccountId`
- `domainKind`
- `providerKind`
- `probeAction`
- `result`
- `reasonCode`
- `remediationHint`
- `retrySafety`
- `approvalRefs`
- `artifactRefs`
- `checkedAt`
- `redactionStatus`
- `retentionExpiresAt`

Allowed outcome results:

- `passed`
- `failed`
- `blocked`
- `skipped`

## Skip And Blocked Reasons

Structured reasons must include:

- `missing_safe_credentials`
- `unsafe_side_effect_scope`
- `tenant_approval_unavailable`
- `provider_outage`
- `unsupported_domain`
- `operator_deferred`
- `missing_tenant_admin_approval`
- `missing_operator_approval`
- `redaction_failed_closed`

Missing safe credentials do not block release readiness when fake-backend coverage passes
and the skip reason is recorded.

## Safe Probe Policy

Default probes must be read-only or reversible. Non-idempotent or externally visible
probes:

- require tenant administrator approval,
- require authorized operator approval,
- must record approval references,
- must record a blocked outcome when either approval is missing,
- must not run automatically during ordinary diagnostic inspection.

## Release Readiness Linkage

Release-readiness evidence consumes:

- domain-level pass/fail/blocked/skipped summary,
- provider and integration account identifiers visible to the tenant,
- reason codes,
- remediation owner and hint,
- artifact references,
- explicit unsupported or limited classification for non-Feishu/Lark domains.

Release readiness must show whether each supported domain passed, failed, was blocked,
was skipped, was limited, or was deliberately unsupported.

## Contract Tests

Required tests:

- report fixtures cover passed, failed, blocked, and skipped outcomes,
- missing safe credentials produce explicit skipped or blocked output,
- risky probes are blocked without both approvals,
- fake-backend coverage remains mandatory when real-account smoke is skipped,
- report and artifact payloads do not expose credential material,
- release-readiness fixtures include Roadmap 42 diagnostic evidence.
