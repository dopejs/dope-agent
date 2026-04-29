# Live Validation

Live validation is the explicit Roadmap 40 path for validating replay candidates
against live-capable systems while preserving operator control and durable evidence.
Non-live replay remains the default.

## Operator Flow

1. Load the operator shell against the test daemon and resolve an active tenant.
2. Select an eligible replay candidate from **Evaluation Replay**.
3. Declare the live-validation side-effect scope in **Live Validation Scope**:
   included tool classes, explicit exclusions when needed, and approval mode.
   The daemon evaluates readiness against the candidate's complete reachable
   `toolClasses`; the included scope is only the subset allowed to run.
4. Resolve required fresh approvals. Scope-level approval can cover read-only and
   idempotent classes; non-idempotent mutations require per-action approval.
5. Start live validation. The daemon checks permission, quota, kill switch, support
   matrix, explicit scope, and approval state before any live side effect can run.
6. Inspect gate status from the shell or with:

   ```bash
   curl -H "Authorization: Bearer $DOPE_TOKEN" \
     -H "X-Dope-Tenant-ID: $DOPE_TENANT_ID" \
     http://127.0.0.1:19192/v1/live-validations
   ```

7. Inspect the side-effect ledger and original-versus-live comparison when those
   records are produced.
8. Reconcile operator-action-needed outcomes only as a tenant owner/admin or a
   principal with `live_validation.reconcile`.

The direct start route is `POST /v1/live-validations`. The replay-candidate scoped
handoff route is `POST /v1/evaluation/replay-candidates/{candidateId}/live-validations`.
The existing non-live replay attempt route remains default-only; `mode:
live_validation` on `/attempts` is rejected so it cannot bypass Roadmap 40 gates.

## Default Safety Rules

- `live_validation.execute` is required to start a validation.
- Hosted quota state fails closed when unavailable.
- The Roadmap 38 `live_validation_attempts` preflight reservation is released when
  a later preflight gate blocks before live start, and committed only after a running
  validation attempt is durably recorded.
- Tenant and global kill switches block new starts.
- Running attempts abort pending and future side effects when a kill switch becomes
  active.
- Already-submitted side effects remain truthful ledger evidence and resolve to
  completed, failed, or operator-action-needed.
- Missing support matrix rows are unsupported and never live-replayed silently.

## Verification

Automated verification must use fake backends. Optional real-account smoke requires
explicit operator scope and safe credentials.
