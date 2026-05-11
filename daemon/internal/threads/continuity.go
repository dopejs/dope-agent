package threads

import (
	"errors"
	"sort"
	"strings"
	"time"
)

const (
	DefaultContinuityMaxPriorTurns  = 12
	DefaultContinuityActiveDays     = 30
	DefaultContinuityWindowPolicyID = "default_recent_12_30d"
	ContinuityOrderDaemonSequence   = "daemon_acceptance_sequence"
)

type ContinuityMode string

const (
	ContinuityModeAuto     ContinuityMode = "auto"
	ContinuityModeDisabled ContinuityMode = "disabled"
)

type ContinuityRole string

const (
	ContinuityRoleUser      ContinuityRole = "user"
	ContinuityRoleAssistant ContinuityRole = "assistant"
)

type ContinuityStatus string

const (
	ContinuityStatusApplied  ContinuityStatus = "applied"
	ContinuityStatusEmpty    ContinuityStatus = "empty"
	ContinuityStatusDisabled ContinuityStatus = "disabled"
	ContinuityStatusBlocked  ContinuityStatus = "blocked"
	ContinuityStatusPartial  ContinuityStatus = "partial"
	ContinuityStatusFailed   ContinuityStatus = "failed"
)

type ContinuityDecision string

const (
	ContinuityDecisionIncluded ContinuityDecision = "included"
	ContinuityDecisionExcluded ContinuityDecision = "excluded"
)

type ContinuityItemKind string

const (
	ContinuityItemTurn            ContinuityItemKind = "turn"
	ContinuityItemArtifactExcerpt ContinuityItemKind = "artifact_excerpt"
)

type ContinuityReason string

const (
	ContinuityReasonIncludedRecent        ContinuityReason = "included_recent"
	ContinuityReasonNoEligibleTurns       ContinuityReason = "no_eligible_turns"
	ContinuityReasonOverLimit             ContinuityReason = "over_limit"
	ContinuityReasonTooOld                ContinuityReason = "too_old"
	ContinuityReasonResetBoundary         ContinuityReason = "reset_boundary"
	ContinuityReasonLifecycleBlocked      ContinuityReason = "lifecycle_blocked"
	ContinuityReasonSourceMismatch        ContinuityReason = "source_mismatch"
	ContinuityReasonPermissionDenied      ContinuityReason = "permission_denied"
	ContinuityReasonRedactionFailed       ContinuityReason = "redaction_failed"
	ContinuityReasonRetentionExpired      ContinuityReason = "retention_expired"
	ContinuityReasonDuplicateSource       ContinuityReason = "duplicate_source_event"
	ContinuityReasonIncompleteEvidence    ContinuityReason = "incomplete_evidence"
	ContinuityReasonUnsupportedSource     ContinuityReason = "unsupported_source"
	ContinuityReasonArtifactReference     ContinuityReason = "artifact_reference_only"
	ContinuityReasonContinuityDisabled    ContinuityReason = "continuity_disabled"
	ContinuityReasonContinuityUnavailable ContinuityReason = "continuity_unavailable"
)

type RuntimeArtifactExcerpt struct {
	ArtifactExcerptID  string          `json:"artifactExcerptId"`
	TenantID           string          `json:"tenantId,omitempty"`
	ThreadID           string          `json:"threadId,omitempty"`
	SessionSegmentID   string          `json:"sessionSegmentId,omitempty"`
	ContinuityTurnID   string          `json:"continuityTurnId,omitempty"`
	ResourceKind       string          `json:"resourceKind"`
	ResourceID         string          `json:"resourceId"`
	ExcerptText        string          `json:"excerptText,omitempty"`
	ExcerptSource      string          `json:"excerptSource,omitempty"`
	CreatedAt          time.Time       `json:"createdAt"`
	RetentionExpiresAt time.Time       `json:"retentionExpiresAt,omitempty"`
	RedactionStatus    RedactionStatus `json:"redactionStatus"`
}

type ContinuityTurn struct {
	ContinuityTurnID       string                   `json:"continuityTurnId"`
	TenantID               string                   `json:"tenantId"`
	ThreadID               string                   `json:"threadId"`
	SessionSegmentID       string                   `json:"sessionSegmentId"`
	AcceptanceSequence     int64                    `json:"acceptanceSequence"`
	Role                   ContinuityRole           `json:"role"`
	SourceKind             SourceKind               `json:"sourceKind"`
	SourceLinkageID        string                   `json:"sourceLinkageId,omitempty"`
	SourceMessageID        string                   `json:"sourceMessageId,omitempty"`
	SourceTimestamp        *time.Time               `json:"sourceTimestamp,omitempty"`
	DispatchID             string                   `json:"dispatchId,omitempty"`
	ResponseToTurnID       string                   `json:"responseToTurnId,omitempty"`
	SafeContent            string                   `json:"safeContent,omitempty"`
	ContentRedactionStatus RedactionStatus          `json:"contentRedactionStatus"`
	ArtifactExcerptRefs    []RuntimeArtifactExcerpt `json:"artifactExcerptRefs,omitempty"`
	RecordedAt             time.Time                `json:"recordedAt"`
	RetentionExpiresAt     time.Time                `json:"retentionExpiresAt"`
	SourceEventKey         string                   `json:"sourceEventKey,omitempty"`
}

