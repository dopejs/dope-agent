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
