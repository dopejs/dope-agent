# Research: Daemon-Owned Thread And Session Lifecycle

## Decision: Add A Thread Lifecycle Layer Over Existing Sessions

**Decision**: Add a focused daemon-owned thread lifecycle layer that composes existing
`router.SessionRouter`, SQLite `sessions`, connector messages, runtime runs, workflows,
approvals, delivery outcomes, and events instead of replacing the session router.

**Rationale**: Roadmap 54 requires product-visible thread lifecycle, source linkage, and
runtime projections. Existing session routing already handles direct/group route keys,
reset generation, and compatibility with `/v1/sessions`; replacing it would risk
breaking connector ingress and current clients. A thread layer can own lifecycle state
while preserving sessions as bounded continuity segments.

**Alternatives considered**:
- Rewrite `router.SessionRouter` into a thread store: rejected because it would create a
  large compatibility break across chat, connector ingress, tests, and existing session
  schemas.
- Keep lifecycle as session fields only: rejected because archive/reopen, source
  uniqueness, runtime projections, and legacy projection need thread-level truth across
  multiple session segments.
- Put lifecycle only in prompt files or client state: rejected by the upstream fixed
  decision that thread truth is structured daemon state.

## Decision: Reset Preserves Thread ID And Creates A New Session Segment

**Decision**: Reset keeps the same thread ID and starts a new active session segment.
Historical sessions, connector messages, runs, workflows, approvals, replies, deliveries,
and audit evidence remain attached as prior evidence.

**Rationale**: Users expect to find the same conversation after reset, while future work
needs a clean continuity boundary. A new session segment gives implementation and tests a
concrete boundary without deleting history or creating a surprising new thread identity.

**Alternatives considered**:
- Reset archives the old thread and creates a new thread: rejected because it makes
  traceability and user-visible conversation identity harder.
- Reset only increments session generation: rejected because it is too weak for
  multi-segment lifecycle projection and source-to-runtime tracing.

## Decision: Archive Blocks Future Continuation But Does Not Cancel Accepted Work

**Decision**: Archive prevents new continuation through users, connector ingress,
scheduled work, workflow-originated work, or retries, but it does not cancel active or
pending runs, approvals, workflows, replies, or deliveries already accepted before the
archive action.

**Rationale**: Archive is a conversation lifecycle control, not a runtime cancellation
mechanism. Cancelling already accepted work would cross into run/workflow/approval/
delivery ownership and could silently destroy operator evidence. Blocking future
continuation provides safe product behavior with low blast radius.

**Alternatives considered**:
- Cancel all linked work on archive: rejected because it conflates lifecycle visibility
  with runtime cancellation and risks data loss.
- Deny archive while work is active: rejected because users still need to stop future
  continuation during incidents or noisy channels.
- Wait until active work completes: rejected because it creates unclear intermediate
  states and operational timing dependencies.

## Decision: Current Thread Identity Uses Tenant, Connector, Source Account, And Source Conversation

**Decision**: Maintain at most one current thread for each tenant, connector, source
account, and source conversation. Source message identity remains event evidence, not
thread identity.

**Rationale**: Channel conformance already models durable inbound identity around tenant,
connector account, conversation, and provider message identity. For continuation,
provider message IDs are too granular because every accepted inbound message would
otherwise create a new thread. The source conversation key gives deterministic routing
and replay behavior while still preserving message-level evidence for duplicates and
audit.

**Alternatives considered**:
- Every inbound message creates a new thread: rejected because it does not provide
  conversation continuity.
- Connector chooses continuation behavior: rejected because it leaves daemon-owned
  thread truth fragmented by provider implementation.
- Client supplies thread identity only: rejected because channel connectors need
  restart-safe daemon-owned routing without a browser or TUI client.

## Decision: Additive SQLite Migration With Lazy Legacy Projection

**Decision**: Add thread lifecycle tables and indexes without destructively rewriting
existing sessions or connector messages. Legacy sessions should be projected lazily or
through a bounded backfill into partial thread evidence when they are listed or inspected.

