# Matrix Channel Loop

Matrix support is scoped to tenant-provided bot accounts on tenant-selected homeservers.
Phase 52 does not operate a shared Matrix homeserver, provision Matrix accounts, or route
WhatsApp as a fallback.

## Setup And Readiness

Operators configure a Matrix connector with a homeserver URL, homeserver id, bot user id,
bot access token, and at least one explicit route policy entry. The hosted setup state is
ready only when bot credentials are valid, the homeserver capability check passes, Matrix
connector conformance passes, and the route policy has an allowed direct user or selected
room. Missing credentials, invalid or revoked bot tokens, unsupported homeservers, network
failures, redaction suppression, and missing route policies stay terminal and
operator-visible.

DopeAgent does not host a Matrix homeserver, create Matrix accounts, manage E2EE key
sessions, or bridge WhatsApp. If an operator needs one of those surfaces, rollback is to
disable Matrix ingress and delivery eligibility while retaining setup, diagnostic, smoke,
and route evidence for support review.

## Routing And Replies

Accepted Matrix ingress is limited to unencrypted text in explicit direct allowments or
selected rooms. Room messages require a bot mention or configured command. Durable
identity and duplicate suppression use tenant id, connector id, homeserver id,
conversation id, and Matrix event id; sync batch and transaction ids are retained only as
redacted evidence.

Encrypted rooms, undecryptable events, media, calls, voice, reactions, bridge metadata,
thinking visibility, and incremental visible updates are unsupported for phase 52. These
surfaces produce explicit unsupported or ignored outcomes rather than weakening routing
or reply invariants.

Foreground replies are final-only Matrix replies. Connector-backed background delivery
can reuse the Matrix sender, but it records a separate delivery boundary from foreground
reply truth and requires the Matrix setup to be ready and delivery eligible.

## Diagnostics And Smoke

Matrix diagnostics map bot auth, room permission, ownership, homeserver, federation,
rate-limit, provider, network, blocked route, duplicate event, unsupported event, and
reply failures into the shared connector reason-code vocabulary. Support inspection is
redacted and freshness-bound; inspection evidence older than the current support window
is stale.

Live Matrix smoke is optional and must use safe tenant-provided credentials. When safe
live authorization is unavailable, release review records a structured skip with owner,
reason, remaining risk, validation timestamp, retention expiry, and redaction status.
