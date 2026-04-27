# SDK

This directory contains client-facing libraries.

Current package:

- `sdk/ts`: shared TypeScript chat client used by both Web and TUI

Design rule:

- clients must share the same daemon contract layer
- Web and TUI must not each implement ad-hoc HTTP/SSE logic
- tenant-scoped callers should use SDK tenant options instead of constructing
  `X-Dope-Tenant-ID` headers directly

## Tenant-Aware TypeScript Client

The TypeScript client can carry tenant intent at the client level or for one
request:

```ts
const client = createDopeClient({
  baseURL: "http://127.0.0.1:19192",
  accessToken,
  defaultTenantId: "ten_personal"
});

await client.getActivity({ limit: 20 });
await client.getActivity({ limit: 20 }, { tenantId: "ten_org" });
await client.resolveApproval("approval_1", { resolution: "approved" }, { tenantId: "ten_org" });
```

When `defaultTenantId` and per-request `tenantId` are both omitted, the SDK does
not send a tenant header and preserves server-resolved default tenant behavior.
Per-request overrides apply only to that call and do not mutate the configured
default tenant.

Tenant identity and membership helpers are exposed through the same client:
`getMe`, `listTenants`, `getTenant`, `listMemberships`, `updateMembershipRole`,
and `removeMembership`. Tenant authorization failures surface through
`DopeClientError.tenantDenied` and stable `denial` metadata when the daemon
returns a tenant denial payload.

Reserved for generated or maintained client SDKs.

Planned targets:

- TypeScript
- Go
