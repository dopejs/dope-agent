# Tasks: Mail Attachment Transfer

**Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md) | **Roadmap**: 64

Stories: US1 download, US2 add/remove + send, US3 inspect/audit.

## Phase 1: Setup
- [X] T001 [Setup] Baseline green; confirm send-time resolveAndBlockOnAttachments + fake resolve.

## Phase 2: Foundational
- [X] T002 [Foundational] attachments_policy.go: EvaluateAttachment(size/MIME) -> status +
  retention class + redaction + failure reason (too_large / unsupported_type).
- [X] T003 [Foundational] types.go: AttachmentReference RetentionClass + Redacted (additive);
  OperationClassDownloadAttachment; DownloadAttachmentInput.
- [X] T004 [Foundational] backend.go: Backend gains DownloadAttachment(resource, account, input).

## Phase 3: US1 — download
- [X] T005 [US1] manager.DownloadAttachment op (download_attachment); fake + feishulark download
  produce a managed attachment artifact under policy.
- [X] T006 [P] [US1] tests: download produces artifact w/ policy metadata; too_large/unsupported
  fail explicitly with no partial.

## Phase 4: US2 — resolve-with-policy + send
- [X] T007 [US2] fake + feishulark ResolveAttachments apply policy (resolved within policy;
  failed over policy); draft attachment add/remove via resolved refs.
- [X] T008 [P] [US2] tests: in-policy attachment resolves + links to draft; over-policy fails;
  unresolved reference blocks send (no partial).

## Phase 5: US3 — audit + diagnostics
- [X] T009 [US3] attachment-bearing send links to operation + delivery; resolution status +
  failure reason inspectable; no content/credential leakage.
- [X] T010 [P] [US3] tests: send links attachments; policy reasons surfaced.

## Phase 6: API + polish
- [X] T011 [API] api/mail.go + types.go: attachment download endpoint (additive); expose
  retentionClass/redacted.
- [X] T012 [Polish] schemas: additive attachment fields + download_attachment op class; contract test.
- [X] T013 [Polish] verify: build/vet/test mail, feishulark, api, opsreadiness, contracts;
  non-attachment mail suite green. Docs note.

## Dependencies
T002/T003 block all. T005 before T006. T007 before T008. T009 after T007.
</content>
