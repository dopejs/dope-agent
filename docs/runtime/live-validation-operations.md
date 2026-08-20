# Live Validation Operations

Roadmap 40 live validation is additive and rollback-safe. Disabling live validation
must block new live starts while preserving historical attempts, side-effect ledger
entries, reconciliation decisions, and comparisons for audit.

## Rollback

1. Enable the tenant or global live-validation kill switch.
2. Confirm new live validation starts are denied before side effects.
3. Inspect running attempts for pending, submitted, or operator-action-needed work.
4. Reconcile ambiguous commits before applying any retention policy.
5. Leave non-live replay and historical inspection available unless separately
   restricted.

Tenant containment uses:

```bash
curl -X POST -H "Authorization: Bearer $KURA_TOKEN" \
  -H "X-Kura-Tenant-ID: $KURA_TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{"scope":"tenant","enabled":true,"reason":"operator containment"}' \
  http://127.0.0.1:19192/v1/live-validations/kill-switches
```

Inspect active switches with:

```bash
curl -H "Authorization: Bearer $KURA_TOKEN" \
  -H "X-Kura-Tenant-ID: $KURA_TENANT_ID" \
  http://127.0.0.1:19192/v1/live-validations/kill-switches
```

When a switch is enabled, queued, awaiting-approval, and running validation attempts
are moved to `aborted`; already terminal ledger entries remain unchanged, while
non-terminal ledger entries are marked `aborted`.

## Retention

Live-validation attempts, ledger entries, reconciliation decisions, and comparisons
are retained indefinitely by default. An explicit operator retention policy may be
introduced later, but active operator-action-needed states must not be deleted before
resolution.

## Evidence Expectations

Operator-visible evidence should explain permission, quota, kill-switch, support,
approval, side-effect, retry, abort, ambiguous-commit, and reconciliation decisions
without relying on raw logs.
