# Feature Specification: Mail Attachment Transfer

**Feature Branch**: `main`
**Created**: 2026-06-30
**Status**: Draft
**Phase / Roadmap**: Phase 64 — Roadmap 64
**Upstream authority**: [docs/specs/049-mail-attachment-transfer.md](../../docs/specs/049-mail-attachment-transfer.md)
**Provider decision**: **Feishu/Lark Mail** (continues Roadmap 63).

## Overview

Roadmap 30 recorded attachment metadata and blocked unresolved attachment references; Roadmap
63 left attachment references unresolved (so attachment-bearing sends block). Roadmap 64 makes
attachments **safe, auditable artifacts**: download an attachment as a managed artifact, upload
(resolve) an attachment for a draft/send, add/remove draft attachments, validate unresolved
references at send time, and enforce size / MIME / retention / redaction policy. Attachments are
artifacts with retention + redaction policy, not raw provider blobs; attachment send is
externally visible and linked to mail operation + delivery truth; large or unsafe attachments
fail explicitly. This roadmap adds no document intelligence or memory extraction.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Download an attachment as a managed artifact (Priority: P1)
**Acceptance Scenarios**:
1. Given a message with an attachment, when the user downloads it, then a managed attachment
   artifact is produced with display name, MIME, size, retention class, and redaction status.
2. Given an attachment exceeding the size limit, when download is attempted, then it fails
   explicitly (too_large) with no partial artifact.
3. Given an unsafe/blocked MIME type, when download is attempted, then it fails explicitly
   (unsupported_type).

### User Story 2 - Add/remove draft attachments and send (Priority: P2)
**Acceptance Scenarios**:
1. Given a draft, when the user adds an attachment within policy, then the attachment resolves
   (resolved) and is linked to the draft.
2. Given a draft attachment over policy, when added, then it is recorded failed (too_large /
   unsupported_type), not silently dropped.
3. Given a send with an unresolved attachment reference, when submitted, then it is blocked
   (attachment_unresolved) with no partial send.
4. Given an attachment-bearing send within policy, when submitted, then the send is linked to
   the attachment artifacts and to delivery truth.

### User Story 3 - Inspect attachment outcome and audit (Priority: P3)
**Acceptance Scenarios**:
1. An operator can inspect whether an attachment was uploaded, blocked, too large, expired, or
   redacted, via the attachment resolution status + failure reason.
2. No raw secret/credential material and no attachment content beyond the redacted artifact is
   exposed in any log/event/diagnostic.

### Edge Cases
- Size limit boundary, blocked MIME, redaction of sensitive content, retention expiry,
  unresolved reference at send, provider download failure (mapped to a stable diagnostic).

## Requirements *(mandatory)*

### Functional Requirements
- **FR-001**: Attachments MUST be modeled as artifacts with retention class and redaction
  status, not raw provider blobs; downloads produce a managed attachment artifact.
- **FR-002**: The provider MUST support attachment download (provider -> managed artifact) and
  upload/resolve (reference -> resolved attachment for a draft/send).
- **FR-003**: Draft attachment add/remove MUST be supported and linked to the draft.
- **FR-004**: Send-time MUST validate attachment references; an unresolved reference MUST block
  the send (attachment_unresolved) with no partial send.
- **FR-005**: Size, MIME, retention, and redaction policy MUST be enforced; over-limit or unsafe
  attachments MUST fail explicitly (too_large / unsupported_type) with a reason, no partial.
- **FR-006**: Attachment-bearing sends MUST be linked to mail operation truth and delivery
  linkage (audit).
- **FR-007**: No credential/secret material and no attachment content beyond the redacted
  artifact MUST be exposed.
- **FR-008**: Existing non-attachment mail behavior MUST remain compatible; attachment fields
  MUST be additive and backward compatible.

### Key Entities
- Attachment Artifact (managed: display name, MIME, size, retention class, redaction status,
  resolution status, failure reason), Attachment Reference (resolved/unresolved/failed),
  Attachment Policy (size limit, MIME rules, retention, redaction).

## Compatibility & Operational Impact *(mandatory)*
- **Compatibility**: Additive attachment fields + new download/resolve operations behind the
  existing mail Backend + adapter plane; non-attachment behavior unchanged.
- **Migration / Rollback**: No migration; rollback = no attachment transfer (references stay
  unresolved, sends with attachments block as in Roadmap 63).
- **Verification**: fake + provider attachment resolve/download/policy tests; send-time block
  test; size/MIME policy tests; audit/delivery linkage; existing mail suite green.
- **Observability**: reuse mail operation/artifact events; attachment resolution + failure
  reason surface the outcome; no new event families.

## Success Criteria *(mandatory)*
- **SC-001**: Download produces a managed attachment artifact with policy metadata.
- **SC-002**: Over-limit / unsafe attachments fail explicitly with a reason, no partial.
- **SC-003**: Unresolved references block sends with no partial send.
- **SC-004**: Attachment-bearing sends link to operation + delivery truth.
- **SC-005**: Zero attachment-content / credential leakage beyond the redacted artifact.
- **SC-006**: Existing non-attachment mail tests remain green; fields additive.

## Assumptions
- Provider is Feishu/Lark Mail. Policy limits are configurable with safe defaults. No document
  parsing / malware scanning beyond basic safety metadata. Fake backend remains primary dev path.
