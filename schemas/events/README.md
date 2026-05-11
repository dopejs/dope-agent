# Event Schemas

This directory holds daemon event-envelope schemas and event-specific payload contracts.

Contract conformance is enforced by `daemon/internal/contracts`; run it with:

```bash
make daemon-contract-test
```

Evaluation events currently include:

- `evaluation.replay_started`
- `evaluation.replay_completed`
- `evaluation.replay_blocked`
- `evaluation.replay_unreplayable`
- `evaluation.replay_failed`
- `evaluation.comparison_completed`

These events are additive audit and refresh facts. Clients should fetch
`/v1/evaluation/*` resources for authoritative replay attempt and comparison state.

Connector conformance events added for Phase 48:

- `connector.conformance_result_recorded`
- `connector.diagnostic_state_changed`
- `connector.diagnostic_redaction_failed`
- `connector.inbound_duplicate_detected`
- `connector.route_outcome_recorded`
- `connector.foreground_reply_failed`
- `connector.delivery_separation_recorded`

Connector event payloads must stay tenant-scoped and redacted. Provider tokens,
authorization headers, raw provider payloads, and cross-tenant identifiers are not valid
event evidence.

Roadmap 55 continuity events are metadata-only audit facts:

- `thread.continuity_turn_recorded`
- `thread.continuity_preview_recorded`

These payloads must not include raw prompts, provider payloads, disallowed message
bodies, secrets, or cross-tenant identifiers.
