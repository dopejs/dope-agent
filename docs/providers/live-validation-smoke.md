# Live Validation Real-Account Smoke

Real-account smoke for Roadmap 40 is optional. Automated acceptance must pass with fake
backends first.

## Preconditions

- The operator explicitly selects the tenant and side-effect scope.
- Safe test credentials are available and owned by the operator.
- Non-idempotent mutations have per-action approval.
- Secrets are not logged, copied into fixtures, or included in evidence payloads.
- A tenant owner/admin or explicit reconciliation permission holder is available for
  ambiguous commits.

## Smoke Evidence

Record structured evidence for:

- one successful supported side-effect replay,
- one denied live validation request,
- one unsupported tool class,
- one ambiguous-commit reconciliation path,
- one kill-switch abort of pending or future side effects,
- inspection of validation history and ledger entries after restart.

If safe credentials are unavailable, record the skip rationale and rely on fake-backend
coverage.

Fake-backend coverage is the required baseline. Real-account smoke must be skipped when
credentials, tenant approval, per-action approval for non-idempotent mutations, or a
qualified reconciliation operator are unavailable.
