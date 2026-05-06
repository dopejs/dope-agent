# Feature Specification: Hosted Credential And OAuth Setup Wizard

**Feature Branch**: `031-credential-oauth-wizard`  
**Created**: 2026-05-06  
**Status**: Draft  
**Input**: User description: "$speckit-specify 结合 docs/specs/031-hosted-credential-and-oauth-setup-wizard.md 完成 phase 46 的工作"

**Upstream authority**: `docs/specs/031-hosted-credential-and-oauth-setup-wizard.md` is the authoritative upstream document for this work (Roadmap 46). This specification translates that document into testable scenarios, requirements, and success criteria. Where the upstream document and this spec disagree, the upstream document wins and this spec must be updated.

## Clarifications

### Session 2026-05-06

- Q: What setup target coverage is required for the v1 wizard? → A: v1 must cover one submitted-secret target and one OAuth target; other existing targets may show unsupported or action-required classifications.
- Q: Which tenant permissions are required for setup mutation and inspection? → A: Mutating setup requires both `secrets.manage` and `integrations.manage`; redacted inspection requires `credentials.inspect`.
- Q: Should setup sessions have a terminal `failed` state? → A: No. Recoverable setup failures must resolve to `action-required` or `unavailable` with stable reason codes.
- Q: Which setup states permit dependent credential-bearing use? → A: Ready permits normal use; degraded permits explicitly marked limited safe use; action-required, unavailable, cancelled, and disabled block use.
- Q: Which v1 proof targets must the setup wizard cover? → A: OpenAI-compatible provider credential setup and Feishu/Lark OAuth setup.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Connect A Provider Account (Priority: P1)

As a hosted user, I can follow a guided setup flow to connect a supported provider account and see whether the connection is ready to use.

**Why this priority**: Hosted users cannot be expected to understand secret references, provider authorization state, connector configuration, or diagnostic probes before they can use the product.

**Independent Test**: Can be fully tested by starting from an activated tenant with no connected account, completing a supported provider setup flow, and confirming the provider reaches a ready or clearly remediable state without developer-only or manual technical setup.

**Acceptance Scenarios**:

1. **Given** an activated hosted user has both credential mutation permissions for the active tenant, **When** the user starts a setup flow for a supported submitted-secret provider, submits required credential material, and confirms setup, **Then** the provider setup reaches a final ready, degraded, unavailable, action-required, or cancelled state with raw credential material hidden after submission.
2. **Given** an activated hosted user has both credential mutation permissions for the active tenant, **When** the user starts a setup flow for a supported OAuth-style provider and completes external authorization, **Then** the setup flow records completion state and shows whether the provider is ready or requires remediation.
3. **Given** the v1 proof targets are available in the active tenant, **When** setup is completed for OpenAI-compatible provider credentials and Feishu/Lark OAuth authorization, **Then** both proof targets produce setup state, diagnostic linkage, retry behavior, and redaction evidence.
4. **Given** setup completes, **When** the user views the integration surface, **Then** the user sees tenant-scoped provider readiness, credential status, and any diagnostic next step without learning internal resource models.

---

### User Story 2 - Retry, Replace, Cancel, Or Disable Setup (Priority: P1)

As a hosted user, I can recover from failed or obsolete credentials by retrying, replacing, cancelling, or disabling setup without leaking secrets or deleting unrelated integration state.

**Why this priority**: Credential and OAuth setup commonly fails because of user mistakes, missing approvals, expired tokens, or provider outages; recovery must be safe and obvious.

**Independent Test**: Can be fully tested by inducing a failed setup, retrying with corrected inputs or replacing credentials, and confirming unrelated integration metadata remains intact while the failed attempt remains diagnosable.

**Acceptance Scenarios**:

1. **Given** a setup attempt fails because credentials are invalid, missing scope, or provider authorization is incomplete, **When** the user retries the same setup flow, **Then** the user can provide new authorization or credential material and see the updated result without exposing prior secrets.
2. **Given** an account is already connected, **When** the user replaces credentials or reconnects OAuth authorization, **Then** new uses resolve the new authorized state and previous credential material remains hidden.
3. **Given** a setup attempt is in progress, **When** the user cancels it, **Then** the attempt is marked cancelled and unrelated integration state remains available for inspection or future repair.
4. **Given** a connected provider should no longer be used, **When** the user disables it, **Then** dependent credential-bearing use is blocked while redacted ownership and status metadata remain visible.

---

### User Story 3 - Diagnose Setup Failures (Priority: P2)

As an operator, I can inspect setup attempts and redacted diagnostics to identify whether a failure is caused by missing scope, tenant approval, token failure, provider outage, network failure, or unsupported setup target.

