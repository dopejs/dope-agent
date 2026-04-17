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
