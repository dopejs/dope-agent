# Roadmap 40 Fake-Backend Fixtures

Roadmap 40 automated acceptance must use fake backends by default. Fixture data in
this directory should exercise supported side-effect paths without touching live
connectors, production tenants, or real external accounts.

Required fixture scenarios:

- accepted live validation with durable gate evidence,
- permission, quota, kill-switch, support, and approval denials,
- completed, failed, skipped, denied, aborted, and operator-action-needed ledger
  outcomes,
- timeout-after-submit, restart-after-submit, duplicate retry, and ambiguous commit,
- reminder lifecycle, delivery dispatch, connector message-send, integration,
  calendar, and mail fake side effects,
- manual reconciliation by an authorized tenant owner/admin or reconciliation
  permission holder,
- unsupported class reporting and mixed supported/unsupported exclusion.

Real-account smoke is documented separately and remains explicit opt-in.