**Rationale**: The repository already uses SQLite with tenant-safe accessors and staged
migration patterns. Additive tables allow rollback by hiding new routes and views while
retaining evidence. Lazy or bounded projection avoids a risky full historical
reconstruction requirement for sessions that lack complete source linkage.

**Alternatives considered**:
- Rewrite all existing sessions into threads in one migration: rejected because legacy
  data may not have complete connector/source/runtime linkage and a destructive rewrite
  is hard to roll back.
- Store thread state only in memory: rejected because restart-safe lifecycle state is a
  core requirement.
- Store only event-derived projections: rejected because list/detail and current-thread
  uniqueness need queryable state and mutation guards.

## Decision: Reuse Existing Tenant Permissions

**Decision**: Require `credentials.inspect` for thread/session lifecycle list, detail,
source linkage, and runtime evidence inspection. Require `connectors.manage` for reset,
archive, and reopen.

**Rationale**: Phase 53 established these permissions for redacted connector inspection
and connector lifecycle mutations. Reuse keeps channel-origin thread evidence aligned
with connector evidence and avoids a new permission family for this roadmap.

**Alternatives considered**:
- Add `sessions.inspect` and `sessions.manage`: rejected for this phase because it would
  require a broader identity/permission rollout before lifecycle value can ship.
- Tenant membership for reads and tenant admin for mutations: rejected because it is too
  broad for support inspection and too coarse for day-to-day channel lifecycle work.

## Decision: API, TypeScript SDK, Web, And Operator Shell/TUI Are Required Surfaces

**Decision**: Expose Roadmap 54 through API resources, TypeScript SDK methods/types, web
product flows, and operator shell/TUI views. Keep existing `/v1/sessions` compatible and
add richer thread lifecycle resources for new behavior.

**Rationale**: The upstream scope explicitly includes SDK and operator shell views. API
contracts provide daemon truth; SDK keeps clients typed; web and operator shell/TUI close
the user and operator inspection workflows without raw database or log access.

**Alternatives considered**:
- API/SDK only: rejected because it would not satisfy user/operator inspection and reset
  workflows.
- Web only: rejected because channel connectors and operator tooling need daemon-owned
  contracts.
- Replace `/v1/sessions`: rejected because backward compatibility is required.

## Decision: Metadata-Only Evidence With 90-Day Default Retention

**Decision**: Lifecycle, source, and runtime projection evidence is metadata-only and
expires from normal inspection after 90 days unless an authorized tenant retention policy
requires longer retention. Redaction failures suppress unsafe detail and emit audit
evidence.

**Rationale**: The feature must let operators trace incidents without creating a new
message-content or provider-payload exposure path. The 90-day default matches connector
management and hosted operational evidence patterns in the repository.

**Alternatives considered**:
- Indefinite lifecycle evidence retention: rejected because it increases privacy and
  operational burden.
- Current-state-only projection: rejected because reset/archive/reopen and support
  tracing require transition history.
- Expose message snippets: rejected because this phase is lifecycle metadata, not memory
  recall or message-content inspection.

## Decision: Runtime Projections Link To Authoritative Records

**Decision**: Thread detail should project summaries of sessions, runs, workflows,
approvals, foreground replies, and background deliveries while keeping each underlying
record authoritative for its own lifecycle.

**Rationale**: Operators need a single thread view that reconstructs what happened, but
the system already has distinct ownership boundaries for runtime, approvals, workflows,
delivery, connector replies, and events. Projection avoids merging these into one
ambiguous status.

**Alternatives considered**:
- Copy full runtime records into thread rows: rejected because it creates stale duplicate
  truth.
- Only link runs and omit approvals/delivery/replies: rejected because the spec requires
  end-to-end source-to-runtime evidence.
- Make thread lifecycle drive run/workflow state: rejected because it violates existing
  subsystem boundaries and archive is not cancellation.
