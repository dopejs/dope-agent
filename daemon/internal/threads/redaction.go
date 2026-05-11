package threads

import "errors"

type RedactionStatus string

const (
	RedactionStatusRedacted        RedactionStatus = "redacted"
	RedactionStatusSuppressed      RedactionStatus = "suppressed"
	RedactionStatusRedactionFailed RedactionStatus = "redaction_failed"
)

var (
	ErrAuditEvidenceRequired         = errors.New("required audit evidence must be recorded before lifecycle mutation")
	ErrLifecycleTransitionNotAllowed = errors.New("lifecycle transition is not allowed")
	ErrLifecycleMutationConflict     = errors.New("lifecycle mutation conflicted with concurrent thread update")
	ErrLifecycleReopenNotEligible    = errors.New("thread source or session is not eligible for reopen")
)

type SafeEvidenceSummary struct {
	Text   string
	Status RedactionStatus
}

func SafeSummary(text string, allowed bool) SafeEvidenceSummary {
	if !allowed {
		return SafeEvidenceSummary{Text: "suppressed", Status: RedactionStatusSuppressed}
	}
	return SafeEvidenceSummary{Text: text, Status: RedactionStatusRedacted}
}
