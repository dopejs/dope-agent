# SDK Tenant Contract

## Goal

The TypeScript SDK is the only tenant transport abstraction used by browser and automated
callers. Callers can configure a default tenant and override the tenant for one request
without constructing `X-Kura-Tenant-ID` manually.

## Public Types

```ts
export type TenantRequestOptions = {
  tenantId?: string;
};

export type KuraClientOptions = {
  baseURL: string;
  accessToken?: string;
  fetchImpl?: typeof fetch;
  defaultTenantId?: string;
};
```

Tenant-related resource types must be exported or reused from the daemon schema-backed
contracts:

- `TenantResource`
- `TenantContextResource`
- `MembershipResource`
- `PrincipalResource`
- `TenantPermission`
- `TokenTenantGrantResource`
- `TenantDenialResource` or equivalent stable denial metadata

## Header Propagation

- If `TenantRequestOptions.tenantId` is present and non-empty, the SDK sends
  `X-Kura-Tenant-ID` with that value for that request.
- Otherwise, if `KuraClientOptions.defaultTenantId` is present and non-empty, the SDK sends
  `X-Kura-Tenant-ID` with the default tenant id.
- Otherwise, the SDK omits `X-Kura-Tenant-ID` and preserves server-resolved default tenant
  behavior.
- Stream and non-stream request helpers follow the same rule.
- SDK callers must not need to pass raw tenant headers.

## Per-Request Override

Existing public methods remain backward-compatible. Tenant-scoped methods accept an
optional trailing `TenantRequestOptions` argument.

Examples:

```ts
const client = createKuraClient({
  baseURL: "http://127.0.0.1:19192",
  accessToken: token,
  defaultTenantId: "ten_personal"
});

await client.getActivity({ limit: 20 });
await client.getActivity({ limit: 20 }, { tenantId: "ten_org" });
await client.listApprovals("pending", { tenantId: "ten_org" });
await client.resolveApproval("approval_1", { resolution: "approved" }, { tenantId: "ten_org" });
```

Acceptance rules:

- Override affects exactly one request.
- Override does not mutate `defaultTenantId`.
- Later calls without an override return to the configured default tenant.

## Tenant And Membership Helpers

The SDK exposes wrappers for the existing daemon tenant identity surfaces:

- `getMe(options?)`
- `listTenants(query?, options?)`
- `getTenant(tenantId, options?)`
- `listMemberships(tenantId, query?, options?)`
- `updateMembershipRole(tenantId, membershipId, input, options?)`
- `removeMembership(tenantId, membershipId, options?)` if the shell exposes removal

Permission-gated methods use the caller's resolved tenant context and stable daemon
authorization denials.

## Stable Denial Mapping

`KuraClientError` remains the public error class. For tenant authorization denials, it
must expose stable metadata:

```ts
class KuraClientError extends Error {
  readonly status: number;
  readonly code?: string;
  readonly tenantDenied?: boolean;
  readonly denial?: TenantDenialResource;
}
```

Acceptance rules:

- Tenant denials can be detected without parsing raw message text.
- Denial metadata does not reveal whether an inaccessible tenant exists.
- Non-tenant failures keep existing error behavior.

## Verification

SDK tests must cover:

- default tenant header propagation
- per-request override header propagation
- override does not mutate the default tenant
- omitted tenant preserves server default resolution
- stream requests receive tenant header behavior
- tenant denial maps to stable error metadata
- tenant/membership helper URLs and methods
