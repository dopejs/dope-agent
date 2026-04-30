package evaluation

import (
	"fmt"
	"time"
)

type CampaignAttemptAggregationInput struct {
	CampaignID                string
	CampaignItemID            string
	TenantID                  string
	ReplayAttemptIDs          []string
	ComparisonIDs             []string
	LiveValidationIDs         []string
	DriftCount                int
	FailureCount              int
	UnsupportedCount          int
	OperatorActionNeededCount int
	RedactionStatus           RedactionStatus
}

type CampaignReplayLaunchPlan struct {
	CampaignID       string   `json:"campaignId"`
	CampaignItemID   string   `json:"campaignItemId"`
	ReplayAttemptIDs []string `json:"replayAttemptIds"`
	Mode             string   `json:"mode"`
}

func BuildCampaignAttemptGroup(input CampaignAttemptAggregationInput, now time.Time) (CampaignAttemptGroup, error) {
	if err := ValidateTenantScopedProductRequest(input.TenantID); err != nil {
		return CampaignAttemptGroup{}, err
	}
	if input.CampaignID == "" || input.CampaignItemID == "" {
		return CampaignAttemptGroup{}, ErrEvaluationCampaignSelectionInvalid
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	status := ProductStatusCompleted
	if input.RedactionStatus == RedactionStatusFailed || input.FailureCount > 0 {
		status = ProductStatusFailed
	}
	return CampaignAttemptGroup{
		AttemptGroupID:            fmt.Sprintf("attempt_group_%s_%s", input.CampaignID, input.CampaignItemID),
		CampaignID:                input.CampaignID,
		CampaignItemID:            input.CampaignItemID,
		TenantID:                  input.TenantID,
		ReplayAttemptIDs:          append([]string(nil), input.ReplayAttemptIDs...),
		ComparisonIDs:             append([]string(nil), input.ComparisonIDs...),
		LiveValidationIDs:         append([]string(nil), input.LiveValidationIDs...),
		Status:                    status,
		DriftCount:                input.DriftCount,
		FailureCount:              input.FailureCount,
		UnsupportedCount:          input.UnsupportedCount,
		OperatorActionNeededCount: input.OperatorActionNeededCount,
		Summary:                   campaignAttemptSummary(input),
		CreatedAt:                 now.UTC(),
		UpdatedAt:                 now.UTC(),
	}, nil
}

func BuildCampaignReplayLaunchPlan(campaign ReplayCampaign, items []CampaignItem) []CampaignReplayLaunchPlan {
	plans := make([]CampaignReplayLaunchPlan, 0, len(items))
	for _, item := range items {
		plans = append(plans, CampaignReplayLaunchPlan{
			CampaignID:       campaign.CampaignID,
			CampaignItemID:   item.CampaignItemID,
			ReplayAttemptIDs: []string{"attempt_" + item.CampaignItemID},
			Mode:             string(ReplayModeNonLive),
		})
	}
	return plans
}

func campaignAttemptSummary(input CampaignAttemptAggregationInput) string {
	return fmt.Sprintf("%d drift, %d failure, %d unsupported, %d operator-action-needed", input.DriftCount, input.FailureCount, input.UnsupportedCount, input.OperatorActionNeededCount)
}
