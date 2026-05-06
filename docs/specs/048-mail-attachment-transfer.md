# Mail Attachment Transfer

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 63, the mail
attachment upload, download, and artifact transfer slice.

Primary source documents:
- `docs/specs/015-mail-integration.md`
- `docs/specs/047-real-mail-provider-closure.md`
- `docs/specs/011-use-computer-capability-plane.md`

## Background

Phase 30 records attachment metadata and blocks unresolved attachment references, but full
attachment upload/download and transfer were deliberately out of scope. Public mail parity
requires attachments to become safe, auditable artifacts.

## Goal

Support mail attachment download, upload, draft attachment linkage, send-time validation,
redaction, and artifact retention.

## Fixed Decisions

- Attachments are artifacts with retention and redaction policy, not raw provider blobs.
- Attachment send is externally visible and must be linked to mail operation truth.
- Large or unsafe attachments must fail explicitly.
- This roadmap does not add document intelligence or memory extraction.

## Dependencies On Completed Phases

- Roadmap 62: Real Mail Provider Closure
- Roadmap 26: Use-Computer Capability Plane, for artifact handling precedent

## In Scope

- attachment artifact resource extensions
- provider download and upload adapters
- draft attachment add/remove
- send-time unresolved reference validation
- size, MIME, retention, and redaction policy
- audit and delivery linkage for attachment-bearing sends

## Out Of Scope

- semantic document parsing
- malware scanning beyond explicit basic safety metadata unless already available
- memory ingestion from attachments
- CRM campaign attachment workflows

## Operator Or User Problems To Solve

- Users need to send and inspect attachments safely.
- Operators need to know whether an attachment was uploaded, blocked, too large, expired,
  or redacted.

## User Stories

- As a user, I can download an attachment as a managed artifact.
- As a user, I can attach a managed artifact to a draft and send it.
- As support, I can inspect redacted attachment evidence without raw sensitive content.

## Functional Requirements

- The system MUST model attachment artifacts with provider IDs, size, MIME type, checksum
  where available, retention state, and redaction state.
- Draft and send operations MUST validate attachment references before provider mutation.
- Failed attachment transfer MUST record stable failure reason codes.
- Attachment content access MUST be permission-gated and auditable.

## Compatibility And Operational Notes

Existing mail metadata remains compatible. Attachment content storage must follow hosted
artifact path and retention rules.

## Verification Expectations

- Fake provider tests for download, upload, too-large, missing reference, and provider
  failure.
- API/schema/SDK/web tests for attachment artifacts and draft linkage.
- Redaction and retention tests for content access.

## Definition Of Done

- Common mail attachment workflows work through managed artifact truth instead of being
  blocked or provider-specific.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/048-mail-attachment-transfer.md 完成 phase 63 的工作`
