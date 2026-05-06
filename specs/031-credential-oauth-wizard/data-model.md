# Data Model: Hosted Credential And OAuth Setup Wizard

## SetupTarget

Represents a supported or explicitly unsupported setup target visible to the hosted
wizard.

**Fields**
- `targetId`: stable identifier such as `provider.openai_compatible` or
  `integration.feishu_lark`.
- `tenantId`: active tenant scope for target projection.
- `targetKind`: `provider`, `integration`, `channel`, or `connector`.
- `setupStyle`: `submitted_secret`, `oauth`, or `unsupported`.
- `displayName`: user-facing target name.
- `proofTarget`: boolean indicating whether this target is required for Roadmap 46 v1.
- `supportStatus`: `supported`, `unsupported`, or `action_required`.
- `requiredPermissions`: mutation and inspection permissions required for this target.
- `limitedSafeCapabilities`: target-declared capabilities that may continue when setup is
  degraded.
- `currentSessionId`: latest setup session for this tenant/target when one exists.
- `currentState`: projected setup state when one exists.
- `diagnosticResultId`: latest linked diagnostic result when one exists.

**Validation rules**
- V1 MUST include OpenAI-compatible provider credential setup as a submitted-secret proof
  target and Feishu/Lark OAuth setup as an OAuth proof target.
- Unsupported targets MUST expose deliberate unsupported or action-required
  classifications instead of starting a partial setup flow.
- Degraded targets MUST declare `limitedSafeCapabilities` before any dependent
  credential-bearing use can be allowed.

## SetupSession

Tenant-scoped guided attempt to connect, repair, replace, cancel, or disable a target.

**Fields**
- `setupSessionId`: stable setup-session identifier.
- `tenantId`: active tenant that owns the setup session.
- `actorPrincipalId`: authenticated actor who initiated the current attempt.
- `targetId`: setup target identifier.
- `targetKind`: target category.
- `setupStyle`: `submitted_secret` or `oauth`.
- `state`: `not_started`, `in_progress`, `ready`, `degraded`, `unavailable`,
  `cancelled`, `action_required`, or `disabled`.
- `reasonCode`: stable reason code when setup is not ready.
- `retryable`: whether retry is safe without replacement.
- `remediationOwner`: `product_user`, `tenant_admin`, `operator`, `provider`, or
  `none_required`.
- `safeUseMode`: `normal`, `limited_safe`, or `blocked`.
- `allowedCapabilities`: target-declared capabilities allowed only when `safeUseMode` is
  `limited_safe`.
- `currentAttemptId`: latest attempt identifier.
- `diagnosticResultId`: linked diagnostic result for current state.
- `redactionStatus`: `redacted`, `suppressed`, or `failed_closed`.
- `createdAt`, `updatedAt`, `lastTransitionAt`: timestamps.

**Uniqueness**
- Current setup state is unique by `tenantId + targetId + setupStyle`.
- Historical attempts are append-only and linked by `setupSessionId`.

**State transitions**
- `not_started -> in_progress`
- `in_progress -> ready`
- `in_progress -> degraded`
- `in_progress -> unavailable`
- `in_progress -> action_required`
- `in_progress -> cancelled`
- `ready -> in_progress` for replacement or reconnect
- `ready -> disabled`
- `degraded -> in_progress` for repair
- `unavailable -> in_progress` for retry
- `action_required -> in_progress` for retry or replacement
- `cancelled -> in_progress` for restart
- `disabled -> in_progress` for reconnect

Recoverable failures MUST NOT transition to a terminal failed setup state. They resolve
to `action_required` or `unavailable` with a reason code.

## SetupAttempt

Append-only redacted record of a setup operation.

**Fields**
- `attemptId`: attempt identifier.
- `setupSessionId`: owning setup session.
- `tenantId`: active tenant.
- `actorPrincipalId`: actor.
- `operation`: `start`, `submit_secret`, `oauth_start`, `oauth_callback`,
  `diagnostic_probe`, `retry`, `replace`, `cancel`, or `disable`.
