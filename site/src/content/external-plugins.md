# External Plugins

External (tier-2) plugins are **separate processes in any language** that
attach to the same hook waterfall and seams as builtin plugins. Install
one by dropping a directory under `<data_dir>/plugins/`:

```
~/.dope-test/plugins/
  my-plugin/
    manifest.json
    run.py            # anything executable
```

## manifest.json

```json
{
  "id": "my-plugin",
  "version": "0.1.0",
  "summary": "vetoes risky turns and serves embeddings",
  "requires": ["chat"],
  "hooks": [
    { "point": "chat/pre-dispatch", "onError": "veto" },
    { "point": "chat/turn-end",     "onError": "continue" }
  ],
  "seams": ["context.embedder"],
  "entry": {
    "kind": "process",
    "command": "python3",
    "args": ["run.py"],
    "timeoutMs": 2000
  }
}
```

- Malformed third-party manifests **never brick the boot** — they become
  warnings in the assembly report and the plugin is skipped.
- External plugins resolve through the same profile/`requires` machinery
  as builtins and show up in `/v1/plugins` as `source: "external"`.
  Duplicate ids lose to builtins.
- `onError` is per hook: `continue` (availability first — failures log
  and the turn proceeds) or `veto` (fail closed — for policy plugins).

## The process protocol

Line-delimited JSON over stdio. The daemon writes one request per line;
your process answers one line:

```jsonc
// request
{"point": "chat/pre-dispatch", "payload": {"messages": [...], "query": "..."}}

// response — payload (optional) REPLACES the hook payload
{"outcome": "continue", "payload": {"messages": [...]}}

// or veto
{"outcome": "halt", "reason": "tenant policy forbids this"}
```

The child is spawned lazily on the first call, respawned once per call if
it died, bounded by `timeoutMs` per call, and killed on daemon shutdown.

## Serving a seam

Declare `"seams": ["context.embedder"]` and answer seam calls on the same
channel — the point is `seam:<name>:<op>`:

```jsonc
// request
{"point": "seam:context.embedder:embed", "payload": {"text": "..."}}
// response
{"outcome": "continue", "payload": {"vector": [0.12, -0.4, ...]}}
```

With that, your process **takes over the vector ranker** for the context
plugin and `/v1/retrieval/queries` — this is how a neural embedding model
plugs in without any daemon change. Failures fall back to the built-in
deterministic embedder.

> Trust note: installing an external plugin is code execution with daemon
> privileges — the same trust class as installing a capability. The
> catalog (`kind=plugin`) carries trust tiers for distribution.
