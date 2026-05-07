# Channel Connector Conformance

Phase 48 defines the shared hosted channel connector contract. The contract applies to
fake connector conformance tests and the Discord regression baseline; Telegram, Slack,
and future connectors consume it in their provider-specific phases.

Hosted-ready connectors must prove these core invariants:

- tenant ownership and permission-gated inspection
- active-tenant account binding for ingress, foreground replies, diagnostics, and
  connector-backed delivery
- redacted APIs, events, logs, fixtures, support output, and conformance evidence
- durable inbound identity using tenant, connector account, channel or conversation, and
  provider message ID, or a documented equivalent durable rule
- stable routing decisions: accepted, ignored, blocked, duplicate, unsupported, or failed
- at least final-only foreground replies for accepted messages
- required diagnostic classifications with freshness, remediation, redaction, and
  retention evidence
- separation between foreground reply outcomes and background delivery outcomes

Provider-specific surfaces such as rooms, threads, rich media, thinking visibility, and
incremental updates may be supported, limited, or unsupported only when explicit. An
unsupported surface cannot weaken a core invariant.

Default verification uses fake connectors and fake credentials in `~/.dope-test`.
Live connector credentials and production tenants are out of scope unless an operator
chooses a separate live validation path.

## Provider-Specific Handoff

Future provider specs must reference this document for shared connector behavior and
limit their own acceptance criteria to provider mechanics. A provider spec may add
surface-specific tests for rooms, threads, media, cards, edits, stop controls, or rate
limits, but it must not redefine tenant ownership, active-tenant account binding,
durable inbound identity, dedupe, routing outcome meanings, redaction, diagnostics, or
foreground/background delivery separation.

Each provider handoff must include:

- a capability profile declaring every core invariant as pass or fail, and every
  provider-specific surface as supported, limited, or unsupported
- any equivalent durable identity rule, with the exact provider fields that replace the
  standard tenant/account/channel/provider-message identity
- routing fixtures for direct, group, mention-gated, blocked, unsupported, duplicate,
  and failed inputs that use the shared outcome vocabulary
- diagnostic mappings from provider failures into the shared connector reason codes
- explicit rollback notes for disabling the provider without deleting retained
  conformance or diagnostic evidence
