package livevalidation

type FakeOutcome string

const (
	FakeOutcomeCompleted          FakeOutcome = "completed"
	FakeOutcomeFailed             FakeOutcome = "failed"
	FakeOutcomeTimeoutAfterSubmit FakeOutcome = "timeout_after_submit"
	FakeOutcomeDuplicateRetry     FakeOutcome = "duplicate_retry"
	FakeOutcomeSubmitUnknown      FakeOutcome = "submit_unknown"
)

type FakeOutcomeResult struct {
	Outcome                LedgerOutcome `json:"outcome"`
	AmbiguousCommit        bool          `json:"ambiguousCommit"`
	AutomaticRetryAllowed  bool          `json:"automaticRetryAllowed"`
	CorrelationKeyRequired bool          `json:"correlationKeyRequired"`
	ReasonCode             string        `json:"reasonCode,omitempty"`
}

func FakeOutcomeResultFor(outcome FakeOutcome, safetyClass SafetyClass) FakeOutcomeResult {
	switch outcome {
	case FakeOutcomeCompleted:
		return FakeOutcomeResult{Outcome: LedgerOutcomeCompleted, CorrelationKeyRequired: true}
	case FakeOutcomeFailed:
		return FakeOutcomeResult{Outcome: LedgerOutcomeFailed, AutomaticRetryAllowed: safetyClass != SafetyClassNonIdempotentMutation, CorrelationKeyRequired: true, ReasonCode: "live_validation.fake_failed"}
	case FakeOutcomeDuplicateRetry:
		return FakeOutcomeResult{Outcome: LedgerOutcomeCompleted, AutomaticRetryAllowed: safetyClass != SafetyClassNonIdempotentMutation, CorrelationKeyRequired: true, ReasonCode: "live_validation.fake_duplicate_retry"}
	case FakeOutcomeTimeoutAfterSubmit, FakeOutcomeSubmitUnknown:
		return FakeOutcomeResult{Outcome: LedgerOutcomeOperatorActionNeeded, AmbiguousCommit: true, AutomaticRetryAllowed: false, CorrelationKeyRequired: true, ReasonCode: "live_validation.fake_ambiguous_commit"}
	default:
		return FakeOutcomeResult{Outcome: LedgerOutcomeDenied, ReasonCode: "live_validation.fake_outcome_unknown"}
	}
}