- `fromState`, `toState`: setup states.
- `reasonCode`: stable reason code for transition.
- `redactedEvidence`: metadata only; no credential or OAuth payload material.
- `resourceRefs`: references to tenant secret, provider auth state, integration, or
  diagnostic records.
- `createdAt`: timestamp.

**Validation rules**
- Attempts never store raw submitted secret values, OAuth authorization codes, access
  tokens, refresh tokens, callback payloads, authorization headers, provider secrets, or
  credential-bearing request bodies.
- Redaction failure changes evidence status to failed-closed and blocks ready state.

## CredentialSubmission

One-time submitted-secret input event for targets such as OpenAI-compatible provider
credentials.

**Fields**
- `attemptId`: linked attempt.
- `secretRef`: tenant secret reference written or rotated.
- `secretVersionId`: active version identifier after successful submission.
- `targetId`: setup target.
- `redactionConfirmation`: proof that raw value is not returned or retained in setup
  evidence.
- `validationStatus`: `accepted`, `action_required`, or `unavailable`.

**Relationships**
- Creates or rotates a tenant secret through the existing tenant secret manager.
- Links to provider check or diagnostic result for readiness.

## OAuthSetupAttempt

Tenant-scoped external authorization attempt for targets such as Feishu/Lark.

**Fields**
- `attemptId`: linked setup attempt.
- `oauthStateId`: opaque state reference, not raw callback payload.
- `authorizationStatus`: `started`, `completed`, `denied`, `expired`, `mismatched`,
  `cancelled`, or `unavailable`.
- `providerKind`: provider family such as `feishu_lark`.
- `accountLabel`: optional redacted account label.
- `diagnosticResultId`: linked diagnostic result after completion or failure.
- `redactionStatus`: redaction outcome.

**Validation rules**
- OAuth authorization codes, access tokens, refresh tokens, callback payloads, and
  provider tokens never appear in setup records or client-visible output.
- Tenant mismatch produces `action_required` or `unavailable` with a stable reason code.

## SetupDiagnosticLink

Connects setup state to existing integration or provider diagnostics.

**Fields**
- `setupSessionId`: setup session.
- `diagnosticResultId`: linked diagnostic result.
- `diagnosticRunId`: optional diagnostic run.
- `status`: mapped setup diagnostic status.
- `reasonCode`: stable setup or integration diagnostic reason.
- `retrySafety`: retry classification.
- `remediationOwner`: owner.
- `checkedAt`, `staleAfter`: diagnostic freshness.

**Validation rules**
- Ready setup state requires diagnostic linkage unless the target is explicitly
  unsupported or action-required.
- Diagnostic evidence must already be redacted before linking.

## SetupAuditRecord

Tenant-scoped metadata-only evidence of setup transitions.

**Fields**
- `eventKind`: setup audit event family.
- `tenantId`: tenant scope.
- `principalId`: actor.
- `setupSessionId`: setup session.
- `targetId`: setup target.
- `operation`: setup operation.
- `fromState`, `toState`: transition.
- `reasonCode`: stable reason.
- `outcome`: `succeeded`, `blocked`, `cancelled`, or `failed_closed`.
- `resourceRefs`: redacted references.
- `createdAt`: timestamp.

**Validation rules**
- Audit records include metadata only and follow the same forbidden-field rules as setup
  attempts and diagnostics.

## DependentUseDecision

Projection used by provider, integration, connector, or channel execution to decide
whether setup state permits credential-bearing use.

**Fields**
- `tenantId`: active tenant.
- `targetId`: setup target.
- `setupState`: current setup state.
- `safeUseMode`: `normal`, `limited_safe`, or `blocked`.
- `allowedCapabilities`: capabilities allowed when degraded.
- `reasonCode`: reason when blocked or limited.
- `checkedAt`: timestamp.

**Rules**
- `ready` maps to `normal`.
- `degraded` maps to `limited_safe` only for capabilities declared by the target and
  echoed in the current setup diagnostic.
- `action_required`, `unavailable`, `cancelled`, and `disabled` map to `blocked`.
