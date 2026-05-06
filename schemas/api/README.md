# API Schemas

This directory holds daemon HTTP API schemas.

Contract conformance is enforced by the Go contract suite in:

- `daemon/internal/contracts`

Run it with:

- `make daemon-contract-test`

Schema updates should ship together with:

- implementation changes
- contract test fixture updates
- release-note or migration-note updates when the wire contract changes

The first P0 schemas should define:

- system info response
- config response
- event list response
- connector resource
- connector list response
- capability resource
- capability list response
- auth pairing resource
- auth access token resource
- decision resource
- llm dispatch resource
- llm dispatch list response
- approval resource
- approval list response
- approval decision response
- session resource
- session list response
- run resource
- run list response
- step resource
- step list response
- tool call resource
- tool call list response
- create run request
- create connector request
- create capability request
- start pairing request
- start pairing response
- complete pairing request
- complete pairing response
- create llm dispatch request
- request approval request
- resolve approval request
- create step request
- update step status request
- report connector health request
- report connector failure request
- report capability health request
- report capability failure request
- create tool call request
- complete tool call request
- fail tool call request

Operator shell schemas should also stay in sync with implementation and fixtures:

- operator onboarding response
- operator readiness item
- operator first useful action
- operator activity record
- operator activity list response
- operator diagnostic finding
- operator diagnostic list response

Hosted activation schemas are additive Roadmap 45 contracts. They must remain
metadata-only for test chat evidence and stay covered by daemon contract fixtures:

- activation state resource
- activation response
- activation test chat request
- activation test chat response
- activation diagnostic list response

Evaluation and replay schemas should stay in sync with daemon-owned evaluation resources,
SDK types, web-shell behavior, and contract fixtures:

- replay candidate resource
- replay candidate list response
- create replay candidate request
- create replay attempt request
- replay attempt resource
- replay attempt list response
- create replay comparison request
- replay comparison resource
- replay comparison list response
- replay drift finding
- replay fixture resource
- replay fixture list response

Evaluation routes are additive under `/v1/evaluation/*`. Replay attempts default to
`non_live`; comparison terminal status is separate from replay execution status; fixture
resources describe repo-managed fixture provenance and must not imply browser-side
fixture editing support.
