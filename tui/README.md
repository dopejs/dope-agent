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
- `--threads`
- `--thread <id>`
- `--thread-trace <id>`
- `--thread-reset <id>`
- `--thread-archive <id>`
- `--thread-reopen <id>`

Environment fallbacks:

- `DOPE_DAEMON_URL`
- `DOPE_ACCESS_TOKEN`
- `DOPE_CHAT_PROVIDER`
- `DOPE_CHAT_MODEL`

Thread lifecycle commands require the daemon to authorize the current token and tenant
for each request. `--threads` lists lifecycle metadata, `--thread` inspects one thread,
and `--thread-trace` prints source-to-runtime evidence with redaction and retention
metadata. The output is inspection metadata only; it is not assistant memory, semantic
summary, context packing, or autonomous pruning input.
