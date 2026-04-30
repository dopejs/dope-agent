package evaluation

import (
	"errors"
	"math"
	"strings"
	"time"
)

var ErrEvaluationProductSourceRequired = errors.New("evaluation product source required")

type CandidateScoringInput struct {
	TenantID              string
	DiscoveryRunID        string
	SourceKind            SourceKind
	SourceID              string
	SourceRefs            []SourceRef
	FailureRecurrence     int
	DriftSignal           bool
	ToolCallClass         string
	LiveValidationOutcome string
	WorkflowCoverage      int
	OperatorRelevance     int
	ObservedAt            time.Time
	RedactionStatus       RedactionStatus
	ReadinessStatus       ReadinessStatus
}

func BuildDiscoveredCandidateFromSignals(input CandidateScoringInput, now time.Time) (DiscoveredCandidate, error) {
	if err := ValidateTenantScopedProductRequest(input.TenantID); err != nil {
		return DiscoveredCandidate{}, err
	}
	if strings.TrimSpace(input.DiscoveryRunID) == "" || input.SourceKind == "" || strings.TrimSpace(input.SourceID) == "" {
		return DiscoveredCandidate{}, ErrEvaluationProductSourceRequired
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	score := candidateDiscoveryScore(input, now)
	return DiscoveredCandidate{
		DiscoveredCandidateID: "candidate_" + strings.ReplaceAll(string(input.SourceKind)+"_"+strings.TrimSpace(input.SourceID), ":", "_"),
		TenantID:              strings.TrimSpace(input.TenantID),
		DiscoveryRunID:        strings.TrimSpace(input.DiscoveryRunID),
		SourceKind:            input.SourceKind,
		SourceID:              strings.TrimSpace(input.SourceID),
		SourceRefs:            append([]SourceRef(nil), input.SourceRefs...),
		Score:                 score,
		ScoreBand:             scoreBandFor(score),
		ExplanationFields:     candidateExplanationFields(input, now),
		RedactionStatus:       redactionStatusDefault(input.RedactionStatus),
		ReadinessStatus:       readinessStatusDefault(input.ReadinessStatus, input.RedactionStatus),
		SuppressionState:      SuppressionStateNone,
		RetentionState:        RetentionStateActive,
		CreatedAt:             now.UTC(),
		UpdatedAt:             now.UTC(),
	}, nil
}

func candidateDiscoveryScore(input CandidateScoringInput, now time.Time) float64 {
	score := 0.20
	score += math.Min(float64(maxInt(input.FailureRecurrence, 0)), 5) * 0.08
	if input.DriftSignal {
		score += 0.20
	}
	if strings.TrimSpace(input.ToolCallClass) != "" {
		score += 0.10
	}
	switch strings.TrimSpace(input.LiveValidationOutcome) {
	case "operator_action_needed", "failed", "denied", "aborted":
		score += 0.15
	case "completed":
		score += 0.05
	}
	if input.WorkflowCoverage >= 2 {
		score += 0.10
	}
	score += math.Min(float64(maxInt(input.OperatorRelevance, 0)), 3) * 0.05
	if !input.ObservedAt.IsZero() {
		age := now.Sub(input.ObservedAt)
		if age <= 24*time.Hour {
			score += 0.10
		} else if age <= 7*24*time.Hour {
			score += 0.05
		}
	}
	if score > 1 {
		score = 1
	}
	return math.Round(score*100) / 100
}

func candidateExplanationFields(input CandidateScoringInput, now time.Time) map[string]any {
	fields := map[string]any{
		"failureRecurrence": input.FailureRecurrence,
		"driftSignal":       input.DriftSignal,
		"workflowCoverage":  input.WorkflowCoverage,
		"operatorRelevance": input.OperatorRelevance,
	}
	if strings.TrimSpace(input.ToolCallClass) != "" {
		fields["toolCallClass"] = strings.TrimSpace(input.ToolCallClass)
	}
	if strings.TrimSpace(input.LiveValidationOutcome) != "" {
		fields["liveValidationOutcome"] = strings.TrimSpace(input.LiveValidationOutcome)
	}
	if !input.ObservedAt.IsZero() {
		fields["observedAt"] = input.ObservedAt.UTC().Format(time.RFC3339Nano)
		fields["ageHours"] = int(now.Sub(input.ObservedAt).Hours())
	}
	return fields
}

func scoreBandFor(score float64) ScoreBand {
	switch {
	case score >= 0.75:
		return ScoreBandHigh
	case score >= 0.40:
		return ScoreBandMedium
	default:
		return ScoreBandLow
	}
}

func redactionStatusDefault(status RedactionStatus) RedactionStatus {
	if status == "" {
		return RedactionStatusClean
	}
	return status
}

func readinessStatusDefault(status ReadinessStatus, redactionStatus RedactionStatus) ReadinessStatus {
	if redactionStatus == RedactionStatusFailed {
		return ReadinessBlocked
	}
	if status == "" {
		return ReadinessFullyReplayable
	}
	return status
}

func maxInt(value, floor int) int {
	if value < floor {
		return floor
	}
	return value
}
