# Quickstart: Tenant Identity And Access Foundation

## Prerequisites

- Work from branch `019-tenant-identity-access`.
- Use the test environment, not production state.
- Keep live connectors disabled unless a later verification step explicitly asks for them.

## Start The Test Daemon

```bash
make daemon-run-test
```

In another shell:

```bash
make daemon-test-status
```

Expected result: the daemon reports healthy on `127.0.0.1:19192` using the test data
directory.

## Pair A Local Token

Start pairing:

```bash
curl -sS -X POST http://127.0.0.1:19192/v1/auth/pairings/start \
  -H 'Content-Type: application/json' \
  -d '{"mode":"local","label":"tenant-foundation-quickstart"}'
```

Complete the pairing with the returned `pairingId` and `pairingCode`:

```bash
curl -sS -X POST http://127.0.0.1:19192/v1/auth/pairings/<pairingId>/complete \
  -H 'Content-Type: application/json' \
  -d '{"code":"<pairingCode>"}'
```

Save the returned `accessToken` in a local shell variable:

```bash
TOKEN='<accessToken>'
```

## Confirm Default Tenant Bootstrap

```bash
curl -sS http://127.0.0.1:19192/v1/auth/me \
  -H "Authorization: Bearer $TOKEN"
```

Expected result:

- response includes a principal
- response includes a default personal tenant
- response includes the current tenant
- response shows the token grant limited to the default personal tenant for bootstrapped
  local tokens

## List Allowed Tenants

```bash
curl -sS http://127.0.0.1:19192/v1/tenants \
  -H "Authorization: Bearer $TOKEN"
```

Expected result: the bootstrapped default personal tenant appears in the allowed tenant
list.

## Exercise Explicit Tenant Selection

Use the default personal tenant id returned by `/v1/auth/me`:

```bash
curl -sS http://127.0.0.1:19192/v1/config \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Kura-Tenant-ID: <defaultTenantId>"
```

Expected result: request succeeds and resolves to the selected tenant.

Use a made-up tenant id:

```bash
curl -sS -i http://127.0.0.1:19192/v1/config \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Kura-Tenant-ID: tenant_not_allowed"
```

Expected result: request is denied with stable tenant authorization error details and does
not reveal whether the tenant exists.

## Exercise Organization Membership

Create an organization tenant:

```bash
curl -sS -X POST http://127.0.0.1:19192/v1/tenants \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"tenantKind":"organization","displayName":"Quickstart Organization"}'
```

Invite another principal:

```bash
curl -sS -X POST http://127.0.0.1:19192/v1/tenants/<organizationTenantId>/invitations \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Kura-Tenant-ID: <organizationTenantId>" \
  -H 'Content-Type: application/json' \
  -d '{"invitedPrincipalId":"<principalId>","role":"operator"}'
```

Expected result: invitation and tenant audit events are recorded. The invited principal can
accept with `POST /v1/tenant-invitations/<invitationId>/accept`.

## Exercise Token Lifecycle

List tokens:

```bash
curl -sS http://127.0.0.1:19192/v1/auth/tokens \
  -H "Authorization: Bearer $TOKEN"
```

Issue a scoped automation token:

```bash
curl -sS -X POST http://127.0.0.1:19192/v1/auth/tokens \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"label":"quickstart automation","defaultTenantId":"<defaultTenantId>","allowedTenantIds":["<defaultTenantId>"]}'
```

Rotate a token:

```bash
curl -sS -X POST http://127.0.0.1:19192/v1/auth/tokens/<tokenId>/rotate \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"reason":"quickstart rotation"}'
```

Revoke the replacement token when it is no longer needed:

```bash
curl -sS -X POST http://127.0.0.1:19192/v1/auth/tokens/<tokenId>/revoke \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"reason":"quickstart revocation"}'
```

Expected result: the revoked token fails on the next protected request.

## Inspect Audit History

```bash
curl -sS 'http://127.0.0.1:19192/v1/tenant-audit-events?limit=20' \
  -H "Authorization: Bearer $TOKEN"
```

Expected result: tenant switching, denied access, membership changes, invitation
decisions, and token lifecycle changes are visible without raw token material or secrets.

## Automated Verification

```bash
(cd daemon && go test ./internal/identity ./internal/auth ./internal/api ./internal/store ./internal/contracts ./internal/app)
make daemon-contract-test
(cd daemon && go test ./...)
(cd daemon && go mod tidy)
```

Run client tests only if the implementation touches SDK, web, or TUI code:

```bash
pnpm test:clients
```
