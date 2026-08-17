# Getting Started

DopeAgent is a **personal agent OS**: a local Rust daemon (the control
plane) plus thin clients — a full-screen terminal UI (`dope-tui`), a React
web shell, and chat-channel connectors. The daemon owns runtime state,
provider dispatch, policy gates, memory, and event fan-out; clients are
thin consumers of its HTTP API.

## Install from a release

Grab the latest release from
[GitHub Releases](https://github.com/dopejs/dope-agent/releases). Each
release ships prebuilt tarballs for macOS and Linux (arm64 + x86_64) with
a `SHA256SUMS` file:

```bash
# Example: Apple Silicon macOS
curl -LO https://github.com/dopejs/dope-agent/releases/latest/download/dope-0.1.0-aarch64-apple-darwin.tar.gz
tar xzf dope-0.1.0-aarch64-apple-darwin.tar.gz
# binaries: dope (daemon) and dope-tui (terminal client)
```

Or use the install script from a checkout:

```bash
./scripts/install.sh
```

## Build from source

Requirements: Rust (1.85+), pnpm (for the web client).

```bash
git clone https://github.com/dopejs/dope-agent
cd dope-agent

# Daemon + TUI
make daemon-build                 # cargo build --release -p dope-cli
cd crates && cargo build --release -p dope-tui

# Web client + SDK
pnpm install
pnpm build:clients
```

## Run the daemon

DopeAgent has two environments; **test is the safe default**:

| Mode | Data dir | Bind address | Command |
|------|----------|--------------|---------|
| test | `~/.dope-test` | `127.0.0.1:19192` | `make daemon-run-test` |
| prod | `~/.dope` | `127.0.0.1:19191` | `make daemon-run-prod` |

```bash
make daemon-run-test              # start in the test environment
make daemon-test-status           # health check (GET /healthz)
```

Live connectors (Discord etc.) are **disabled by default** in the test
environment. Opt in explicitly:

```bash
make daemon-run-test-live         # test env with Discord enabled
```

## First conversation

The daemon always ships a deterministic `echo` provider, so you can talk
to it before configuring any model:

```bash
curl -s http://127.0.0.1:19192/v1/chat/query \
  -H 'content-type: application/json' \
  -d '{"query": "hello", "provider": "echo"}'
```

Then configure a real provider (Claude CLI, Codex CLI, or any
OpenAI-compatible endpoint) — see **Configuration**.

## Terminal UI

```bash
dope-tui                          # full-screen Claude-Code-style client
```

The TUI includes a live daemon event stream viewer (`/events`).
