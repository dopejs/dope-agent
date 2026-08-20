# Feature Specification: Mail Integration

**Feature Branch**: `[015-mail-integration]`  
**Created**: 2026-04-23  
**Status**: Implemented  
**Input**: User description: "结合 docs/specs/015-mail-integration.md 完成 phase 30 的工作"

## Clarifications

### Session 2026-04-23

- Q: phase 30 的附件范围要定到哪一层？ → A: 仅要求附件元数据与失败真相可见，不要求完整上传/下载附件能力。
- Q: background mail workflow 对“最终发送邮件”应允许到什么程度？ → A: 后台 workflow 只有在工作流显式声明允许 send side effect 时才可最终发送。
- Q: 对“新发邮件”（不是 reply/forward）的收件人选择，phase 30 应限制到什么程度？ → A: 仅允许使用用户在当前请求中明确给出的收件人。
- Q: phase 30 的最终发送路径要限制成什么形式？ → A: 既支持直接发送新消息，也支持发送已有 draft，但两者结果必须可区分。
- Q: 当发送请求引用了附件，但附件引用无法解析或验证时，phase 30 应如何处理？ → A: 阻止最终发送；返回显式失败或保留为 draft-only 结果。

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Inspect Mailbox State Truthfully (Priority: P1)

As a user or operator, I need to inspect which mailbox is active, what conversations or
messages exist, and what account is being used so I can trust the mail domain before the
agent drafts or sends anything.

**Why this priority**: Phase 30 is unsafe if mailbox truth is hidden. Operators need to
understand readiness, mailbox identity, and thread or message state before trusting any
side-effecting mail action.

**Independent Test**: Inspect a representative mail account, request thread list or
message detail information, and confirm the result shows mailbox identity, selected
integration truth, and message or thread state without creating a draft or sending a
message.

**Acceptance Scenarios**:

1. **Given** a healthy mail account is available, **When** the user asks to list recent
   conversations or inspect a specific thread, **Then** the system returns truthful
   mailbox identity and thread or message details without creating a draft or sending
   mail.
2. **Given** more than one mail integration could satisfy the same request, **When** the
   user performs mail inspection without naming a specific integration, **Then** the
   system uses the canonical default account projection and makes that choice inspectable
   to the operator.
3. **Given** more than one mail integration could satisfy the same request, **When** the
   user provides an explicit `integrationId` on a mail inspection request, **Then** the
   system uses that integration or returns a truthful selection failure instead of
   silently falling back to another mailbox.

---

### User Story 2 - Draft And Send With Explicit Side-Effect Truth (Priority: P2)

As a user, I need the agent to draft, update, send, reply, and forward mail while making
it obvious which actions only changed draft state and which actions created a sent-mail
side effect.

**Why this priority**: Mail is valuable only if side effects are truthful. The system
must never blur draft-only actions with final send behavior, especially when reply,
forward, and attachment handling are involved.

**Independent Test**: Create a representative draft, revise it, send a message, reply to
an existing thread, and forward a message while confirming each action preserves mailbox
truth, distinguishes draft-only work from final send, and records any attachment
metadata or attachment failure explicitly.

**Acceptance Scenarios**:

1. **Given** a healthy mail account and valid message content, **When** the user asks the
   agent to create a draft, **Then** the system records a draft result with mailbox,
   thread, and draft identity where applicable and does not present the action as a sent
   message.
2. **Given** an existing draft, **When** the user asks the agent to update that draft,
   **Then** the system preserves the draft identity and returns the changed draft state
   or a truthful explanation of why the update could not be applied.
3. **Given** a valid outgoing message or draft is ready, **When** the user asks the
   agent to send it, **Then** the system records a sent-message result that is clearly
   distinguishable from draft-only outcomes.
4. **Given** the user requests final send, **When** the request either references an
   existing draft or provides complete new-message content directly, **Then** the system
   may send through either path but records which path produced the sent-message result.
5. **Given** a new outbound message is not a reply or forward, **When** the user asks
   the agent to send it, **Then** the system uses only recipients explicitly provided
   in the current request and does not infer additional recipients from mailbox context
   or prior drafts.
