# Configuration

The daemon loads configuration from three layers, in increasing
precedence:

1. Built-in environment-aware defaults
2. `<data_dir>/config.json`
3. `KURA_*` environment variables

Test env (`KURA_ENV=test`) targets `~/.kura-test` and `127.0.0.1:19192`;
production targets `~/.kura` and `127.0.0.1:19191`.

## config.json shape

```json
{
  "logLevel": "info",
  "llm": {
    "defaultProvider": "claude_code_cli",
    "defaultModel": "",
    "defaultTimeoutMs": 30000,
    "openaiCompatible": {
      "baseURL": "https://api.example.com/v1",
      "apiKeyEnv": "MY_API_KEY",
      "model": "some-model"
    },
    "claude": { "cliPath": "", "defaultModel": "", "workDir": "~" },
    "codex":  { "cliPath": "", "defaultModel": "", "workDir": "~" }
  },
  "connectors": {
    "discord":  { "enabled": false, "botTokenEnv": "DISCORD_BOT_TOKEN" },
    "telegram": { "enabled": false, "botTokenEnv": "TELEGRAM_BOT_TOKEN" },
    "slack":    { "enabled": false },
    "matrix":   { "enabled": false }
  }
}
```

## LLM providers

- **echo** — deterministic in-process fallback; always registered, so the
  daemon works with zero configuration.
- **claude_code_cli / codex_cli** — managed CLI providers: the daemon
  drives the locally installed CLI (`cliPath`, `workDir`).
- **openai_compatible** — any OpenAI-compatible HTTP endpoint
  (`baseURL`, `apiKey`/`apiKeyEnv`, `model`, stream timeouts).

`llm.defaultProvider` picks the default; per-request overrides are
accepted on the chat APIs.

## Secrets

Prefer the `*Env` fields (e.g. `botTokenEnv`) so credentials come from the
environment instead of being written into config files. Tenant-scoped
secrets live behind the daemon's secrets plane (`tenant-secret-values/`
under the data dir) and are referenced by `secretRef` where supported
(e.g. `slack.botTokenSecretRef`).

## The plugin profile

Runtime composition is configured separately in `<data_dir>/plugins.json`
— which plugins are enabled and their per-plugin config. See **Plugins**.

## Useful environment variables

| Variable | Effect |
|----------|--------|
| `KURA_ENV` | `test` / `prod` environment selection |
| `KURA_CONNECTORS_DISCORD_ENABLED` | opt into the live Discord connector |
| `KURA_*` | every config field has an env override |
