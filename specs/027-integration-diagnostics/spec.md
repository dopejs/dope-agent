# Feature Specification: Integration Health And Permission Diagnostics

**Feature Branch**: `027-integration-diagnostics`  
**Created**: 2026-04-30  
**Status**: Draft  
**Input**: User description: "$speckit-specify 结合 docs/specs/027-integration-health-and-permission-diagnostics.md 完成 phase 42 的工作"

## Clarifications

### Session 2026-04-30

- Q: Which diagnostic coverage boundary should phase 42 require? → A: Full Feishu/Lark diagnostics now; all other supported domains must show structured unsupported or limited diagnostic classification.
- Q: What freshness rule should diagnostic state use? → A: Cached diagnostic state may be shown, but it becomes stale after 15 minutes; user actions that fail must produce current diagnostic truth.
- Q: How long should diagnostic runs and smoke report evidence be retained by default? → A: 90 days by default.
- Q: What should happen if diagnostic evidence cannot be confidently redacted? → A: Fail closed: suppress the diagnostic detail, emit redaction-failure audit evidence, and show only a generic safe classification.
- Q: Who must approve non-idempotent or externally visible real-account smoke probes? → A: Both a tenant administrator and an authorized operator.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Inspect Integration Readiness (Priority: P1)

As an operator, I can inspect an integration account and see whether its app or bot authorization, user authorization, tenant approval, provider scopes, token freshness, provider availability, and network reachability are healthy.

**Why this priority**: Operators need a single product-visible source of diagnostic truth before provider failures can be remediated, reviewed, or used in release-readiness evidence.

**Independent Test**: Can be fully tested by seeding representative Feishu/Lark diagnostic states for one tenant and verifying that an authorized operator sees scoped readiness, reason codes, timestamps, and remediation hints without secret material.

**Acceptance Scenarios**:

1. **Given** a Feishu/Lark integration has healthy app credentials, user authorization, tenant approval, required scopes, and fresh tokens, **When** an operator inspects diagnostic state, **Then** the operator sees a healthy result with the checked authorization dimensions and the diagnostic run time.
2. **Given** a Feishu/Lark integration is missing a required scope, **When** an operator inspects diagnostic state, **Then** the operator sees a stable missing-scope reason code, the affected capability, the remediation owner, and no raw credential material.
3. **Given** an operator can access tenant A but not tenant B, **When** the operator requests diagnostics for tenant B, **Then** the request is denied without revealing whether tenant B has a matching integration account.

---

### User Story 2 - Remediate User-Facing Failures (Priority: P1)

As a product user, I can see a clear next step when an integration action fails because authorization, tenant approval, provider availability, or retry safety blocks the action.

**Why this priority**: Users should not receive raw provider errors or ambiguous failures when the system can explain who must act and whether retrying is safe.

**Independent Test**: Can be fully tested by causing representative permission, authorization, provider-outage, rate-limit, and unsafe-retry failures and verifying that each failure produces a stable user-facing remediation message.

**Acceptance Scenarios**:

1. **Given** a calendar action fails because tenant administrator approval is required, **When** the user views the failure, **Then** the user sees that tenant administrator approval is needed and does not see raw provider error text.
2. **Given** a user authorization has expired or been revoked, **When** the user attempts a protected integration action, **Then** the user sees an authorization remediation path owned by the user.
3. **Given** a side-effecting integration action may have committed downstream but returned an ambiguous provider failure, **When** the failure is shown, **Then** the user sees that automatic retry is unsafe and operator review is required.

---

### User Story 3 - Classify Provider Failures Consistently (Priority: P2)

As an engineer or release reviewer, I can confirm that provider-specific failures are mapped to stable system-level reason codes and retry-safety categories.

**Why this priority**: Stable classifications are required before diagnostics can be reused across integration domains, live validation, delivery workflows, and release evidence.

**Independent Test**: Can be fully tested by replaying provider error fixtures for Feishu/Lark and other supported domains and verifying the expected system-level reason code, retry classification, and remediation owner.

**Acceptance Scenarios**:

1. **Given** a provider returns distinguishable evidence for app scope missing, user authorization missing, and tenant approval pending, **When** failures are classified, **Then** each failure maps to a different stable reason code.
2. **Given** provider evidence is incomplete or ambiguous, **When** the failure is classified, **Then** the diagnostic result uses an explicit unknown or ambiguous reason instead of inventing a more specific cause.
3. **Given** the same downstream failure appears in diagnostic state, smoke reports, audit events, and user-facing failure details, **When** each surface is reviewed, **Then** they use the same stable reason code and compatible remediation meaning.

---

### User Story 4 - Produce Real-Account Smoke Evidence (Priority: P2)

As an engineer, I can run a safe real-account smoke matrix and receive a structured report showing pass, fail, blocked, and skipped outcomes with remediation fields.

