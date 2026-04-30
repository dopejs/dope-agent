package evaluation

import (
	"errors"
	"strings"
	"time"
)

var ErrEvaluationToolCallInspectionEvidenceRequired = errors.New("evaluation tool-call inspection evidence required")

const (
	InspectionMatched                      = "matched"
	InspectionDrifted                      = "drifted"
	InspectionFailed                       = "failed"
	InspectionUnsupported                  = "unsupported"
	InspectionMissingOriginalEvidence      = "missing_original_evidence"
	InspectionMissingReplayEvidence        = "missing_replay_evidence"
	InspectionLiveValidationDenied         = "live_validation_denied"
	InspectionLiveValidationAborted        = "live_validation_aborted"
	InspectionLiveValidationFailed         = "live_validation_failed"
	InspectionLiveValidationOperatorAction = "live_validation_operator_action_needed"
	InspectionLiveValidationCompleted      = "live_validation_completed"
)

type ToolCallInspectionInput struct {
	InspectionID             string
	TenantID                 string
	CampaignID               string
	CampaignItemID           string
	ToolCallRef              string
	OriginalEvidenceRef      string
	NonLiveReplayEvidenceRef string
	LiveValidationLedgerRefs []string
	Unsupported              bool
	Failed                   bool
	Drifted                  bool
	LiveValidationOutcome    string
	DiffSummary              string
	RedactionStatus          RedactionStatus
}

func BuildToolCallInspection(input ToolCallInspectionInput, now time.Time) (ToolCallInspection, error) {
	if err := ValidateTenantScopedProductRequest(input.TenantID); err != nil {
		return ToolCallInspection{}, err
	}
	if input.CampaignID == "" || input.CampaignItemID == "" || input.ToolCallRef == "" {
		return ToolCallInspection{}, ErrEvaluationToolCallInspectionEvidenceRequired
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	inspectionID := input.InspectionID
	if inspectionID == "" {
		inspectionID = "inspection_" + strings.NewReplacer(":", "_", "/", "_").Replace(input.ToolCallRef)
	}
	return ToolCallInspection{
		InspectionID:             inspectionID,
		TenantID:                 input.TenantID,
		CampaignID:               input.CampaignID,
		CampaignItemID:           input.CampaignItemID,
		ToolCallRef:              input.ToolCallRef,
		OriginalEvidenceRef:      input.OriginalEvidenceRef,
		NonLiveReplayEvidenceRef: input.NonLiveReplayEvidenceRef,
		LiveValidationLedgerRefs: append([]string(nil), input.LiveValidationLedgerRefs...),
		Classification:           ClassifyToolCallInspection(input),
		DiffSummary:              input.DiffSummary,
		RedactionStatus:          redactionStatusDefault(input.RedactionStatus),
		RetentionState:           RetentionStateActive,
		CreatedAt:                now.UTC(),
		UpdatedAt:                now.UTC(),
	}, nil
}

func ClassifyToolCallInspection(input ToolCallInspectionInput) string {
	if input.Unsupported {
		return InspectionUnsupported
	}
	if input.OriginalEvidenceRef == "" {
		return InspectionMissingOriginalEvidence
	}
	if input.NonLiveReplayEvidenceRef == "" {
		return InspectionMissingReplayEvidence
	}
	switch input.LiveValidationOutcome {
	case "denied":
		return InspectionLiveValidationDenied
	case "aborted":
		return InspectionLiveValidationAborted
	case "failed":
		return InspectionLiveValidationFailed
	case "operator_action_needed":
		return InspectionLiveValidationOperatorAction
	case "completed":
		return InspectionLiveValidationCompleted
	}
	if input.Failed {
		return InspectionFailed
	}
	if input.Drifted {
		return InspectionDrifted
	}
	return InspectionMatched
}
