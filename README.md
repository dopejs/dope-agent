# Kura

A personal agent OS: a local daemon (control plane) plus thin clients — a React
web UI, a full-screen Rust TUI, and chat-channel connectors.

## Architecture

| Path | Role |
|------|------|
| `crates/` | Rust workspace — the daemon control plane (runtime, LLM providers, channels/connectors, store, events, HTTP API, harness). User binary: `kura` (Cargo package: `kura-cli`); HTTP API: `kura-api`. |
| `web/` | React 19 + Vite web UI. |
| `crates/surface/tui/` | Rust full-screen terminal client (`kura-tui`; Cargo package: `kura-tui`). |
| `sdk/ts/` | TypeScript client SDK (`@kura/client`). |
| `schemas/` | JSON Schema contracts (API, events, config) — source of truth for cross-language contracts. |
| `docs/` | Planning and design docs, organized by module. |

The Go `daemon/` control plane was fully replaced by the Rust workspace and
removed; see `crates/MIGRATION.md` for the migration record.

## Build & Test

Daemon (Rust, from `crates/`):

```bash
make daemon-build              # emits crates/target/release/kura
make daemon-test               # cargo test --workspace
make daemon-contract-test      # cargo test -p kura-contracts
```

TUI (Rust):

```bash
cd crates && cargo build -p kura-tui  # emits target/debug/kura-tui
```

Clients (TypeScript):

```bash
pnpm build:clients             # build SDK + web
pnpm test:clients              # build + test SDK + web
```

## Run

```bash
make daemon-run-test           # test env: ~/.kura-test, 127.0.0.1:19192
make daemon-run-test-live      # test env with Discord enabled
make daemon-run-prod           # prod env: ~/.kura, 127.0.0.1:19191
make daemon-test-status        # health check
```

For local debugging, use the project skill at `.agents/skills/kura-test-env/SKILL.md`.
`make daemon-run-test` is the safe default and keeps Discord disabled unless you
opt in with `make daemon-run-test-live` or `KURA_CONNECTORS_DISCORD_ENABLED=true`.

## Model Roles

Models are routed by **modality**, not by performance tier:

| Role | Modality |
|------|----------|
| `primary` | Text reasoning and tool use |
| `vision` | Image understanding |
| `image` | Image generation |
| `video` | Video generation |
| `embed` | Vector embeddings |

```
GET    /v1/model-roles          # every role, including unrouted ones
PUT    /v1/model-roles/{role}   # {"providerId": "...", "model": "..."}
DELETE /v1/model-roles/{role}   # unroute, so the configured default applies
```

Routing lives in the runtime rather than inside each tool. A tool that needs a
picture asks for the `image` role instead of carrying its own provider, API key
and retry policy, so credentials, usage accounting and provider swaps stay in
one place.

Resolution order per role: a stored assignment (`PUT`), then daemon config
(`llm.roles` or `KURA_LLM_ROLE_<ROLE>=provider[:model]`), and finally — for
`primary` only — `llm.defaultProvider`/`defaultModel`. Every other role stays
**unrouted** when unconfigured, and `routed: false` means *capability
unavailable*: callers must not fall back to the default provider, or a text
model ends up being asked for a picture.

Assigning a role records which provider serves it. Execution goes through
`GenerationProvider`, a separate trait from the streaming `ModelProvider`
because image and video calls are request/poll shaped rather than token
streams:

- `generate()` returns `Ready` for synchronous providers (typical for images)
  or `Pending { job_id }` for queued ones (typical for video).
- `poll()` has a default that reports an unknown job, so a synchronous provider
  cannot accidentally imply a generation finished.
- A finished asset is either inline `Bytes` or a `Url`. The difference is kept
  rather than normalized: inline data costs memory, a URL can expire before the
  caller fetches it, and the caller has to know which it holds.

`OpenAiCompatibleImageClient` implements it over `/images/generations`. Video
has the trait but no concrete client yet.

## Provider Listing

`GET /v1/providers?include=models` embeds each provider's models keyed by
provider id, so a client rendering a provider list does not need one request
per provider. Without the parameter the response is unchanged. Unknown
`include` values are ignored rather than rejected, so a newer client does not
break against an older daemon.

