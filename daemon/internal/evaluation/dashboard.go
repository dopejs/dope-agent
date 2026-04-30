package evaluation

import (
	"fmt"
	"sort"
	"time"
)

type DashboardProjectionInput struct {
	ProjectionID  string
	TenantID      string
	WindowStart   time.Time
	WindowEnd     time.Time
	Campaigns     []ReplayCampaign
	Candidates    []DiscoveredCandidate
	Fixtures      []ProductManagedFixture
	AttemptGroups []CampaignAttemptGroup
	GeneratedAt   time.Time
}

func BuildDashboardProjection(input DashboardProjectionInput) (DashboardProjection, error) {
	if err := ValidateTenantScopedProductRequest(input.TenantID); err != nil {
		return DashboardProjection{}, err
	}
	generatedAt := input.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	projectionID := input.ProjectionID
	if projectionID == "" {
		projectionID = fmt.Sprintf("dashboard_%d", generatedAt.UnixNano())
	}
	projection := DashboardProjection{
		ProjectionID:                projectionID,
		TenantID:                    input.TenantID,
		WindowStart:                 input.WindowStart,
		WindowEnd:                   input.WindowEnd,
		CampaignStatusCounts:        map[string]int{},
		DriftSummary:                map[string]int{"total": 0},
		FailureSummary:              map[string]int{"total": 0},
		UnsupportedSummary:          map[string]int{"total": 0},
		OperatorActionNeededSummary: map[string]int{"total": 0},
		LiveValidationSummary:       map[string]int{"linked": 0},
		CandidateSummary:            map[string]int{},
		FixtureSummary:              map[string]int{},
		GeneratedAt:                 generatedAt.UTC(),
		RetentionState:              RetentionStateActive,
	}
	for _, campaign := range input.Campaigns {
		if campaign.TenantID == input.TenantID {
			projection.CampaignStatusCounts[string(campaign.Status)]++
		}
	}
	for _, candidate := range input.Candidates {
		if candidate.TenantID == input.TenantID {
			projection.CandidateSummary[string(candidate.RetentionState)]++
			projection.CandidateSummary[string(candidate.SuppressionState)]++
		}
	}
	for _, fixture := range input.Fixtures {
		if fixture.TenantID == input.TenantID {
			projection.FixtureSummary[string(fixture.ReviewState)]++
			projection.FixtureSummary[string(fixture.RetentionState)]++
		}
	}
	for _, group := range input.AttemptGroups {
		if group.TenantID == input.TenantID {
			projection.DriftSummary["total"] += group.DriftCount
			projection.FailureSummary["total"] += group.FailureCount
			projection.UnsupportedSummary["total"] += group.UnsupportedCount
			projection.OperatorActionNeededSummary["total"] += group.OperatorActionNeededCount
			if len(group.LiveValidationIDs) > 0 {
				projection.LiveValidationSummary["linked"] += len(group.LiveValidationIDs)
			}
		}
	}
	return projection, nil
}

func PageDashboardProjections(items []DashboardProjection, cursor string, limit int) ([]DashboardProjection, string) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].GeneratedAt.Equal(items[j].GeneratedAt) {
			return items[i].ProjectionID > items[j].ProjectionID
		}
		return items[i].GeneratedAt.After(items[j].GeneratedAt)
	})
	if cursor != "" {
		filtered := make([]DashboardProjection, 0, len(items))
		for _, item := range items {
			if item.ProjectionID < cursor {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	limit = NormalizeProductLimit(limit)
	if len(items) <= limit {
		return append([]DashboardProjection(nil), items...), ""
	}
	return append([]DashboardProjection(nil), items[:limit]...), items[limit-1].ProjectionID
}
