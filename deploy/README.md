# Deploy & Install

How to install and run Kura for real use. Kura is a **daemon** (the
control plane) plus thin clients (Web UI, TUI). Installing means getting the
daemon running as a background service; clients then connect to it.

> Status: there is no prebuilt-binary release channel yet, so every path below
> **builds the daemon from source** (Rust release build).
> Once CI publishes release artifacts, `get.sh` and the install script should be
> updated to download a verified binary and skip the toolchain step.

## Prerequisites

| Path | Needs |
|------|-------|
| Native (macOS / Linux) | Rust toolchain (cargo), `git`, `curl` |
| Docker | Docker 24+ (no Rust needed — built inside the image) |

The daemon is the only thing that must be installed. The clients are optional
and built separately with `pnpm` (see [Connecting a client](#connecting-a-client)).

---

## Option A — Native service (recommended for self-host)

One installer handles both OSes and registers a background service that starts
on boot: **launchd** on macOS, **systemd** on Linux.

```bash
git clone https://github.com/dopejs/kura.git
cd kura
./deploy/install.sh
```

This builds `kura`, installs it to `~/.local/bin/kura` (with a legacy `dope`
symlink), creates the data dir
`~/.dope`, registers the service, starts it, and waits for `/healthz`.

Defaults (override with flags or env):

| What | Default | Override |
|------|---------|----------|
| Environment | `prod` | `--env test` |
| Data dir | `~/.dope` (`~/.dope-test` for test) | `--data-dir <d>` |
| Bind address | `127.0.0.1:19191` (`:19192` for test) | `--bind <host:port>` |
| Binary dir | `~/.local/bin` | `--bin-dir <d>` |
| Linux service scope | user (`systemctl --user`) | `--system` (system-wide, sudo) |

Useful variants:

```bash
./deploy/install.sh --env test          # parallel test instance on :19192
./deploy/install.sh --no-service        # build + init only; run it yourself
./deploy/install.sh --system            # Linux: boot-time system service (sudo)
```

### One-line bootstrap

```bash
curl -fsSL https://raw.githubusercontent.com/dopejs/kura/main/deploy/get.sh | bash
# pass through env/flags:
curl -fsSL .../get.sh | DOPE_ENV=test bash
curl -fsSL .../get.sh | bash -s -- --no-service
```

`get.sh` clones into `~/.cache/dope-agent/src` and runs `install.sh`.

### Manage the service

macOS (launchd):
```bash
launchctl print gui/$(id -u)/com.dopejs.dope-agent | grep state
tail -f ~/.dope/logs/daemon.err.log
launchctl bootout gui/$(id -u)/com.dopejs.dope-agent   # stop
```

Linux (systemd user):
```bash
systemctl --user status dope-agent
journalctl --user -u dope-agent -f
systemctl --user restart dope-agent
```

Upgrade = pull + re-run (atomic binary swap; restart picks it up):
```bash
git pull && ./deploy/install.sh
```

Uninstall (keeps your data unless `--purge`):
```bash
./deploy/uninstall.sh            # remove service + binary
./deploy/uninstall.sh --purge    # also delete ~/.dope (irreversible, confirms)
```

---

## Option B — Docker

```bash
cd deploy/docker
docker compose up -d --build
docker compose ps          # wait for STATUS = (healthy)
docker compose logs -f
```

Data persists in the `dope-data` named volume. The compose file binds the host
port to **loopback** (`127.0.0.1:19191`) so it is not network-reachable by
default. To expose it, change the port mapping to `0.0.0.0:19191` **and** put
authentication / a reverse proxy in front — see [Security](#security).

Secrets (LLM keys, connector tokens) go in `deploy/docker/.env` (gitignored) as
`DOPE_*` variables, then uncomment `env_file` in `docker-compose.yml`.

Plain `docker` without compose:
```bash
docker build -f deploy/docker/Dockerfile -t dope-agent .   # run from repo root
docker run -d --name dope-agent -p 127.0.0.1:19191:19191 -v dope-data:/data dope-agent
```

---

## Connecting a client

Clients are built from source and default to the **test** port `19192`, so for a
`prod` daemon you must point them at `:19191`.

```bash
pnpm install
pnpm build:clients

# TUI (Rust; build once from crates/):
cd crates && cargo build --release -p dope-tui
DOPE_DAEMON_URL=http://127.0.0.1:19191 ./target/release/kura-tui
# Web (dev server; set the daemon URL inside the UI):
pnpm dev:web
```

---

## Configuration & secrets

On first start the daemon initializes its data dir and `config.json`. The
supported knobs are environment variables read at startup (`DOPE_*`). Set them
in the service definition:

- **Native:** edit the `Environment`/`EnvironmentVariables` entries in the
  generated unit (`~/.config/systemd/user/dope-agent.service` or
  `~/Library/LaunchAgents/com.dopejs.dope-agent.plist`), then restart.
- **Docker:** put them in `deploy/docker/.env`.

Common ones:

| Variable | Purpose |
|----------|---------|
| `DOPE_LLM_DEFAULT_PROVIDER` / `DOPE_LLM_DEFAULT_MODEL` | default LLM routing |
| `DOPE_LLM_OPENAI_COMPATIBLE_BASE_URL` / `_API_KEY` | OpenAI-compatible provider |
| `DOPE_CONNECTORS_DISCORD_ENABLED` / `_BOT_TOKEN` | Discord connector (off by default) |
| `DOPE_LOG_LEVEL` | `debug` / `info` / `warn` |

(Full list: search `DOPE_` in `crates/foundation/config`.)

---

## Security

- The daemon **binds loopback by default** (`127.0.0.1`). It has no built-in
  network authentication, so do **not** expose it directly to the internet.
- To reach it from other machines, bind `0.0.0.0` only behind a reverse proxy
  (TLS + auth) or an SSH tunnel / private network.
- Live connectors (Discord, etc.) are **disabled by default**; enable them
  explicitly once you have set the corresponding token.

---

## Files in this directory

| Path | Purpose |
|------|---------|
| `install.sh` | build + install + register service (macOS/Linux) |
| `uninstall.sh` | remove service + binary (data kept unless `--purge`) |
| `get.sh` | `curl \| bash` bootstrap → clone + `install.sh` |
| `launchd/…plist.template` | macOS launchd user agent (tokens filled by installer) |
| `systemd/…service.template` | systemd unit, user or system scope |
| `docker/Dockerfile` | multi-stage static build → Alpine runtime |
| `docker/docker-compose.yml` | one-command container deployment |
