# Quickstart: Tenant-Aware Operator Shell And SDK

## Preconditions

- Work from branch `022-tenant-aware-shell-sdk`.
- Use the default test daemon environment: `~/.dope-test` and `127.0.0.1:19192`.
- Do not use live connectors or production state for this roadmap.

## Implementation Order

1. Update SDK tenant types and request plumbing in `sdk/ts/src/index.ts`.
2. Add SDK tenant and membership helper methods.
3. Add SDK tests for default tenant, per-request override, stable denial mapping, stream
   tenant headers, and membership helper routes.
4. Update the shell bootstrap to load identity/allowed tenants before tenant-scoped
   projections.
5. Add active tenant state, tenant switcher, persisted selection revalidation, stale
   generation handling, and tenant-scoped event stream reopening in `web/src/app/App.tsx`.
6. Add membership inspection and role update UI for active tenant users with
   `tenant.manage`.
7. Add web tests for first load, switching, stale data, denied states, revocation, and
   membership management.
8. Fill any daemon/API contract gaps discovered by the UI/SDK work, especially last-owner
   protection and stable denial response behavior.

## Verification Commands

From the repository root:

```bash
pnpm --dir sdk/ts test
pnpm --dir web test
pnpm test:clients
pnpm build
make daemon-contract-test
```

From `daemon/` after any daemon-side changes:

```bash
go test ./...
go mod tidy
```

## Manual Smoke Check

1. Start the test daemon:

   ```bash
   make daemon-run-test
   ```

2. Open the web shell against `http://127.0.0.1:19192`.
3. Authenticate with a test access token.
4. Verify first load shows the personal tenant as active.
5. Create or use a test organization tenant and verify the switcher lists it only when
   allowed.
6. Switch tenants and confirm activity, diagnostics, approvals, onboarding, and evaluation
   projections clear or mark stale before refetching.
7. Open a detail pane, switch tenants, and confirm old detail data is cleared or marked
   stale.
8. Update a member role as an authorized user and verify the resulting role state is shown.
9. Attempt a last-owner downgrade/removal and verify the action is prevented.

Implementation notes:

- The shell stores only the last selected tenant id in browser local storage,
  keyed by daemon URL and principal id, and revalidates it against
  `GET /v1/tenants` before loading tenant-scoped projections.
- Active tenant denial clears shell projections and membership rows; the user
  must explicitly choose another allowed tenant before tenant-scoped actions are
  enabled again.
- Membership role updates rely on daemon-confirmed membership state and daemon
  audit records; the shell does not optimistically change roles.

## Rollback

Rollback is client-surface rollback only:

- remove or hide the tenant switcher and membership panel
- stop passing SDK tenant defaults/overrides
- keep existing access token and server-resolved default tenant behavior

No daemon storage rollback is required for this roadmap.