**Why this priority**: Release reviewers need repeatable evidence from real external systems without leaking secrets or creating uncontrolled provider side effects.

**Independent Test**: Can be fully tested by running the smoke matrix against configured safe accounts, including unavailable credentials or approval states, and verifying the structured report fields, skip reasons, artifact links, and redaction behavior.

**Acceptance Scenarios**:

1. **Given** safe Feishu/Lark credentials and tenant approval are available, **When** an engineer runs the smoke matrix, **Then** the report records each domain, tenant, integration account, probe action, outcome, reason code, remediation hint, timestamp, and supporting artifact link.
2. **Given** safe credentials are unavailable, tenant approval is unavailable, the provider is down, or the operator defers the run, **When** smoke is evaluated, **Then** the report records an explicit skipped or blocked outcome with the structured reason.
3. **Given** a probe would create a non-reversible external side effect, **When** smoke is prepared, **Then** the probe is not run unless both a tenant administrator and an authorized operator explicitly approve that scope.

---

### User Story 5 - Audit Diagnostic Runs And State Changes (Priority: P3)

As an operator or auditor, I can review diagnostic runs and remediation-relevant state transitions for the tenants I am allowed to access.

**Why this priority**: Diagnostics influence remediation, release decisions, and retry behavior, so their execution and state transitions must be traceable without exposing secrets.

**Independent Test**: Can be fully tested by running diagnostics, changing authorization state, publishing smoke reports, and verifying scoped audit evidence for authorized and unauthorized viewers.

**Acceptance Scenarios**:

1. **Given** a diagnostic run changes an integration from healthy to missing authorization, **When** an authorized operator reviews audit evidence, **Then** the operator sees the run identity, actor, tenant, result transition, reason code, and timestamp.
2. **Given** a diagnostic run includes provider error details, **When** audit evidence is reviewed, **Then** raw secrets, tokens, authorization headers, and credential-bearing payloads are absent.
3. **Given** release readiness references diagnostic or smoke evidence, **When** a reviewer follows the evidence link, **Then** the reviewer can determine which domains passed, failed, were blocked, or were skipped.

### Edge Cases

