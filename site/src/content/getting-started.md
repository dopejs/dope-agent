# Getting Started

DopeAgent is a **personal agent OS**: a local Rust daemon (the control
plane) plus thin clients — a full-screen terminal UI (`dope-tui`), a React
web shell, and chat-channel connectors. The daemon owns runtime state,
provider dispatch, policy gates, memory, and event fan-out; clients are
thin consumers of its HTTP API.

## Install

One line (macOS and Linux, arm64 + x86_64):

```bash
curl -fsSL https://agent.dopejs.com/install.sh | sh
```

The installer detects your platform, downloads the latest
[GitHub Release](https://github.com/dopejs/dope-agent/releases), verifies
its SHA-256 against the release's `SHA256SUMS`, and installs `dope` (the
daemon) and `dope-tui` (the terminal client) into `~/.local/bin` or
`/usr/local/bin`. Pin a version or destination with:

```bash
DOPE_VERSION=v0.1.0 DOPE_INSTALL_DIR=~/bin sh -c "$(curl -fsSL https://agent.dopejs.com/install.sh)"
```

Prefer manual? Grab a tarball from the releases page:

```bash
curl -LO https://github.com/dopejs/dope-agent/releases/latest/download/dope-0.1.0-aarch64-apple-darwin.tar.gz
tar xzf dope-0.1.0-aarch64-apple-darwin.tar.gz
sudo install -m 755 dope-0.1.0-aarch64-apple-darwin/{dope,dope-tui} /usr/local/bin/
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

With an installed release binary, just run it:

```bash
dope                              # release build defaults to prod:
                                  # data dir ~/.dope, 127.0.0.1:19191
curl -s http://127.0.0.1:19191/healthz
```

DopeAgent has two environments, selected by `DOPE_ENV`:

| Mode | Data dir | Bind address | Command |
|------|----------|--------------|---------|
| prod (release default) | `~/.dope` | `127.0.0.1:19191` | `dope` |
| test | `~/.dope-test` | `127.0.0.1:19192` | `DOPE_ENV=test dope` |

From a source checkout, the Make targets wrap the same thing and default
to the **test** environment (the safe development default):

```bash
make daemon-run-test              # test env
make daemon-test-status           # health check (GET /healthz)
make daemon-run-test-live         # test env with Discord enabled
```

Live connectors (Discord etc.) are **disabled by default** everywhere
until you enable them in config.

## First conversation

The daemon always ships a deterministic `echo` provider, so you can talk
to it before configuring any model:

```bash
curl -s http://127.0.0.1:19191/v1/chat/query \
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
