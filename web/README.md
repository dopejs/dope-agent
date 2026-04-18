# Web Client

This package is the operator-facing Web chat surface for DopeAgent.

Role:

- client only
- consumes daemon chat APIs through `@dope/client`
- does not own runtime truth
- does not assume daemon-side multi-turn memory

Operator flow:

- configure daemon URL
- provide access token
- optionally override provider and model
- send one single-turn query
- receive a non-stream or stream reply
