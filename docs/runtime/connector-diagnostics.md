# Connector Diagnostics

Connector diagnostics use the Phase 48 shared reason-code vocabulary:
`auth_missing`, `permission_missing`, `rate_limited`, `provider_unavailable`,
`network_failed`, `unsupported_capability`, `blocked_route`, `duplicate_inbound`,
`reply_failed`, and `unknown_connector_failure`.

Each diagnostic state is tenant-scoped and redacted. Operator projections include the
reason code, remediation owner, retry safety, user-visible severity, evidence timestamp,
freshness, redaction status, and retention expiry. Cached diagnostics become stale after
15 minutes; connector action failures must produce current diagnostic truth before
showing remediation.

Detailed provider evidence is retained only when redaction is reliable. If evidence
cannot be confidently redacted, the connector suppresses detailed evidence, records a
redaction-failure outcome, and exposes only a generic safe classification. Connector
diagnostic evidence, conformance results, and redaction-failure outcomes use 90-day
default retention unless an authorized longer policy applies.

## Reply And Delivery Boundaries

Foreground replies belong to an active connector conversation and record their own
reply outcome. Background delivery belongs to a delivery target and records selected
target, attempt, retry or suppression, and terminal outcome independently. The same
connector transport may be reused, but a successful foreground reply is not proof that
background delivery succeeded, and a successful background delivery is not proof that an
active foreground reply completed.

Rollback for connector-backed delivery is to disable or remove the connector delivery
target while leaving active foreground connector replies untouched. Debugging should
compare connector message records, delivery attempts, and connector delivery-boundary
events by delivery id instead of merging the two product truths.