6. **Given** an existing inbound or sent message, **When** the user asks the agent to
   reply, **Then** the system preserves the relationship to the original conversation and
   returns truthful send or draft-only outcome based on the requested action.
7. **Given** an existing message, **When** the user asks the agent to forward it,
   **Then** the system records the forward as a distinct mail action and returns
   truthful outcome and message linkage.
8. **Given** a mail action references attachments, **When** attachment metadata is
   available, unavailable, or partially incomplete, **Then** the system keeps
   attachment metadata and attachment failures operator-visible and does not present
   phase 30 as having completed full attachment transfer behavior.
9. **Given** a final-send request references attachments, **When** any required
   attachment reference cannot be resolved or validated, **Then** the system does not
   finalize send and instead returns explicit failure or a draft-only outcome.

---

### User Story 3 - Run Mail Work Through Shared Delivery (Priority: P3)

As a user, I need background or workflow-driven mail work to deliver a truthful result
after it inspects mail, creates a draft, or sends a message, even when I am not in an
active foreground conversation.

**Why this priority**: The upstream roadmap explicitly requires mail to reuse the shared
delivery plane. Phase 30 is incomplete if mail actions only work during live foreground
chat.

**Independent Test**: Run a scheduled or workflow-driven mail task that inspects mail,
creates a draft, or sends a message, and confirm it uses normal mail operation truth and
routes its result through the shared background-result delivery behavior.

**Acceptance Scenarios**:

1. **Given** a scheduled or workflow-driven mail task runs in the background, **When**
   it inspects mailbox state, creates a draft, or sends a message, **Then** the system
   records the mail outcome and can deliver a truthful result without requiring an
   active chat session.
2. **Given** a background workflow can perform mail work, **When** it has not explicitly
   declared permission for send side effects, **Then** it may inspect mail or create or
   update drafts but MUST NOT finalize a sent message.
3. **Given** a background mail action succeeds but result delivery fails, **When** an
   operator inspects the outcome, **Then** the system distinguishes successful mail
   execution from failed delivery rather than conflating them.
4. **Given** a background mail action fails before any message is sent, **When** the
   result is inspected, **Then** the system reports the mail failure truthfully and does
   not claim a sent-mail side effect or successful delivery that never happened.

### Edge Cases

- If the mail account is connected in one environment but not another, the system keeps
  mailbox readiness, draft history, send history, and delivery truth environment-scoped
  instead of implying cross-environment reuse.
- If multiple mail integrations can represent the same mailbox, the system makes the
  selected integration inspectable and fails truthfully instead of silently rerouting a
  request to another mailbox.
- If a thread or draft changes externally between inspection and a later update or send
  request, the system returns a truthful stale-state or conflict result instead of
  claiming the action succeeded unchanged.
- If attachment metadata cannot be inspected or an attachment reference cannot be
  resolved as requested, the system exposes that failure explicitly instead of silently
  claiming successful attachment transfer behavior that phase 30 does not support.
- If a send request depends on attachment references that cannot be resolved or
  validated, the system blocks final send rather than silently sending a message with
  missing attachments.
- If a user asks only to draft a message, the system does not create a sent-mail side
  effect as part of satisfying the request.
- If a new outbound mail request omits explicit recipients, the system does not infer
  recipients from unrelated mailbox context and instead fails truthfully or remains in
  draft-only state.
- If a background mail action completes successfully but delivery is suppressed, delayed,
  or fails, operators can still inspect successful mail execution separately from
  delivery truth.
- If a background workflow is not explicitly marked as allowed to send mail, the system
  does not finalize a sent message even if it can inspect mail or prepare a draft.
- If the user attempts CRM-style mailbox automation, campaign messaging, or other
  generalized outbound automation, the system reports those capabilities as out of scope
  for phase 30 rather than presenting partial behavior as complete.
- If a later workflow references calendar state while composing mail, phase 30 continues
  to rely on the established calendar-domain truth rather than inventing a separate
  calendar view inside the mail domain.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST expose inspectable mail account readiness and mailbox
  identity by reusing the shared integration readiness, account-binding, and
  canonical-default contract from phase 27.
- **FR-002**: Users or operators MUST be able to inspect mail thread lists, thread
  details, and message state through the selected mailbox without creating a draft or
  sending mail as a side effect of inspection.