**Why this priority**: Public hosted setup will create support load; operators need stable diagnostic truth instead of manually inspecting storage, provider consoles, or raw logs.

**Independent Test**: Can be fully tested by inducing representative setup failures and confirming diagnostics include tenant scope, setup target, stage, reason, retryability, remediation owner, and safe evidence without raw credentials or OAuth payloads.

**Acceptance Scenarios**:

1. **Given** setup fails during secret submission, OAuth authorization, callback completion, diagnostic probing, or provider readiness, **When** an operator reviews setup diagnostics, **Then** the failure includes a stable reason, setup stage, retry safety, remediation owner, and affected tenant scope when accessible.
2. **Given** a provider requires tenant administrator approval or additional scopes, **When** diagnostics are shown to the user or operator, **Then** the next step identifies the needed approval or scope without exposing raw provider errors that contain credential material.
3. **Given** setup is retried after a recoverable failure, **When** the retry succeeds, **Then** the audit trail preserves the failed attempt and successful completion as tenant-scoped, redacted evidence.

### Edge Cases

- A user starts setup from an inactive, missing, or revoked tenant context; the setup flow must stop and show a stable tenant-access reason.
- A user without both credential mutation permissions tries to submit, replace, cancel, or disable setup; the system must deny the action without exposing whether inaccessible tenant credentials exist.
- A user without credential inspection permission tries to inspect redacted setup state; the system must deny inspection without exposing raw credential material or inaccessible tenant credential existence.
- The same external account is connected in two tenants; each tenant must own its own setup state, authorization, diagnostics, and remediation independently.
- A submitted secret is malformed, expired, revoked, or missing required provider permissions; setup must remain recoverable and hide the submitted value after receipt.
- OAuth authorization is abandoned, denied, replayed, expired, mismatched to another tenant, or returns an error payload; setup must produce a stable action-required, unavailable, or cancelled state without persisting raw callback payloads.
- The diagnostic probe cannot run because the provider is unavailable, rate-limited, network unreachable, or tenant approval is pending; setup must distinguish retryable provider failures from permission remediation.
- Setup is cancelled after credential material was submitted but before diagnostics complete; raw material must remain hidden and dependent use must not be marked ready.
- A setup target is degraded; dependent credential-bearing use must be limited to explicitly marked safe capabilities and must not proceed as normal ready-state use.
- A setup target is action-required, unavailable, cancelled, or disabled; dependent credential-bearing use must be blocked until the state is repaired or replaced.
- Setup is retried concurrently for the same tenant and provider target; attempts must converge to a coherent current setup state without losing prior redacted evidence.
- Replacing or disabling credentials must not delete unrelated connector, integration, MCP, or diagnostic metadata needed for repair.
- The daemon restarts during setup, after callback, or after diagnostic probing; setup state must remain durable and resumable.
- Redaction fails or credential-bearing content is detected in a setup record, diagnostic, audit event, log, fixture, or report; the setup evidence must fail closed and require operator remediation.
- Unsupported provider domains must receive a deliberate unsupported classification rather than a generic failure or accidental partial setup.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide a guided setup flow for supported credential-bearing provider and channel targets in the active tenant.
- **FR-002**: The setup flow MUST support both submitted-secret providers and OAuth-style authorization providers where those setup styles are already supported by existing domains.
- **FR-002a**: The v1 setup wizard MUST fully cover at least one submitted-secret target and at least one OAuth target, while other existing targets MAY expose deliberate unsupported or action-required classifications instead of full setup.
- **FR-002b**: The required v1 proof targets MUST be OpenAI-compatible provider credential setup for submitted-secret coverage and Feishu/Lark OAuth setup for OAuth coverage.
- **FR-003**: The setup flow MUST NOT add new provider domains or require users to configure unsupported domains through the wizard.
- **FR-004**: The system MUST persist each setup attempt with tenant, actor, target provider or channel, setup style, current state, redacted evidence, timestamps, and diagnostic linkage.
- **FR-005**: Setup state MUST use stable final and intermediate classifications, including ready, degraded, unavailable, cancelled, disabled, action-required, and in-progress.
- **FR-005a**: Recoverable setup failures MUST NOT create a terminal failed setup state; they MUST be represented as action-required or unavailable with stable reason codes and remediation guidance.
- **FR-006**: Raw credential material, OAuth authorization codes, access tokens, refresh tokens, callback payloads, provider secrets, and authorization headers MUST never be displayed after submission or stored in user-visible setup evidence.
- **FR-007**: Users with both secret-management and integration-management permission MUST be able to start, retry, replace, cancel, and disable setup for supported targets in the active tenant.
- **FR-008**: Users without both secret-management and integration-management permission MUST receive stable denials for setup mutation attempts without exposure of raw credential material or inaccessible tenant credential existence.
- **FR-008a**: Users MUST have credential-inspection permission to inspect redacted setup state; users without that permission MUST receive stable denials without credential details.
- **FR-009**: Setup completion MUST trigger or link to a diagnostic probe that classifies readiness, missing scope, tenant approval needed, token failure, provider unavailable, network failure, rate limit, unsupported target, and other stable provider authorization outcomes.
- **FR-010**: User-facing setup results MUST show remediation next steps from stable diagnostic reason codes rather than raw provider errors.
- **FR-011**: Operator-facing diagnostics MUST expose setup stage, reason code, retryability, remediation owner, tenant scope when accessible, target provider or channel, and redacted evidence links.
- **FR-012**: Setup attempts, retries, replacement, cancellation, disablement, diagnostic results, and remediation-relevant transitions MUST be tenant-scoped and audit-visible.
- **FR-013**: Failed setup attempts MUST remain recoverable without deleting unrelated integration, connector, MCP, provider authorization, or diagnostic metadata.
- **FR-014**: Replacing credentials or OAuth authorization MUST make the new authorized state the current setup state for future credential-bearing use while preserving redacted historical attempt evidence.
- **FR-015**: Disabling setup MUST block dependent credential-bearing use while retaining redacted ownership, target, and status metadata for inspection and repair.
- **FR-015a**: Ready setup state MUST permit normal dependent credential-bearing use, degraded setup state MUST permit only explicitly marked limited safe use, and action-required, unavailable, cancelled, or disabled setup states MUST block dependent credential-bearing use.
- **FR-015b**: Each degraded setup target MUST declare its allowed limited-safe capabilities in setup state and diagnostics before any degraded credential-bearing use is permitted.
- **FR-016**: Setup state MUST remain durable and resumable across reloads and daemon restarts before submission, after submission, after OAuth callback, and after diagnostic probing.
- **FR-017**: Client-facing setup state MUST be consistently available to hosted shell users and automated clients that need to guide setup or display remediation.
- **FR-018**: The feature MUST preserve compatibility with existing hosted activation, tenant credential isolation, integration health diagnostics, provider authorization, connector, secret, audit, and readiness behavior.
- **FR-019**: The feature MUST NOT introduce external managed secret managers, enterprise SSO, new integration domains, memory, context personalization, or autonomous remediation without user or operator action.