## Provider Request Tuning

`llm.openaiCompatible` accepts two additions:

- `headers` — extra HTTP headers for every request to that provider, for
  gateways that route on one. `Authorization` is reserved for the API key and
  is dropped from this map, so a header cannot silently replace the credential.
- `sampling.temperature` / `sampling.topP` — omitted from the request body when
  unset, because some OpenAI-compatible gateways reject an explicit
  `temperature` on reasoning models. "Unset" and "set to zero" stay distinct.

## Local Environment Modes

Development defaults to the **test** environment:

- `KURA_ENV=test`
- data dir: `~/.kura-test`
- config file: `~/.kura-test/config.json`
- bind addr: `127.0.0.1:19192`

Production is explicit:

- `KURA_ENV=prod`
- data dir: `~/.kura`
- config file: `~/.kura/config.json`
- bind addr: `127.0.0.1:19191`

Never touch prod config or live connectors without explicit intent; live
connectors are disabled by default in the test environment.

## Embedded Mode

`KURA_ENV=embedded` is for host applications that supervise their own daemon —
one instance per workspace, with the host owning the process lifecycle:

- `KURA_ENV=embedded`
- data dir: supplied by the host via `KURA_DATA_DIR`
- bind addr: supplied by the host via `KURA_BIND_ADDR`

Isolation matches the test environment: managed CLI provider homes live under
`<data dir>/managed-provider-home` rather than the user's real home, and hosted
billing quotas are not enforced. The defaults (`~/.kura-embedded`,
`127.0.0.1:19193`) exist only so a misconfigured host cannot collide with the
prod or test daemon — a host is expected to set both variables explicitly.

Authentication applies as everywhere else: `/v1` routes require a bearer token,
obtained through the unauthenticated local pairing exchange
(`POST /v1/auth/pairings/start` then `.../complete` with the returned code). In
`local` mode the code is returned by the start response, so a supervisor that
already owns the daemon process completes it without a human step.

What embedded adds is a bootstrapped identity. Pairing has no way to ask who is
pairing, so by default it issues a token with no principal, which the resolver
correctly refuses — leaving a daemon that authenticates and then denies every
request. On boot, embedded provisions a personal tenant (`ten_local`), a local
operator principal (`prn_local_operator`) and an owner membership, and pairing
stamps that identity onto issued tokens along with a tenant grant. All four
links are required: without the membership the operator resolves with no
permissions, and without the grant it fails resolution outright. Bootstrapping
is idempotent, so a restart reuses the workspace rather than resetting it.

Two things are deliberately *not* different from `test`:

- **`environmentScope`** stays `test`. It is a data-isolation label, not a
  deployment name, and every API schema constrains it to `test|prod`. Embedded
  is a non-production scope, so it shares the test scope and the persisted
  format is unchanged.
- **Secret scopes** resolve against `SecretEnvironmentScope::Test` for the same
  reason.

What *is* different is self-reporting: `/v1/config` and the system-info
response return `environment: "embedded"`, so a host daemon no longer has to
claim to be a developer test daemon. Prefer this over `KURA_ENV=test` when
embedding: the test environment carries no stability promise for hosts.

## Planning Docs

High-signal entry points:

- Product scope: `docs/product/product-outline.md`
- Runtime roadmap & tasks: `docs/runtime/daemon-roadmaps.md`, `docs/runtime/daemon-tasks.md`
- Provider architecture: `docs/providers/provider-architecture.md`
- Channel behavior: `docs/channels/channel-reply-progression.md`
- Harness architecture: `docs/harness/harness-architecture.md`
- Sandbox design: `docs/harness/sandbox-execution-plane.md`
- Test environment workflow: `docs/dev/test-environment-workflow.md`
- Module map: `docs/architecture/module-map.md`

## Working Assumptions

- OpenClaw is treated as a feature and product benchmark, not a required runtime base.
- Context engineering, memory, planning, handoff, and policy will be redesigned rather than lightly patched.
- Long-lived agent state must be observable, replayable, and safe to evolve.

## License

Kura is licensed under the [Apache License 2.0](LICENSE).
