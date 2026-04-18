# TUI Client

This package is the terminal operator client for DopeAgent.

Current role:

- daemon client only
- consumes daemon chat APIs through `@dope/client`
- does not import daemon internals directly
- stays single-turn and stateless

Supported inputs:

- `--daemon-url`
- `--token`
- `--provider`
- `--model`
- `--query`
- `--stream`

Environment fallbacks:

- `DOPE_DAEMON_URL`
- `DOPE_ACCESS_TOKEN`
- `DOPE_CHAT_PROVIDER`
- `DOPE_CHAT_MODEL`
