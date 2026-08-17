# Skills & Self-Improvement

Both follow one product rule: **the agent proposes, the operator
governs**. No self-directed behavior outside the proposal/approval loop.

## Agent-managed skills

A skill proposal *is* a governed memory asset (`kind: skill`):

1. **Propose** — `POST /v1/skills/proposals` with name, description, body,
   and **motivating evidence links** (required — the validator refuses
   evidence-free proposals). Agent authorship is forced into `Pending` by
   the write policy, landing in the standard memory review queue.
2. **Review** — the operator approves or rejects through the existing
   memory review (`/v1/memory/assets/{id}/approve`).
3. **Publish** — `POST /v1/skills/proposals/{id}/publish` (approved
   proposals only; anything else is refused). Publication writes the
   `SKILL.md` bundle into `<data_dir>/skills/`, registers a Community
   catalog item whose version source permanently records
   `memory:<assetId>` (the provenance chain), and reloads the registry.

The runtime guard is structural: the skills registry only scans the
skills directory, so pending or rejected proposals are **never loadable**.

## Audited self-improvement

The closed loop the operator can audit and veto. Current target class:
plugin-profile configuration values (e.g. session budgets, context
budgets).

```bash
POST /v1/improvement/proposals
{
  "targetPlugin": "session-strategy",
  "configKey": "personalBudgetChars",
  "currentValue": 48000,
  "proposedValue": 64000,
  "predictedEffect": "fewer elisions in long sessions",
  "evidenceLinks": [{ "kind": "event", "id": "evt_ctx_123" }],
  "proposedBy": "agent"
}
```

Hard rules, enforced in code:

- **Evidence required** — proposals without motivating evidence are
  refused.
- **Rate-bounded** — max proposals per target per 24h window; the bound is
  operator configuration (`self-improve` plugin config), deliberately not
  agent-adjustable.
- **No change without a rollback path** — apply snapshots the *full prior
  profile* into the proposal before atomically rewriting `plugins.json`;
  `POST …/{id}/rollback` restores it exactly.
- **Full audit chain** — `improvement.proposed → applied → kept/rolled_back`
  are events; proposals persist as white-box JSON under
  `<data_dir>/improvement/` and survive restarts.

Follow-up-evaluation with regression-triggered automatic rollback is the
next slice on the roadmap.
