# Setup API And SDK Contract

## Goals

- Expose hosted credential and OAuth setup as tenant-scoped product behavior.
- Keep existing tenant secrets, provider auth state, and integration diagnostics as the
  authoritative resources.
- Provide SDK methods and schemas that never expose raw credential or OAuth material.

## Permissions

| Operation | Required permission |
|-----------|---------------------|
| List setup targets | `credentials.inspect` |
| Read setup session | `credentials.inspect` |
| Start setup | `secrets.manage` and `integrations.manage` |
| Submit secret | `secrets.manage` and `integrations.manage` |
| Start OAuth | `secrets.manage` and `integrations.manage` |
| Complete OAuth callback | setup state validation plus original tenant scope |
| Retry setup | `secrets.manage` and `integrations.manage` |
| Replace setup | `secrets.manage` and `integrations.manage` |
| Cancel setup | `secrets.manage` and `integrations.manage` |
| Disable setup | `secrets.manage` and `integrations.manage` |
| Inspect redacted diagnostics | `credentials.inspect` |

Permission denials must not reveal inaccessible tenant credential existence.

## Setup Target Resource

```json
{
  "targetId": "provider.openai_compatible",
  "tenantId": "ten_personal",
  "targetKind": "provider",
  "setupStyle": "submitted_secret",
  "displayName": "OpenAI-compatible provider",
  "proofTarget": true,
  "supportStatus": "supported",
  "requiredPermissions": ["secrets.manage", "integrations.manage"],
  "limitedSafeCapabilities": ["metadata_read"],
  "currentSessionId": "setup_ten_personal_provider_openai_compatible",
  "currentState": "action_required",
  "diagnosticResultId": "diag_openai_setup"
}
```

`setupStyle` values: `submitted_secret`, `oauth`, `unsupported`.

`supportStatus` values: `supported`, `unsupported`, `action_required`.

## Setup Session Resource

```json
{
  "setupSessionId": "setup_ten_personal_provider_openai_compatible",
  "tenantId": "ten_personal",
  "actorPrincipalId": "prn_123",
  "targetId": "provider.openai_compatible",
  "targetKind": "provider",
  "setupStyle": "submitted_secret",
  "state": "action_required",
  "reasonCode": "credential_missing",
  "retryable": true,
  "remediationOwner": "product_user",
  "safeUseMode": "blocked",
  "allowedCapabilities": [],
  "currentAttemptId": "attempt_123",
  "diagnosticResultId": "diag_123",
  "redactionStatus": "redacted",
  "resourceRefs": [
    {"kind": "tenant_secret", "id": "OPENAI_COMPATIBLE_API_KEY"}
  ],
  "createdAt": "2026-05-06T00:00:00Z",
  "updatedAt": "2026-05-06T00:00:00Z",
  "lastTransitionAt": "2026-05-06T00:00:00Z"
}
```

`state` values: `not_started`, `in_progress`, `ready`, `degraded`, `unavailable`,
`cancelled`, `action_required`, `disabled`.

No terminal `failed` setup state is allowed.

`safeUseMode` values: `normal`, `limited_safe`, `blocked`.

`allowedCapabilities` is empty unless `safeUseMode` is `limited_safe`. When setup is
degraded, every allowed capability must be declared by the target catalog and confirmed by
the linked diagnostic result.

## Endpoint Contract

| Route | Method | Purpose |
|-------|--------|---------|
| `/v1/setup/targets` | GET | List setup targets for active tenant |
| `/v1/setup/sessions` | GET | List setup sessions for active tenant |
| `/v1/setup/sessions` | POST | Start setup for a target |
| `/v1/setup/sessions/{sessionId}` | GET | Read one setup session |
| `/v1/setup/sessions/{sessionId}/submit-secret` | POST | Submit or rotate secret material |
| `/v1/setup/sessions/{sessionId}/oauth/start` | POST | Start OAuth authorization |
| `/v1/setup/sessions/{sessionId}/oauth/callback` | POST | Complete OAuth callback with redacted evidence |
| `/v1/setup/sessions/{sessionId}/retry` | POST | Retry a recoverable setup state |
| `/v1/setup/sessions/{sessionId}/replace` | POST | Start replacement flow |
| `/v1/setup/sessions/{sessionId}/cancel` | POST | Cancel in-progress setup |
| `/v1/setup/sessions/{sessionId}/disable` | POST | Disable dependent credential-bearing use |
| `/v1/setup/sessions/{sessionId}/diagnostics` | GET | Read linked setup diagnostics |

