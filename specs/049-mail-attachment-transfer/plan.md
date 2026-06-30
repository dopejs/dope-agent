# Implementation Plan: Mail Attachment Transfer

**Branch**: `main` | **Spec**: [spec.md](./spec.md) | **Upstream**: [docs/specs/049-mail-attachment-transfer.md](../../docs/specs/049-mail-attachment-transfer.md)
**Phase / Roadmap**: Phase 64 — Roadmap 64

## Summary

Make mail attachments safe, auditable artifacts on the existing mail domain + Feishu/Lark
provider. Add a shared attachment policy (size / MIME / retention / redaction); make attachment
resolution actually resolve within-policy references (real provider replaces the always-
unresolved behavior from Roadmap 63) and fail over-policy/unsafe ones explicitly; add an
attachment download operation producing a managed attachment artifact; keep send-time
unresolved-reference validation (already blocks) and link attachment-bearing sends to operation
+ delivery truth. No document intelligence / memory extraction.

## Technical Context
- **Language**: Go 1.24 (daemon). Additive only.
- **Dependencies**: `internal/mail` (AttachmentReference, Backend, Manager, fake, artifacts),
  `internal/integrations/providers/feishulark` (ResolveAttachments + DownloadAttachment), the
  adapter plane, `internal/api/mail.go` + schemas.
- **Storage**: attachments reuse the existing mail Artifact / attachment persistence; managed
  attachment artifact rides on the existing attachment reference (no new store).
- **Testing**: policy unit tests (size/MIME); real provider resolve-with-policy; download
  produces artifact; send blocks on unresolved; existing mail suite green.
- **Constraints**: additive + backward compatible; no partial send/transfer on policy failure;
  no attachment-content or credential leakage beyond the redacted artifact.

## Constitution Check
- **Roadmap closure**: attachments become safe artifacts with download/upload, policy, and audit
  linkage — the upstream DoD.
- **Production-grade**: explicit size/MIME limits, no-partial-on-failure, redaction/retention,
  unresolved-reference send block.
- **Contracts first**: additive attachment fields (retentionClass, redacted) + download_attachment
  op class; contract tests validate.
- **Verification**: policy + provider + manager + send-block coverage; non-attachment unchanged.
- **Environment**: fake default; real provider only with explicit operator credentials.

## Project Structure
```
specs/049-mail-attachment-transfer/  spec.md plan.md tasks.md checklists/
daemon/internal/mail/attachments_policy.go   # NEW: size/MIME/retention/redaction policy
daemon/internal/mail/types.go                # EDIT: AttachmentReference RetentionClass+Redacted;
                                             #       OperationClassDownloadAttachment; DownloadAttachmentInput
daemon/internal/mail/backend.go              # EDIT: Backend gains DownloadAttachment
daemon/internal/mail/manager.go              # EDIT: DownloadAttachment op; policy on resolve
daemon/internal/mail/fake_backend.go         # EDIT: policy in resolve; DownloadAttachment
daemon/internal/mail/adapter_backend.go      # EDIT: DownloadAttachment dispatch
daemon/internal/integrations/providers/feishulark/mail.go  # EDIT: resolve-with-policy + download
daemon/internal/api/mail.go, types.go        # EDIT: attachment download endpoint (additive)
schemas/api                                  # additive attachment fields + op class
```

## Complexity Tracking
No violations. Attachment policy + download are additive behind the existing mail Backend +
adapter plane; send-time unresolved-reference blocking already exists and is reused.
</content>
