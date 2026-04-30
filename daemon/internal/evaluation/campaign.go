package evaluation

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrEvaluationCampaignTransitionInvalid = errors.New("evaluation campaign transition invalid")
	ErrEvaluationCampaignSelectionInvalid  = errors.New("evaluation campaign selection invalid")
)

type CampaignSourceSelection struct {
	SourceType       ProductResourceKind
	SourceID         string
	TenantID         string
	SourceSnapshot   map[string]any
	SelectionReason  string
	SuppressionState SuppressionState
	RetentionState   RetentionState
	ReviewState      ProductLifecycleStatus
}

type CreateCampaignInput struct {
	CampaignID       string
	TenantID         string
	DisplayName      string
	ScopeSummary     string
	StartedBy        string
	IdempotencyKey   string
	SourceSelections []CampaignSourceSelection
	StartImmediately bool
}

type CampaignTransition string

const (
	CampaignTransitionStart    CampaignTransition = "start"
	CampaignTransitionComplete CampaignTransition = "complete"
	CampaignTransitionPublish  CampaignTransition = "publish"
	CampaignTransitionCancel   CampaignTransition = "cancel"
	CampaignTransitionFail     CampaignTransition = "fail"
)

func CreateReplayCampaign(input CreateCampaignInput, now time.Time) (ReplayCampaign, []CampaignItem, error) {
	if err := ValidateTenantScopedProductRequest(input.TenantID); err != nil {
		return ReplayCampaign{}, nil, err
	}
	if strings.TrimSpace(input.DisplayName) == "" {
		return ReplayCampaign{}, nil, ErrEvaluationCampaignSelectionInvalid
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	campaignID := strings.TrimSpace(input.CampaignID)
	if campaignID == "" {
		campaignID = "campaign_" + strings.ReplaceAll(strings.ToLower(strings.TrimSpace(input.DisplayName)), " ", "_")
	}
	status := ProductStatusDraft
	var startedAt *time.Time
	if input.StartImmediately {
		status = ProductStatusQueued
		value := now.UTC()
		startedAt = &value
	}
	campaign := ReplayCampaign{
		CampaignID:     campaignID,
		TenantID:       strings.TrimSpace(input.TenantID),
		DisplayName:    strings.TrimSpace(input.DisplayName),
		Status:         status,
		ScopeSummary:   strings.TrimSpace(input.ScopeSummary),
		StartedBy:      strings.TrimSpace(input.StartedBy),
		CreatedAt:      now.UTC(),
		StartedAt:      startedAt,
		RetentionState: RetentionStateActive,
		IdempotencyKey: strings.TrimSpace(input.IdempotencyKey),
	}
	items := make([]CampaignItem, 0, len(input.SourceSelections))
	for idx, selection := range input.SourceSelections {
		item, err := CampaignItemFromSelection(campaign, selection, idx+1, now)
		if err != nil {
			return ReplayCampaign{}, nil, err
		}
		items = append(items, item)
	}
	return campaign, items, nil
}

func CampaignItemFromSelection(campaign ReplayCampaign, selection CampaignSourceSelection, ordinal int, now time.Time) (CampaignItem, error) {
	if strings.TrimSpace(selection.TenantID) != "" && strings.TrimSpace(selection.TenantID) != strings.TrimSpace(campaign.TenantID) {
		return CampaignItem{}, ErrEvaluationProductCrossTenantSource
	}
	if selection.SourceType == "" || strings.TrimSpace(selection.SourceID) == "" {
		return CampaignItem{}, ErrEvaluationCampaignSelectionInvalid
	}
	if selection.SuppressionState == SuppressionStateSuppressed || selection.RetentionState == RetentionStateExpired || selection.RetentionState == RetentionStateDeleted || selection.RetentionState == RetentionStateTombstone {
		return CampaignItem{}, ErrEvaluationCampaignSelectionInvalid
	}
	if selection.SourceType == ProductResourceProductFixture && selection.ReviewState != ProductStatusApproved {
		return CampaignItem{}, ErrEvaluationCampaignSelectionInvalid
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	itemID := fmt.Sprintf("%s_item_%03d", campaign.CampaignID, ordinal)
	return CampaignItem{
		CampaignItemID:       itemID,
		CampaignID:           campaign.CampaignID,
		TenantID:             campaign.TenantID,
		SourceType:           selection.SourceType,
		SourceID:             strings.TrimSpace(selection.SourceID),
		SourceSnapshot:       clonePayload(selection.SourceSnapshot),
		SelectionReason:      strings.TrimSpace(selection.SelectionReason),
		SuppressionCheckedAt: now.UTC(),
		CreatedAt:            now.UTC(),
	}, nil
}

func TransitionReplayCampaign(campaign ReplayCampaign, transition CampaignTransition, now time.Time) (ReplayCampaign, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	switch transition {
	case CampaignTransitionStart:
		if campaign.Status != ProductStatusDraft && campaign.Status != ProductStatusQueued {
			return ReplayCampaign{}, ErrEvaluationCampaignTransitionInvalid
		}
		campaign.Status = ProductStatusRunning
		value := now.UTC()
		campaign.StartedAt = &value
	case CampaignTransitionComplete:
		if campaign.Status != ProductStatusRunning {
			return ReplayCampaign{}, ErrEvaluationCampaignTransitionInvalid
		}
		campaign.Status = ProductStatusCompleted
		value := now.UTC()
		campaign.CompletedAt = &value
	case CampaignTransitionPublish:
		if campaign.Status != ProductStatusCompleted {
			return ReplayCampaign{}, ErrEvaluationCampaignTransitionInvalid
		}
		campaign.Status = ProductStatusPublished
		value := now.UTC()
		campaign.PublishedAt = &value
	case CampaignTransitionCancel:
		if campaign.Status != ProductStatusDraft && campaign.Status != ProductStatusQueued && campaign.Status != ProductStatusRunning {
			return ReplayCampaign{}, ErrEvaluationCampaignTransitionInvalid
		}
		campaign.Status = ProductStatusCancelled
	case CampaignTransitionFail:
		if campaign.Status != ProductStatusQueued && campaign.Status != ProductStatusRunning {
			return ReplayCampaign{}, ErrEvaluationCampaignTransitionInvalid
		}
		campaign.Status = ProductStatusFailed
		value := now.UTC()
		campaign.CompletedAt = &value
	default:
		return ReplayCampaign{}, ErrEvaluationCampaignTransitionInvalid
	}
	return campaign, nil
}

func CampaignIdempotencyScope(campaign ReplayCampaign) string {
	if strings.TrimSpace(campaign.TenantID) == "" || strings.TrimSpace(campaign.IdempotencyKey) == "" {
		return ""
	}
	return strings.TrimSpace(campaign.TenantID) + ":" + strings.TrimSpace(campaign.IdempotencyKey)
}
