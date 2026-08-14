# Rust Migration Ledger — Go daemon → rs/ workspace

Strategy: incremental, depth-first, production-grade. The Go daemon stays the
control plane of record until a Rust surface reaches parity and is verified.
No demo-grade ports: a module counts as migrated only when its types,
behavior, and meaningful tests are ported and `cargo test -p <crate>` passes.

## Conventions (binding for every ported crate)

- Go package `internal/foo` → crate `dope-foo` at `rs/foo`. Folded packages:
  `tenantctx` and `auth` live inside `dope-identity`; `imtypes` inside `dope-im`.
- Types: `serde` with `#[serde(rename_all = "camelCase")]` to match Go JSON
  tags. Shared IDs (`TenantId`, `ThreadId`, `RunId`, `ProfileId`) come from
  `dope-protocol`; add new UUIDv7 IDs there via `define_id!`.
- Errors: `thiserror` enums per crate. No `unwrap`/`expect` outside tests.
- Async: tokio; object-safe traits use boxed futures (`Pin<Box<dyn Future ..>>`).
  Async locks: `tokio::sync`; sync locks: `parking_lot` (no `.lock().unwrap()`).
- Time: `chrono::DateTime<Utc>`. SQLite: `rusqlite` (bundled). HTTP client:
  `reqwest` (rustls). HTTP server (wave 6): `axum`.
- Every `Cargo.toml` uses workspace deps; crates may edit only their own
  crate directory. Shared workspace deps are owned by the integrator.
- Tests: port the Go package's behavioral tests (validation rules, state
  transitions, isolation, error paths). Skip pure plumbing tests.
- No type duplication across crates: if a needed type lives in an unported
  crate, STOP and report the missing dependency in the result instead of
  copying the type.

## Architecture decisions

- **Persistence inversion**: Go's `internal/store` converges on every domain
  package (typed DAOs). In Rust, `dope-store` owns the SQLite connection,
  the migration runner, and all `CREATE TABLE` migrations (self-contained
  SQL), and depends on domain crates — never the reverse. Domain crates stay
  persistence-free; services receive DAO handles. DAO modules land
  incrementally as their domain crates arrive.
- **Subpackage parity**: Go subpackages (e.g. `integrations/adapterrpc`) map
  to Rust modules or standalone crates per the topo order, not collapsed.

## Wave plan and status

Sequencing follows the verified 18-layer topological order (no cycles) from
the daemon's non-test import graph. LOC = non-test lines.

| Wave | Packages (crate) | Status |
|---|---|---|
| 1 leaves | config(997), identity(1128, +tenantctx 61 +auth 413), telemetry(42), billing(2150), bindings(1126), llm(1083), profiles(688), threads(1327), router(220) | done (187 tests green) |
| 2 base | secrets(1037), contracts(451), activation(1263), capabilities(344), adapterrpc(551), adapterprovider(155), adapterref(157), inventory(254), setupwizard(1753), imtypes(108), livevalidation(1977), providers(1079) | done (12/12) |
| 3 integration-base | connectors-root(1543), integrations(1674), calendar(1665), mail(2468), opsreadiness(1939), policy(259), feishulark(1457), runtime(1120) | in progress (integrations core+diagnostics types+classifier, policy, calendar done; mail + opsreadiness + feishulark + runtime pending) |
| 4 runtime-services | computeruse(1239), artifacts(158), orchestration(869), evaluation(3396), events(2185) | pending |
| 5 store | audit(558), dope-store core+domains(28.6k, split), migrationfixture(1513), tenancy(2626), checkpoints(107), delivery(1817), managerdoc(66), sandbox(3419) | pending |
| 6 managers | catalog(421), evidence(322), execprofile(394), managedproviders(1697), mcp(5351, split), scheduler(1485), skills(785), triage(417), webhook(447), chat(1119), reminders(1326), routine(399) | pending |
| 7 channels | im(1559), connectors/discord(1517), connectors/matrix(1896), connectors/slack(1583), connectors/telegram(1449) | pending |
| 8 surface | api(26.3k, split by route family), app(2401), daemon binary wiring, contract-test parity | pending |

Per-module status is updated as waves land. Source of truth for sizes and
dependency edges: `daemon/internal/<pkg>` (LOC measured 2026-08-13).
