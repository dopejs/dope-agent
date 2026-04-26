# Roadmap 35 US2 Test Fixtures

This directory holds reproducible SQLite fixtures used by the Roadmap 35 (Tenant-Scoped Data Migration) regression suite.

## `BuildPreTenantV21Fixture(dataDir)`

Builds a fresh SQLite database in `dataDir` initialized at schema version 21 — the last "pre-tenant" head before Roadmap 35 added `tenant_id` columns at v22+. Seeds at least one parent + one child row in every in-scope `tenant_owned` table so per-domain backfill drivers (`tenant_migration:backfill:*`) can be exercised end-to-end.

### Usage

```go
s, err := testdata.BuildPreTenantV21Fixture(t.TempDir())
require.NoError(t, err)
defer s.Close()

before, _ := testdata.CountSeededRows(ctx, s)
require.NoError(t, s.ApplyHeadMigrations(ctx))    // schema v22..v33 + register progress rows
// ... run backfill drivers from app.runRuntimeBackfills etc ...
after, _ := testdata.CountSeededRows(ctx, s)
require.Equal(t, before, after)                    // T078 assertion
```

### Why programmatic, not a binary blob?

A check-in `.sqlite` file would (a) drift silently from the live schema, (b) be unreviewable in PRs, (c) fail to track schema migrations as they evolve. The programmatic seeder is a code-reviewable surface that always rebuilds against the current `schemaMigrations` array.

### Seed coverage

- runtime spine: sessions, runs, steps, tool_calls, llm_dispatches, checkpoints
- schedules + targets + dispatch_attempts
- workflows + steps + dependencies + handoffs
- integrations + delivery (targets, preferences, outcomes, attempts, summary windows)
- calendar + mail (accounts → operations → artifacts)
- reminders + occurrences + actions
- computer-use sessions + actions + artifacts
- approvals + decisions
- evaluation replay candidates + attempts
- harness: consumer_policy_records, provider_preferences, mcp_tool_exposure_rules, secret_scope_bindings, sandbox_executions
- mcp servers + tools (parents for the exposure rules FK)
- connector_messages: one with session_id, one with run_id, one with neither (default-personal-tenant fallback case)
- events: tenant-owned (run_id), global (system), connector with proper resource pointer, connector legacy (reclassify to `connector_global`), capability-only (reclassify to `capability_global`)
