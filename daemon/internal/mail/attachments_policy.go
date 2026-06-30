package mail

import (
	"fmt"
	"path/filepath"
	"strings"
)

// MaxAttachmentBytes is the default attachment size limit (Roadmap 64). Attachments larger than
// this are failed explicitly (too_large) rather than transferred.
const MaxAttachmentBytes int64 = 25 * 1024 * 1024

// RetentionClassStandard is the default retention class for a resolved attachment artifact.
const RetentionClassStandard = "standard"

// blockedMediaTypes and blockedExtensions are basic safety rules: executable / script content is
// not transferred. This is explicit-rule safety only — no document parsing or malware scanning.
var blockedMediaTypes = map[string]bool{
	"application/x-msdownload":                      true,
	"application/x-dosexec":                         true,
	"application/x-executable":                      true,
	"application/vnd.microsoft.portable-executable": true,
	"application/x-sh":                              true,
	"application/x-bat":                             true,
	"application/x-msdos-program":                   true,
}

var blockedExtensions = map[string]bool{
	".exe": true, ".bat": true, ".cmd": true, ".com": true, ".scr": true,
	".sh": true, ".dll": true, ".msi": true, ".ps1": true, ".js": true,
}

// AttachmentPolicyResult is the outcome of evaluating an attachment against transfer policy.
type AttachmentPolicyResult struct {
	Status         AttachmentResolutionStatus
	RetentionClass string
	Redacted       bool
	FailureReason  string
}

// EvaluateAttachment applies size and MIME/extension safety policy. Over-limit attachments fail
// with a too_large reason; blocked types fail with an unsupported_type reason; otherwise the
// attachment resolves with the standard retention class. Content is never inspected here.
func EvaluateAttachment(displayName, mediaType string, sizeBytes int64) AttachmentPolicyResult {
	if sizeBytes > MaxAttachmentBytes {
		return AttachmentPolicyResult{
			Status:        AttachmentResolutionFailed,
			FailureReason: fmt.Sprintf("too_large: attachment is %d bytes (limit %d)", sizeBytes, MaxAttachmentBytes),
		}
	}
	if isBlockedAttachment(displayName, mediaType) {
		return AttachmentPolicyResult{
			Status:        AttachmentResolutionFailed,
			FailureReason: "unsupported_type: attachment media type is not permitted for transfer",
		}
	}
	return AttachmentPolicyResult{
		Status:         AttachmentResolutionResolved,
		RetentionClass: RetentionClassStandard,
		Redacted:       false,
	}
}

func isBlockedAttachment(displayName, mediaType string) bool {
	if blockedMediaTypes[strings.ToLower(strings.TrimSpace(mediaType))] {
		return true
	}
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(displayName)))
	return blockedExtensions[ext]
}

// ApplyAttachmentPolicy stamps the policy result onto an attachment reference. It is the single
// place provider and fake backends use so resolution is consistent.
func ApplyAttachmentPolicy(ref *AttachmentReference) {
	result := EvaluateAttachment(ref.DisplayName, ref.MediaType, ref.SizeBytes)
	ref.ResolutionStatus = result.Status
	ref.RetentionClass = result.RetentionClass
	ref.Redacted = result.Redacted
	if result.FailureReason != "" {
		ref.FailureReason = result.FailureReason
	}
}
