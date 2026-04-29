# Real-Account Smoke Policy

Fake-backend coverage is mandatory for every supported integration domain.
Real-account smoke is opt-in and uses operator-provided safe credentials only.

For each supported domain:

- run real-account smoke when safe credentials are available
- record an explicit skip reason when credentials are unavailable, expired,
  revoked, or unsafe
- keep fake-backend coverage passing regardless of real-account availability
- never log, report, back up, restore, or expose raw credential material

Readiness may pass with recorded skips only when all fake-backend and
operational evidence passes and the skipped real-account domains have explicit
reasons.
