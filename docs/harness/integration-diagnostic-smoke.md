# Integration Diagnostic Smoke

Roadmap 42 real-account smoke reports record pass, fail, blocked, and skipped outcomes
with stable reason codes, remediation hints, retry-safety classification, artifact
references, actor, timestamps, and 90-day default retention.

Smoke defaults to read-only or reversible probes. Non-idempotent or externally visible
probes require both tenant administrator approval and authorized operator approval before
they run. Missing credentials, missing approval, provider outage, unsupported domain, or
operator deferral must produce structured blocked or skipped outcomes.