- **FR-003**: Mail reads, draft creation, draft updates, send, reply, and forward MUST
  remain separate operation classes with independently inspectable outcomes.
- **FR-004**: Users MUST be able to create a draft and receive a truthful result that
  distinguishes draft-only state from any sent-message side effect.
- **FR-005**: Users MUST be able to update an existing draft and receive either the
  updated draft state or a truthful explanation of why the update was not applied.
- **FR-006**: Users MUST be able to send a message and receive a truthful outcome that
  distinguishes successful send, failed send, and draft-only results.
- **FR-006a**: For new outbound mail that is not a reply or forward, the system MUST use
  only recipients explicitly provided in the current user request. It MUST NOT infer
  additional recipients from mailbox context, prior drafts, or background workflow state.
- **FR-006b**: Phase 30 MUST support both direct send of a new message and send of an
  existing draft, and operator-visible history MUST distinguish which send path was used.
- **FR-007**: Users MUST be able to reply to an existing message or thread while
  preserving the relationship to the original conversation in operator-visible history.
- **FR-008**: Users MUST be able to forward an existing message while preserving
  operator-visible linkage to the forwarded source.
- **FR-009**: Mail reads and writes MUST preserve mailbox identity plus thread, message,
  or draft identity where applicable across operator-visible history and downstream
  result delivery.
- **FR-010**: Attachment handling MUST remain explicit and auditable. Attachment
  metadata and attachment-related failures MUST remain operator-visible for draft, send,
  reply, and forward actions when attachment references are involved.
- **FR-011**: Phase 30 attachment scope is limited to attachment metadata and failure
  truth. Full attachment upload, download, or generalized transfer behavior MUST NOT be
  required to claim completion in this phase.
- **FR-012**: The system MUST NOT silently drop or hide attachment-related failures; if
  attachment metadata cannot be inspected or an attachment reference cannot be resolved
  as requested, the resulting truth MUST say so explicitly.
- **FR-012a**: If a final-send request references attachments that cannot be resolved or
  validated as required, the system MUST NOT finalize send. It MUST return explicit
  failure or a draft-only result instead.
- **FR-013**: Scheduled workflows and other normal runtime workflows MUST be able to
  invoke mail inspection, draft, send, reply, and forward behavior without requiring a
  special mail-only execution path.
- **FR-014**: Background workflows MUST NOT finalize a sent message unless that workflow
  explicitly declares permission for send side effects. Without that declaration,
  background mail behavior is limited to mailbox inspection and draft-only actions.
- **FR-015**: Background mail results MUST reuse the shared delivery targets,
  preferences, and outcome history from phase 28 instead of introducing a mail-specific
  notification plane.
- **FR-016**: Operator-visible history MUST distinguish mail readiness truth, mail
  execution truth, and delivery truth so connection problems, draft failures, send
  failures, and delivery failures are not conflated.
- **FR-017**: When multiple mail integrations can satisfy a request, mail read and write
  routes MUST honor an explicit request-scoped `integrationId` when provided or use the
  canonical default mailbox projection otherwise, and the chosen integration selection
  MUST remain inspectable in the resulting mail operation.
- **FR-018**: Mail behavior MUST remain environment-scoped so test and later live
  environments do not share implicit mailbox bindings, draft state, send history, or
  delivery history.
- **FR-019**: Phase 30 MUST stay scoped to the first personal mail slice and MUST NOT
  require CRM mailbox automation, generalized marketing campaigns, or cross-domain
  calendar truth redefinition to claim completion.

### Key Entities *(include if feature involves data)*

- **Mail Account Projection**: The inspectable mail-domain view of the selected
  integration binding within one environment, including mailbox identity, readiness, and
  whether mailbox selection was explicit or canonical-default.
- **Mail Thread**: A conversation grouping whose identity, participants, subject
  context, and message membership can be inspected and reused by reply or forward
  actions.
- **Mail Message**: One operator-visible inbound or outbound message with sender or
  recipient context, content state, delivery state, and optional relationship to a
  thread.
- **Mail Draft**: A not-yet-sent outbound mail object that can be created, updated,
  inspected, and later sent while remaining distinguishable from a delivered message.