- Provider evidence is too ambiguous to distinguish missing scope from tenant approval pending; the result must use an explicit ambiguous permission reason and recommend the safest remediation owner.
- Provider errors include localized text, nested payloads, request identifiers, or credential-bearing fields; diagnostics must preserve useful non-secret context while redacting sensitive material.
- Diagnostic evidence cannot be confidently redacted; the diagnostic detail must be suppressed, redaction-failure audit evidence must be emitted, and only a generic safe classification may be shown.
- Tokens expire or are revoked between a successful readiness check and a user action; the action failure must produce current diagnostic truth instead of relying on cached readiness.
- A token refresh attempt fails after partial provider interaction; the diagnostic result must distinguish refresh failure from user authorization missing where evidence permits.
- The provider is rate-limited, unavailable, or unreachable through the local network; those failures must not be reported as permission failures.
- A side-effecting provider action returns an error after the downstream system may have committed the action; automatic retry must be treated as unsafe unless commit evidence proves otherwise.
- The same external account is connected to multiple tenants; diagnostic reads, smoke reports, remediation hints, and audit events must remain tenant-scoped.
- A domain is not yet supported by diagnostics; users and reviewers must see a deliberate not-yet-supported classification rather than a silent absence.
- Real-account smoke cannot safely run because credentials, tenant approval, or reversible probes are unavailable; the report must record an explicit blocked or skipped outcome.
- A non-idempotent or externally visible real-account smoke probe is requested without both tenant administrator and authorized operator approval; the probe must not run and must record a blocked outcome.
- A diagnostic run or smoke report is requested by a user without permission; the denial must not disclose inaccessible tenant existence, integration names, or credential state.
- Diagnostic run or smoke report evidence reaches its default retention limit; it must expire from normal inspection after 90 days unless a longer authorized retention policy applies.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST expose tenant-scoped integration diagnostic results for supported integration accounts.
- **FR-002**: Diagnostic results MUST identify the tenant, integration domain, provider, integration account, checked capability or probe, diagnostic status, reason code, remediation hint, retry-safety classification, evidence timestamp, and freshness state.
- **FR-002a**: Cached diagnostic state MAY be shown for inspection, but it MUST be marked stale after 15 minutes.
- **FR-003**: The system MUST provide stable reason codes for healthy state, app or bot authorization missing, user authorization missing, tenant approval pending, scope missing, token missing, token expired, token revoked, token refresh failed, tenant mismatch, rate limited, provider unavailable, network failed, transient provider failure, ambiguous downstream commit, unsafe retry, unsupported diagnostic, and unknown provider error.
- **FR-004**: Diagnostics MUST distinguish app or bot authorization health from user authorization health.
- **FR-005**: Diagnostics MUST distinguish missing provider scope from tenant approval pending when provider evidence permits that distinction.
- **FR-006**: Diagnostics MUST use an explicit ambiguous permission classification when provider evidence does not safely distinguish scope, approval, or authorization causes.
- **FR-007**: Diagnostics MUST classify token missing, token expired, token revoked, missing refresh credentials, and token refresh failure separately.
- **FR-008**: Diagnostics MUST classify rate limits, provider outages, transient provider failures, and local network failures separately from permission and authorization failures.
- **FR-009**: Side-effecting integration failures MUST be classified as retryable, unsafe to retry, or operator-action-needed based on idempotency expectations and available commit evidence.
- **FR-010**: Remediation hints MUST identify the remediation owner as product user, tenant administrator, operator, provider, or no action needed.
- **FR-011**: Product users MUST receive stable remediation messages for integration failures without raw provider error text, secrets, tokens, app secrets, refresh tokens, authorization headers, or credential-bearing payloads.
- **FR-011a**: User-facing integration failures MUST produce current diagnostic truth before presenting remediation, even when cached diagnostic state exists.
- **FR-012**: Operators MUST be able to inspect the latest diagnostic state and recent diagnostic runs for integration accounts they are authorized to access.
- **FR-013**: Diagnostic projections for operator and client surfaces MUST use the same reason-code meanings and retry-safety meanings.
- **FR-014**: Provider-specific error classification MUST map provider evidence to stable system-level reason codes while preserving only redacted diagnostic detail.
- **FR-015**: The Feishu/Lark full proof domain MUST cover app or bot authorization, user authorization, tenant approval, scopes, token freshness, provider availability, rate limits, network reachability, and safe smoke probes.
- **FR-016**: Feishu/Lark MUST provide full diagnostic coverage in this phase; every other supported integration domain MUST provide either a limited structured diagnostic result or a deliberate unsupported-diagnostic classification.
- **FR-017**: Real-account smoke reports MUST include domain, tenant, integration account, probe action, result, reason code, remediation hint, retry-safety classification where relevant, timestamp, actor, and artifact links.
- **FR-018**: Real-account smoke outcomes MUST distinguish passed, failed, blocked, and skipped.
- **FR-019**: Real-account smoke skip or blocked reasons MUST include missing safe credentials, unsafe side-effect scope, tenant approval unavailable, provider outage, unsupported domain, and operator-deferred.
- **FR-020**: Real-account smoke MUST default to read-only or reversible probes.
- **FR-021**: Non-idempotent or externally visible smoke probes MUST require explicit tenant administrator approval and authorized operator approval before they run.
- **FR-022**: Diagnostic runs, diagnostic state transitions, remediation-relevant changes, smoke report publication, and permission denials MUST emit audit evidence.
- **FR-022a**: Diagnostic runs and smoke report evidence MUST use a 90-day default retention period unless an authorized retention policy requires a different period.
- **FR-023**: Diagnostic state, smoke reports, remediation hints, and audit evidence MUST enforce tenant isolation for reads and writes.
- **FR-024**: Diagnostics, reports, logs, events, fixtures, audit evidence, and evaluation artifacts MUST redact raw secrets, OAuth tokens, refresh tokens, app secrets, authorization headers, and credential-bearing request or response payloads.
- **FR-024a**: If diagnostic evidence cannot be confidently redacted, the system MUST suppress the diagnostic detail, emit redaction-failure audit evidence, and show only a generic safe classification.
- **FR-025**: Release-readiness evidence MUST be able to include diagnostic and smoke outcomes for supported domains, including explicit pass, fail, blocked, and skipped states.
- **FR-026**: Fake-backend diagnostic coverage MUST remain available for every classified outcome required by this feature, even when real-account smoke is available.

### Key Entities

