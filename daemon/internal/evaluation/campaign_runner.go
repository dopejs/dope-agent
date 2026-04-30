package evaluation

import "time"

type CampaignRunnerInput struct {
	Campaign          ReplayCampaign
	Items             []CampaignItem
	LiveValidationIDs map[string][]string
	Now               time.Time
}

type CampaignRunnerPlan struct {
	CampaignID string                     `json:"campaignId"`
	Launches   []CampaignReplayLaunchPlan `json:"launches"`
	Groups     []CampaignAttemptGroup     `json:"groups"`
}

func BuildCampaignRunnerPlan(input CampaignRunnerInput) (CampaignRunnerPlan, error) {
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	launches := BuildCampaignReplayLaunchPlan(input.Campaign, input.Items)
	groups := make([]CampaignAttemptGroup, 0, len(input.Items))
	for _, item := range input.Items {
		group, err := BuildCampaignAttemptGroup(CampaignAttemptAggregationInput{
			CampaignID:        input.Campaign.CampaignID,
			CampaignItemID:    item.CampaignItemID,
			TenantID:          input.Campaign.TenantID,
			ReplayAttemptIDs:  []string{"attempt_" + item.CampaignItemID},
			LiveValidationIDs: input.LiveValidationIDs[item.CampaignItemID],
		}, now)
		if err != nil {
			return CampaignRunnerPlan{}, err
		}
		groups = append(groups, group)
	}
	return CampaignRunnerPlan{
		CampaignID: input.Campaign.CampaignID,
		Launches:   launches,
		Groups:     groups,
	}, nil
}