- **Attachment Reference**: The operator-visible description of an attachment associated
  with a draft, sent message, reply, or forward action, including identity, metadata,
  and any attachment-specific failure truth.
- **Mail Operation**: The operator-visible record of a mail read or write, including the
  chosen mailbox projection, operation class, related thread or message or draft
  identity, and the truthful terminal outcome.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Additive operator-visible contract, schema, event, config,
  and storage surface changes are expected for mailbox projections, thread inspection,
  draft and send history, attachment metadata, and mail-linked delivery results. Existing
  non-mail behavior remains backward compatible.
- **Migration / Rollback**: Additive mail-domain resources and history are required.
  Rollback is a revert of mail-specific routes, projections, and persistence while
  preserving already-recorded mail and delivery history as read-only audit truth where
  needed.
- **Verification Strategy**: Required validation includes targeted domain coverage for
  mailbox inspection, draft create and update behavior, send or reply or forward flows,
  separation of draft-only and sent-message truth, attachment metadata and failure
  projection, workflow and schedule-driven mail execution, separation of readiness,
  execution, and delivery truth, contract coverage for any new mail projections, and at
  least one repo-owned local or fixture-based verification path in `KURA_ENV=test`.
- **Observability Impact**: Operators must be able to inspect the selected mailbox,
  operation class, related thread or message or draft identity, attachment summaries and
  failures, sent-versus-draft-only outcome, and downstream delivery outcome without
  reading raw connector logs or provider-specific debug output.
- **Environment & Secrets**: Work defaults to `KURA_ENV=test`. Live mail connectors are
  optional for initial validation. Any credentials or tokens used for mail access remain
  operator-owned, environment-scoped, and redacted from operator-visible history.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In manual validation, an operator can determine the active mailbox
  projection, current readiness, and recent thread or message state in under 2 minutes
  using operator-visible surfaces only.
- **SC-002**: In automated and manual validation combined, a representative draft can be
  created and updated, and a representative message can be sent, replied to, and
  forwarded with truthful separation between draft-only and sent-message outcomes in
  100% of exercised test cases.
- **SC-003**: In automated or fixture-based verification, 100% of exercised
  attachment-related cases show attachment metadata or attachment failure truth, with no
  tested case of an unresolved attachment being silently omitted from a finalized send
  outcome.
- **SC-004**: At least one scheduled or workflow-driven mail task can inspect mail,
  create a draft, or send a message and deliver its final result through the shared
  background-result delivery path without requiring an active chat session.
- **SC-005**: In manual or fixture-based validation, an operator can determine within 2
  minutes whether a failed mail outcome was caused by readiness, stale or conflicting
  mailbox state, attachment handling, send failure, or downstream delivery failure using
  operator-visible surfaces only.

## Assumptions

- Phase 30 builds on the shared integration readiness and account-binding behavior from
  phase 27 instead of redefining connection lifecycle rules inside the mail domain.
- Phase 30 reuses the shared delivery targets, preferences, and outcome history from
  phase 28 instead of creating a mail-specific notification plane.
- Existing scheduled-task and workflow capabilities remain the trigger surface for
  background mail work; this phase does not define a separate scheduling system.
- Background workflows can inspect mail and manage drafts by default, but final send in
  background execution requires an explicit workflow-level declaration allowing send side
  effects.
- The first mail slice focuses on truthful mailbox inspection, draft handling, send,
  reply, and forward behavior for personal mailbox use.
- Phase 30 send behavior supports both direct-send and send-existing-draft flows, but it
  must preserve truthful distinction between those send paths in operator-visible history.
- New outbound mail in phase 30 requires explicit recipients in the current request;
  broader recipient inference is intentionally deferred beyond this phase.
- Attachment handling in phase 30 must remain explicit and auditable, but the phase does
  not require full attachment upload or download behavior, generalized mailbox
  automation, campaign tooling, or broad attachment content-processing workflows to
  claim completion.
- If a requested send depends on attachment references that cannot be resolved, phase 30
  favors blocking final send or preserving draft-only truth over degraded send.
- Later workflows that combine mail and calendar state must reuse the established
  calendar-domain contract rather than redefining calendar truth inside the mail domain.
- Single-operator environment behavior remains the default; multi-user tenancy remains
  out of scope for this phase.
