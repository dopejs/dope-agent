# Thread Session Lifecycle

Roadmap 54 makes tenant threads, session segments, source linkage, lifecycle actions,
and runtime projections daemon-owned inspection resources.

The feature is lifecycle metadata, not memory. Thread detail may show redacted source
identity, current session segment, prior segments, lifecycle actions, connector routing
outcomes, runs, replies, approvals, workflows, and delivery facts. It must not create
memory recall, semantic summaries, context packing, autonomous pruning, or memory-driven
routing.

## Operator Model

- `GET /v1/threads` lists tenant-scoped thread resources with lifecycle state, source
  kind, source summary, current session metadata, last activity, redaction status,
  retention expiry, and available actions.
- `GET /v1/threads/{threadId}` returns detail with session segments, source linkages,
  runtime projections, and lifecycle actions. Source and runtime facts are separate
  metadata records.
- `POST /v1/threads/{threadId}/reset` preserves the thread id and starts a new current
  session segment.
- `POST /v1/threads/{threadId}/archive` blocks future continuation for current source
  mappings but does not cancel work already accepted.
- `POST /v1/threads/{threadId}/reopen` preserves prior archive/reopen evidence and
  allows future continuation when the source remains eligible.

Reads require `credentials.inspect`. Mutations require `connectors.manage`. Callers must
reauthorize each list, detail, trace, reset, archive, and reopen request.

## Source And Runtime Evidence

Connector ingress records accepted, duplicate, blocked, ignored, disabled, unsupported,
failed, unknown-source, stale-source, and inaccessible-tenant-binding outcomes. Accepted
messages attach to the daemon-owned current thread and session segment for the tenant,
connector, source account, and source conversation. Duplicate and blocked replay uses
persisted daemon source linkage after restart.

Runtime projections link the thread to sessions, runs, workflows, approvals, foreground
replies, background deliveries, and connector messages. Projection summaries must remain
metadata-only and redact or suppress unsafe provider payloads, message bodies, secrets,
and cross-tenant identifiers.

## Restart And Retention

Startup restores existing sessions and projects legacy sessions as partial thread
evidence when complete lifecycle linkage is missing. Ambiguous restart state records
metadata-only recovery evidence for operator review.

Lifecycle, source, and runtime evidence uses a 90-day default inspection retention period
unless an authorized tenant policy requires longer retention. Expired evidence is omitted
from normal thread detail responses; retained audit metadata remains redacted.