All routes are protected and tenant-scoped. Mutation routes require the active tenant to
match the setup session tenant.

## Request Shapes

### Start Setup

```json
{
  "targetId": "provider.openai_compatible",
  "setupStyle": "submitted_secret",
  "source": "wizard"
}
```

### Submit Secret

```json
{
  "secretRef": "OPENAI_COMPATIBLE_API_KEY",
  "value": "submitted once and never returned",
  "displayName": "OpenAI-compatible API key"
}
```

The response must never include `value`.

### OAuth Start

```json
{
  "redirectRoute": "/setup/oauth/feishu-lark/callback"
}
```

Response includes an authorization URL or test-fixture equivalent and an opaque setup
state reference. It must not include provider client secret material.

### OAuth Callback

```json
{
  "state": "opaque-state-reference",
  "result": "completed"
}
```

Callback payloads are accepted only for completion and are not stored verbatim.

## Error Contract

```json
{
  "error": "setup permission denied",
  "code": "setup_denied:missing_permission",
  "reasonCode": "setup_denied:missing_permission",
  "stage": "permission",
  "retryable": false,
  "remediationOwner": "tenant_admin"
}
```

Reason-code families:
- `setup_denied:missing_permission`
- `setup_denied:tenant_access`
- `setup_blocked:unsupported_target`
- `setup_action_required:credential_missing`
- `setup_action_required:scope_missing`
- `setup_action_required:tenant_approval_pending`
- `setup_action_required:oauth_denied`
- `setup_unavailable:provider_unavailable`
- `setup_unavailable:network_failed`
- `setup_unavailable:redaction_failed_closed`
- `setup_cancelled:user_cancelled`
- `setup_failed:persistence`
- `setup_failed:audit_write`
- `setup_failed:unexpected`

`setup_failed:*` reason codes may be used for operation errors but must not create a
terminal failed setup-session state.

## SDK Contract

SDK methods:
- `listSetupTargets(tenantOptions?)`
- `listSetupSessions(tenantOptions?)`
- `startSetup(input, tenantOptions?)`
- `getSetupSession(sessionId, tenantOptions?)`
- `submitSetupSecret(sessionId, input, tenantOptions?)`
- `startSetupOAuth(sessionId, input, tenantOptions?)`
- `completeSetupOAuth(sessionId, input, tenantOptions?)`
- `retrySetup(sessionId, tenantOptions?)`
- `replaceSetup(sessionId, input, tenantOptions?)`
- `cancelSetup(sessionId, tenantOptions?)`
- `disableSetup(sessionId, input, tenantOptions?)`
- `getSetupDiagnostics(sessionId, tenantOptions?)`

SDK types must preserve stable string literal unions for setup state, setup style,
safe-use mode, remediation owner, redaction status, and reason codes.

## Contract Tests

- Schema fixtures reject responses containing `value`, `authorizationCode`,
  `accessToken`, `refreshToken`, `providerToken`, `callbackPayload`, `Authorization`, or
  `clientSecret`.
- Permission fixtures cover missing `secrets.manage`, missing `integrations.manage`, and
  missing `credentials.inspect` for list/read/diagnostics inspection.
- State fixtures prove recoverable failures use `action_required` or `unavailable`, not
  terminal `failed`.
- Degraded fixtures prove `allowedCapabilities` is declared by both target and diagnostic
  evidence before dependent use is allowed.