type ContinuityWindowPolicy struct {
	WindowPolicyID   string `json:"windowPolicyId"`
	MaxPriorTurns    int    `json:"maxPriorTurns"`
	ActiveWindowDays int    `json:"activeWindowDays"`
	OrderedBy        string `json:"orderedBy"`
}

type ContinuityPreview struct {
	ContinuityPreviewID string           `json:"continuityPreviewId"`
	TenantID            string           `json:"tenantId"`
	ThreadID            string           `json:"threadId"`
	SessionSegmentID    string           `json:"sessionSegmentId"`
	DispatchID          string           `json:"dispatchId,omitempty"`
	RequestTurnID       string           `json:"requestTurnId,omitempty"`
	ResponseTurnID      string           `json:"responseTurnId,omitempty"`
	WindowPolicyID      string           `json:"windowPolicyId"`
	MaxPriorTurns       int              `json:"maxPriorTurns"`
	ActiveWindowDays    int              `json:"activeWindowDays"`
	IncludedCount       int              `json:"includedCount"`
	ExcludedCount       int              `json:"excludedCount"`
	ContinuityApplied   bool             `json:"continuityApplied"`
	Status              ContinuityStatus `json:"status"`
	FailureClass        string           `json:"failureClass,omitempty"`
	AssemblyStartedAt   time.Time        `json:"assemblyStartedAt"`
	AssemblyCompletedAt time.Time        `json:"assemblyCompletedAt"`
	AssemblyDurationMs  int64            `json:"assemblyDurationMs"`
	RetentionExpiresAt  time.Time        `json:"retentionExpiresAt"`
	RedactionStatus     RedactionStatus  `json:"redactionStatus"`
}

type ContinuityPreviewItem struct {
	PreviewItemID       string             `json:"previewItemId"`
	ContinuityPreviewID string             `json:"continuityPreviewId"`
	TenantID            string             `json:"tenantId"`
	ThreadID            string             `json:"threadId"`
	ItemKind            ContinuityItemKind `json:"itemKind"`
	ContinuityTurnID    string             `json:"continuityTurnId,omitempty"`
	Role                ContinuityRole     `json:"role,omitempty"`
	ArtifactRef         string             `json:"artifactRef,omitempty"`
	ArtifactExcerptID   string             `json:"artifactExcerptId,omitempty"`
	Decision            ContinuityDecision `json:"decision"`
	ReasonCode          ContinuityReason   `json:"reasonCode"`
	AcceptanceSequence  int64              `json:"acceptanceSequence,omitempty"`
	SourceTimestamp     *time.Time         `json:"sourceTimestamp,omitempty"`
	SafeSummary         string             `json:"safeSummary,omitempty"`
	RedactionStatus     RedactionStatus    `json:"redactionStatus"`
	ItemOrder           int                `json:"itemOrder"`
}

type ContinuityPreviewDetail struct {
	Preview ContinuityPreview       `json:"preview"`
	Items   []ContinuityPreviewItem `json:"items"`
}

func DefaultContinuityPolicy() ContinuityWindowPolicy {
	return ContinuityWindowPolicy{
		WindowPolicyID:   DefaultContinuityWindowPolicyID,
		MaxPriorTurns:    DefaultContinuityMaxPriorTurns,
		ActiveWindowDays: DefaultContinuityActiveDays,
		OrderedBy:        ContinuityOrderDaemonSequence,
	}
}

func NormalizeContinuityMode(mode ContinuityMode) ContinuityMode {
	if strings.TrimSpace(string(mode)) == "" {
		return ContinuityModeAuto
	}
	if mode == ContinuityModeDisabled {
		return ContinuityModeDisabled
	}
	return ContinuityModeAuto
}

func ValidateContinuityTurn(turn ContinuityTurn) error {
	if strings.TrimSpace(turn.TenantID) == "" || strings.TrimSpace(turn.ThreadID) == "" || strings.TrimSpace(turn.SessionSegmentID) == "" {
		return errors.New("continuity turn requires tenant, thread, and session segment")
	}
	if turn.Role != ContinuityRoleUser && turn.Role != ContinuityRoleAssistant {
		return errors.New("continuity turn role is invalid")
	}
	if turn.ContentRedactionStatus == "" {
		return errors.New("continuity turn redaction status is required")
	}
	return nil
}