- **Integration Diagnostic Result**: Tenant-scoped diagnostic state for a provider account or capability; includes status, reason code, remediation hint, retry-safety classification, freshness, redacted evidence summary, and checked dimensions.
- **Diagnostic Reason Code**: A stable system-level classification that represents provider health, authorization, permission, token, network, rate-limit, retry-safety, unsupported, or unknown states.
- **Remediation Hint**: User-facing or operator-facing guidance tied to a reason code; includes owner, next step, severity, and whether retry is safe.
- **Diagnostic Run**: A bounded diagnostic execution for one tenant and integration account; includes actor, checked probes, result transitions, timestamps, redaction status, and retention expiry.
- **Provider Error Classification**: Redacted interpretation of provider-specific evidence into a diagnostic reason code, retry-safety category, and remediation owner.
- **Smoke Matrix Report**: Structured real-account smoke evidence for one run; includes domain outcomes, account scope, skipped or blocked reasons, artifact links, publication status, and retention expiry.
- **Smoke Probe Outcome**: Result of an individual safe probe; includes action name, result, reason code, remediation hint, retry-safety classification where relevant, timestamp, and redacted evidence.
- **Integration Diagnostic Audit Event**: Tenant-scoped audit evidence for diagnostic execution, state transition, smoke report publication, remediation-relevant change, permission denial, or redaction failure.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Diagnostics are additive product behavior across integration status, user-facing failure details, operator inspection, smoke reports, audit evidence, and release-readiness projections. Existing integration actions remain compatible, but their failures gain stable diagnostic classifications and remediation text.
- **Migration / Rollback**: Rollout should introduce diagnostic state and smoke report publication without changing existing provider credentials or integration ownership. Rollback must be able to disable diagnostic runs, hide new diagnostic projections, and stop smoke publication while preserving existing integration behavior and already-written audit evidence for authorized operators.
- **Verification Strategy**: Required validation includes provider error classification tests, Feishu/Lark diagnostic fixture coverage, tenant isolation tests, redaction tests across reports and audit evidence, user-facing remediation tests, operator inspection tests, smoke report fixture tests for pass, fail, blocked, and skipped outcomes, and release-readiness evidence checks.
- **Observability Impact**: The feature must add operator-visible diagnostic run history, state-transition audit evidence, redaction failure evidence, smoke report publication evidence, retry-safety visibility, and release-readiness summaries for pass, fail, blocked, skipped, and unsupported states.
- **Environment & Secrets**: Development and automated verification must default to the test environment. Live connector or real-account smoke usage requires explicit safe credentials and tenant approval, and secret or credential material must never appear in diagnostics, reports, logs, events, audit evidence, fixtures, or evaluation artifacts.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An authorized operator can determine the current diagnostic status and remediation owner for a configured Feishu/Lark integration within 2 minutes of starting inspection.
- **SC-001a**: 100% of diagnostic inspection views mark cached diagnostic state older than 15 minutes as stale.
- **SC-002**: 100% of representative Feishu/Lark fixture cases for app or bot authorization, user authorization, tenant approval, missing scope, token expiry, token revocation, token refresh failure, rate limit, provider outage, network failure, and healthy readiness map to the expected stable reason code.
- **SC-003**: 100% of seeded permission-denial and cross-tenant access attempts are denied without exposing inaccessible tenant, integration account, or credential existence.
- **SC-004**: 100% of diagnostics, smoke reports, logs, events, audit evidence, fixtures, and evaluation artifacts in redaction tests exclude raw secrets, OAuth tokens, refresh tokens, app secrets, authorization headers, and credential-bearing payloads.
- **SC-004a**: 100% of redaction-uncertain fixture cases fail closed by suppressing diagnostic detail, emitting redaction-failure audit evidence, and showing only a generic safe classification.
- **SC-005**: Product users receive a remediation message with owner and next step for at least 95% of representative integration failures without seeing raw provider error text.
- **SC-006**: Real-account smoke reports represent pass, fail, blocked, and skipped outcomes with structured reason codes and remediation fields for 100% of seeded report cases.
- **SC-006a**: 100% of diagnostic runs and smoke report evidence in retention tests expire from normal inspection after 90 days unless covered by an authorized longer retention policy.
- **SC-007**: Safe real-account smoke for an approved Feishu/Lark diagnostic path completes within 10 minutes when safe credentials and tenant approval are available, or records a structured blocked or skipped outcome when they are not.
- **SC-007a**: 100% of non-idempotent or externally visible smoke probe attempts without both tenant administrator and authorized operator approval are blocked and recorded with a structured reason.
- **SC-008**: 100% of representative side-effecting failure cases are classified as retryable, unsafe to retry, or operator-action-needed according to available commit evidence.
- **SC-009**: Release reviewers can identify which supported domains passed, failed, were blocked, were skipped, or are deliberately unsupported from a single readiness evidence set.

## Assumptions

- Roadmap 27 personal integrations, Roadmap 37 hosted secrets and connector isolation, Roadmap 39 production operations, Roadmap 40 live validation, and Roadmap 41 evaluation product capabilities are available as prerequisites.
- Feishu/Lark is the full proof domain for this phase; the diagnostic model must be reusable across calendar, mail, reminders, tasks, delivery, connectors, and provider authorization states, which may initially expose limited or unsupported diagnostic classifications.
- Diagnostics are product-visible behavior and not only local test scripts.
- Provider-specific raw errors may be retained only as redacted diagnostic details that do not expose secret material.
- Real-account smoke is allowed to skip or block only with explicit structured reasons.
- Real-account smoke defaults to read-only or reversible probes; externally visible or non-idempotent probes require both tenant administrator approval and authorized operator approval.
- The feature does not add new integration domains, bypass provider approval, perform autonomous remediation, or change tenant administrator controls.
