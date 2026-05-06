# Wizard UI Contract

## Entry Points

The hosted shell must expose setup from existing first-run, provider, and integration
surfaces when the active tenant is resolved. The wizard must not appear as an anonymous
or tenantless flow.

Required entry points:
- First-run repair card after hosted activation.
- Provider setup row for OpenAI-compatible credentials.
- Integration setup row for Feishu/Lark OAuth.
- Diagnostics repair action when setup is action-required or unavailable.

## User-Visible States

| Setup state | UI behavior |
|-------------|-------------|
| `not_started` | Show Connect action |
| `in_progress` | Show current step, loading state, and Cancel action |
| `ready` | Show connected state and normal-use availability |
| `degraded` | Show limited safe-use warning and explicitly allowed capabilities from setup state |
| `action_required` | Show remediation action and retry/replace affordance |
| `unavailable` | Show retry-safe or provider/operator remediation guidance |
| `cancelled` | Show restart setup action |
| `disabled` | Show reconnect/replace action and blocked-use explanation |

The UI must not use terminal "failed" as setup session state.

## Submitted-Secret Flow

The OpenAI-compatible proof target flow must:
1. Explain the target and credential metadata without showing raw secret values.
2. Accept a one-time secret value.
3. Hide the value immediately after submission.
4. Show redacted secret reference/status and setup diagnostic result.
5. Offer retry, replace, cancel, and disable where state permits.

The UI must not render previously submitted secret values, masked values that imply
length, authorization headers, provider tokens, or raw provider error text.

## OAuth Flow

The Feishu/Lark proof target flow must:
1. Start from the active tenant.
2. Show the external authorization step or safe test-fixture equivalent.
3. Handle completed, denied, abandoned, expired, tenant-mismatched, and provider-error
   outcomes.
4. Show action-required or unavailable remediation from diagnostic reason codes.
5. Offer retry, replace, cancel, and disable where state permits.

The UI must not render raw OAuth authorization codes, access tokens, refresh tokens,
callback payloads, provider secrets, or credential-bearing request bodies.

## Tenant Switch And Revocation

- Switching tenants clears stale setup state before loading the new tenant projection.
- Revoked tenant access stops setup mutation and inspection for that tenant.
- Inaccessible tenant setup existence must not be disclosed to the user.
- Retrying setup after revocation must require a valid allowed tenant.

## Accessibility And Interaction

- Primary setup actions must be reachable by keyboard.
- Loading states must be visible without relying only on color.
- Error and remediation text must identify the stable reason and next action in product
  terms.
- Users must be able to identify setup state and next action in 30 seconds or less.

## Web Tests

Required test cases:
- OpenAI-compatible submitted-secret setup happy path hides raw secret after submission.
- Feishu/Lark OAuth happy path reaches ready with diagnostic linkage.
- OAuth denial, abandonment, expiry, tenant mismatch, provider unavailable, and missing
  scope show action-required or unavailable states.
- Retry, replace, cancel, and disable flows preserve redacted metadata.
- Missing mutation permissions disable mutation controls.
- Missing inspection permission blocks redacted setup display without leaking existence.
- Degraded targets render only the allowed limited-safe capabilities declared by setup
  state and diagnostics.
- Tenant switch clears stale setup state.
- Redaction tests prove forbidden credential/OAuth strings are absent from rendered DOM.
