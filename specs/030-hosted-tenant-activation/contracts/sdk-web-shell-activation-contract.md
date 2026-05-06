# Contract: SDK And Web Shell Activation

## SDK Contract

Add TypeScript resource types and methods without breaking existing client construction or
tenant request options.

### Types

- `ActivationStatus`
- `ActivationReasonCode`
- `ActivationReadinessItem`
- `ActivationQuotaBaseline`
- `ActivationFirstAction`
- `ActivationStateResource`
- `ActivationResponse`
- `RunActivationInput`
- `RunActivationTestChatInput`
- `ActivationTestChatMetadata`
- `ActivationTestChatResponse`
- `ActivationDiagnostic`
- `ActivationDiagnosticListResponse`

### Methods

```ts
getActivation(tenantOptions?: TenantRequestOptions): Promise<ActivationResponse>
activate(input?: RunActivationInput, tenantOptions?: TenantRequestOptions): Promise<ActivationResponse>
runActivationTestChat(input: RunActivationTestChatInput, tenantOptions?: TenantRequestOptions): Promise<ActivationTestChatResponse>
getActivationDiagnostics(tenantOptions?: TenantRequestOptions): Promise<ActivationDiagnosticListResponse>
```

### SDK Error Behavior

- Stable activation reason codes must be exposed through `DopeClientError.code` or an
  activation-specific payload field.
- Tenant denials continue to use existing tenant denial mapping.
- Quota baseline unavailable must be distinguishable from generic network/server failure.

## Web Shell Contract

The first-run shell must show activation state before tenant-scoped activation actions can
run.

### Required View State

- Active tenant display and tenant kind.
- Environment scope.
- Activation status.
- Quota baseline readiness and plan key when available.
- Blocking readiness reasons.
- Required v1 action: `test_chat`.
- Optional follow-up actions such as reminders or provider setup only after they do not
  block personal activation.

### Interaction Rules

- The activation surface uses the currently resolved active tenant.
- If active tenant access is denied or revoked, activation actions are disabled and
  previous-tenant activation data is cleared or marked stale.
- If quota baseline is unavailable, activation completion and test chat are blocked with
  `activation_blocked:quota_baseline_unavailable`.
- Test chat success marks activation `first_action_completed`.
- Test chat transcript or message content must not be shown in diagnostics/history panes
  as retained activation evidence. The immediate action result may show current-request
  feedback, but persisted activation projections remain metadata-only.

## Compatibility Assessment

- Existing SDK methods and constructor options remain compatible.
- Existing web shell tenant switch and onboarding behavior remain compatible; activation
  state is an additional first-run projection.
- Existing `operator-first-use-action` may include activation action values if the
  implementation chooses to surface activation through onboarding, but activation state
  remains authoritative under the new activation resource.

## Required Tests

- SDK calls send tenant intent through existing `TenantRequestOptions`.
- SDK parses activation state, readiness, quota baseline, and test chat metadata.
- SDK parses activation diagnostics, stage, reason code, retryability, remediation owner,
  and tenant scope without exposing forbidden data.
- SDK maps activation blocked/failed responses without raw text parsing.
- Web shell shows active tenant, environment, quota baseline, activation status, and test
  chat action within first-run flow.
- Web shell disables test chat when quota baseline is missing.
- Web shell clears or marks activation state stale after tenant switch, tenant denial, or
  tenant access revocation.
- Web shell marks activation complete after test chat success and reload.
- Web shell does not retain or render test chat transcript as diagnostic/audit evidence.
