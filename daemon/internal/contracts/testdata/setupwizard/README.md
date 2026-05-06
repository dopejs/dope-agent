# Setup Wizard Contract Fixtures

Roadmap 46 setup wizard fixtures cover hosted credential setup and OAuth setup state.
Response, diagnostic, error, audit, SDK, and web fixtures must remain metadata-only:
raw submitted secret values, OAuth authorization codes, access tokens, refresh tokens,
callback payloads, authorization headers, and derived credential material do not belong
in fixture output.

Inbound mutation request fixtures may include placeholder values only where the API
contract explicitly accepts credential material, such as `setup-secret-submit.request`.
Those values must not be copied into response, diagnostic, audit, SDK, web, log, or
operator diagnostic fixtures.