### Key Entities *(include if feature involves data)*

- **Setup Session**: A tenant-scoped guided attempt to connect, repair, replace, cancel, or disable a supported credential-bearing provider or channel target.
- **Setup Target**: The provider, channel, connector, integration account, or authorization binding being configured for the active tenant.
- **Setup Actor**: The authenticated user or operator initiating or inspecting setup, subject to tenant permissions. Mutation requires both secret-management and integration-management permission; redacted inspection requires credential-inspection permission.
- **Credential Submission**: A one-time submitted-secret input event whose raw value is accepted only for setup and never displayed afterward.
- **OAuth Authorization Attempt**: A tenant-scoped external authorization attempt, including start, callback, completion, failure, and cancellation state without raw token or callback payload exposure.
- **Setup State**: The current user- and operator-visible state of setup, including progress, final status, blockers, retryability, and remediation.
- **Diagnostic Probe**: A readiness check associated with setup that classifies authorization, permission, tenant approval, token freshness, provider availability, network, rate-limit, and unsupported-domain outcomes.
- **Setup Audit Record**: Tenant-scoped metadata-only evidence of setup start, submission, callback, completion, failure, retry, replacement, cancellation, disablement, and diagnostic transitions.
- **Redacted Evidence**: Safe setup and diagnostic metadata that can be displayed, persisted, logged, and tested without credential material or inaccessible tenant details.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: This feature adds guided hosted setup behavior, setup-session state, client-visible setup projection, redacted diagnostics, and tenant-scoped audit expectations. Existing secret, integration, provider authorization, connector, diagnostic, tenant activation, and readiness behavior must remain compatible and authoritative.
- **Migration / Rollback**: Existing credential and integration records must remain the source of truth. Rollback should hide the guided wizard and block new setup-session mutation while preserving existing credentials, integration bindings, provider authorization state, connector metadata, diagnostics, and already-written setup audit records for support review.
- **Verification Strategy**: Required validation includes setup lifecycle coverage, submitted-secret redaction, OAuth start/callback/completion and failure coverage, retry/replace/cancel/disable flows, permission denials, tenant isolation, diagnostic classification, restart recovery, client representation coverage, and a manual test-environment walkthrough from activated tenant to connected or action-required setup.
- **Observability Impact**: The feature must add or update setup diagnostics, stable reason codes, tenant audit records, operator-facing remediation fields, and redacted evidence links so setup failures can be investigated without raw storage inspection or provider-console archaeology.
- **Environment & Secrets**: Development and automated validation must default to the test environment. Tests must use fake credential values and safe OAuth fixtures unless explicit live-provider smoke is separately approved. Production secrets, live provider credentials, external managed secret managers, and enterprise identity credentials are out of scope for default verification.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of covered supported setup targets can be started from the hosted product surface without developer-only or manual technical setup.
- **SC-001a**: v1 coverage includes at least one submitted-secret target and at least one OAuth target with complete setup, diagnostic, retry, and redaction validation.
- **SC-001b**: OpenAI-compatible provider credential setup and Feishu/Lark OAuth setup both complete the v1 setup lifecycle coverage with setup state, diagnostic linkage, retry behavior, and redaction validation.
- **SC-002**: 100% of covered submitted-secret setup flows hide raw credential material after submission and show only redacted setup evidence.
- **SC-003**: 100% of covered OAuth-style setup flows produce a stable final or action-required state for success, denial, abandonment, expired authorization, tenant mismatch, and provider error outcomes.
- **SC-004**: 100% of covered setup attempts reach one of ready, degraded, unavailable, cancelled, disabled, or action-required within the defined test flow, with no ambiguous terminal state.
- **SC-004a**: 100% of covered recoverable setup failures are represented as action-required or unavailable with stable reason codes rather than a terminal failed setup state.
- **SC-005**: 100% of covered action-required or unavailable setup flows can be retried, replaced, cancelled, or disabled without deleting unrelated integration, connector, MCP, provider authorization, or diagnostic metadata.
- **SC-005a**: 100% of covered dependent-use checks allow normal use only for ready setup, allow limited use only for explicitly safe degraded capabilities, and block use for action-required, unavailable, cancelled, or disabled setup.
- **SC-006**: 100% of covered diagnostic failures classify missing scope, tenant approval needed, token failure, provider unavailable, network failure, rate limit, and unsupported target with stable reason codes and remediation owners.
- **SC-007**: Redaction validation finds zero raw secrets, OAuth authorization codes, access tokens, refresh tokens, provider secrets, callback payloads, authorization headers, or credential-bearing request bodies in setup state, diagnostics, audit records, logs, fixtures, reports, or client-visible output.
- **SC-008**: Tenant isolation tests show zero cross-tenant setup state, credential evidence, authorization state, diagnostic result, or remediation leakage across covered personal and organization tenant scenarios.
- **SC-009**: Restart recovery tests show 100% of setup sessions remain resumable or correctly terminal after restart before credential submission, after submission, after OAuth callback, and after diagnostic probing.
- **SC-010**: Operators can identify the failed setup stage, stable reason, retry safety, remediation owner, and affected tenant scope in 10 minutes or less for representative setup failures.
- **SC-011**: 100% of covered permission-denial cases return stable denials without exposing raw credential material or inaccessible tenant credential existence.
- **SC-011a**: Permission coverage includes mutation denial for users missing either required mutation permission and inspection denial for users missing credential-inspection permission.
- **SC-012**: Hosted users can identify whether a setup target is ready, action-required, degraded, unavailable, cancelled, or disabled in 30 seconds or less during first-run or repair workflows.

## Assumptions

- Roadmap 46 builds on Roadmap 37 hosted credential isolation, Roadmap 42 integration diagnostics, and Roadmap 45 hosted tenant activation.
- The active tenant is already resolved before a hosted setup wizard is shown.
- Existing secret, integration, provider authorization, connector, diagnostic, and readiness records remain authoritative; setup sessions orchestrate and explain those records rather than replacing them.
- v1 setup covers OpenAI-compatible provider credential setup as the submitted-secret proof target and Feishu/Lark OAuth setup as the OAuth proof target. Other existing supported provider and channel domains can expose unsupported or action-required classifications until selected for full wizard coverage.
- Submitted-secret and OAuth-style flows may have different user steps, but both must converge on the same tenant-scoped setup state and diagnostic model.
- Default verification uses fake secrets, fake OAuth fixtures, and the test environment. Real-account smoke is optional unless a later release-readiness gate explicitly selects it.
- Setup state gates future credential-bearing use: ready permits normal use, degraded permits explicitly safe limited use, and action-required, unavailable, cancelled, or disabled blocks use. Already captured historical audit evidence remains metadata-only and readable.
- External managed secret managers, enterprise SSO, new integration domains, memory, context recall, and personalized knowledge behavior remain out of scope for this phase.