func EligibleContinuityTurns(turns []ContinuityTurn, policy ContinuityWindowPolicy, now time.Time) ([]ContinuityTurn, []ContinuityPreviewItem) {
	if policy.MaxPriorTurns <= 0 {
		policy = DefaultContinuityPolicy()
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	cutoff := now.AddDate(0, 0, -policy.ActiveWindowDays)
	eligible := make([]ContinuityTurn, 0, len(turns))
	excluded := []ContinuityPreviewItem{}
	for _, turn := range turns {
		reason := ContinuityReason("")
		switch {
		case turn.RetentionExpiresAt.Before(now):
			reason = ContinuityReasonRetentionExpired
		case turn.RecordedAt.Before(cutoff):
			reason = ContinuityReasonTooOld
		case turn.ContentRedactionStatus == RedactionStatusRedactionFailed || turn.ContentRedactionStatus == RedactionStatusSuppressed:
			reason = ContinuityReasonRedactionFailed
		}
		if reason != "" {
			excluded = append(excluded, PreviewItemForTurn(turn, ContinuityDecisionExcluded, reason, len(excluded)))
			continue
		}
		eligible = append(eligible, turn)
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		return eligible[i].AcceptanceSequence < eligible[j].AcceptanceSequence
	})
	if len(eligible) > policy.MaxPriorTurns {
		overLimit := eligible[:len(eligible)-policy.MaxPriorTurns]
		for _, turn := range overLimit {
			excluded = append(excluded, PreviewItemForTurn(turn, ContinuityDecisionExcluded, ContinuityReasonOverLimit, len(excluded)))
		}
		eligible = eligible[len(eligible)-policy.MaxPriorTurns:]
	}
	return eligible, excluded
}

func PreviewItemForTurn(turn ContinuityTurn, decision ContinuityDecision, reason ContinuityReason, order int) ContinuityPreviewItem {
	summary := SafeSummary(turn.SafeContent, turn.ContentRedactionStatus == RedactionStatusRedacted)
	return ContinuityPreviewItem{
		TenantID:           turn.TenantID,
		ThreadID:           turn.ThreadID,
		ItemKind:           ContinuityItemTurn,
		ContinuityTurnID:   turn.ContinuityTurnID,
		Role:               turn.Role,
		Decision:           decision,
		ReasonCode:         reason,
		AcceptanceSequence: turn.AcceptanceSequence,
		SourceTimestamp:    turn.SourceTimestamp,
		SafeSummary:        summary.Text,
		RedactionStatus:    summary.Status,
		ItemOrder:          order,
	}
}

func ResetBoundaryPreviewItems(turns []ContinuityTurn, startOrder int) []ContinuityPreviewItem {
	items := make([]ContinuityPreviewItem, 0, len(turns))
	for _, turn := range turns {
		items = append(items, PreviewItemForTurn(turn, ContinuityDecisionExcluded, ContinuityReasonResetBoundary, startOrder+len(items)))
	}
	return items
}

func PreviewItemsForArtifactExcerpts(turn ContinuityTurn, startOrder int, now time.Time) []ContinuityPreviewItem {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	items := make([]ContinuityPreviewItem, 0, len(turn.ArtifactExcerptRefs))
	for _, excerpt := range turn.ArtifactExcerptRefs {
		item := ContinuityPreviewItem{
			TenantID:           turn.TenantID,
			ThreadID:           turn.ThreadID,
			ItemKind:           ContinuityItemArtifactExcerpt,
			ContinuityTurnID:   turn.ContinuityTurnID,
			ArtifactRef:        strings.TrimSpace(excerpt.ResourceKind + "/" + excerpt.ResourceID),
			ArtifactExcerptID:  excerpt.ArtifactExcerptID,
			Decision:           ContinuityDecisionExcluded,
			ReasonCode:         ContinuityReasonArtifactReference,
			AcceptanceSequence: turn.AcceptanceSequence,
			SourceTimestamp:    turn.SourceTimestamp,
			SafeSummary:        "suppressed",
			RedactionStatus:    RedactionStatusSuppressed,
			ItemOrder:          startOrder + len(items),
		}
		switch {
		case !excerpt.RetentionExpiresAt.IsZero() && excerpt.RetentionExpiresAt.Before(now):
			item.ReasonCode = ContinuityReasonRetentionExpired
		case excerpt.RedactionStatus != RedactionStatusRedacted:
			item.ReasonCode = ContinuityReasonRedactionFailed
		case strings.TrimSpace(excerpt.ExcerptText) == "":
			item.ReasonCode = ContinuityReasonIncompleteEvidence
		default:
			summary := SafeContinuityContent(excerpt.ExcerptText)
			if summary.Status != RedactionStatusRedacted {
				item.ReasonCode = ContinuityReasonRedactionFailed
			} else {
				item.Decision = ContinuityDecisionIncluded
				item.ReasonCode = ContinuityReasonIncludedRecent
			}
			item.SafeSummary = summary.Text
			item.RedactionStatus = summary.Status
		}
		items = append(items, item)
	}
	return items
}
