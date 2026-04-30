# Evaluation Product Contract Fixtures

Fixtures in this directory cover Roadmap 41 API and event contracts:

- discovery policies, discovery runs, discovered candidates, candidate evidence,
  suppressions, and retention/deletion application results,
- product-managed fixtures, fixture revisions, review responses, and
  repo-managed fixture read-only projections,
- replay campaigns, campaign items, campaign attempt groups, result summaries,
  dashboard projections, and pagination cursors,
- tool-call inspection resources, classification states, and redacted diff
  payloads,
- audit/event payloads for lifecycle changes, redaction failures, denials, and
  retention/deletion application.

Fixture payloads must remain tenant-scoped and must not contain raw secrets,
credentials, access tokens, refresh tokens, session tokens, or real external
account identifiers.

