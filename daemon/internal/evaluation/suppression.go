package evaluation

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrEvaluationProductSuppressionTargetRequired = errors.New("evaluation product suppression target required")

type CreateSuppressionInput struct {
	SuppressionID   string
	TenantID        string
	TargetKind      ProductResourceKind
	TargetID        string
	TargetSourceRef string
	ReasonCode      string
	Reason          string
	CreatedBy       string
	ExpiresAt       *time.Time
}

func NewSuppressionRecord(input CreateSuppressionInput, now time.Time) (SuppressionRecord, error) {
	if err := ValidateTenantScopedProductRequest(input.TenantID); err != nil {
		return SuppressionRecord{}, err
	}
	if input.TargetKind == "" || (strings.TrimSpace(input.TargetID) == "" && strings.TrimSpace(input.TargetSourceRef) == "") {
		return SuppressionRecord{}, ErrEvaluationProductSuppressionTargetRequired
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	suppressionID := strings.TrimSpace(input.SuppressionID)
	if suppressionID == "" {
		target := strings.TrimSpace(input.TargetID)
		if target == "" {
			target = strings.NewReplacer(":", "_", "/", "_").Replace(strings.TrimSpace(input.TargetSourceRef))
		}
		suppressionID = fmt.Sprintf("suppression_%s_%s", input.TargetKind, target)
	}
	reasonCode := strings.TrimSpace(input.ReasonCode)
	if reasonCode == "" {
		reasonCode = "operator_hidden"
	}
	return SuppressionRecord{
		SuppressionID:   suppressionID,
		TenantID:        strings.TrimSpace(input.TenantID),
		TargetKind:      input.TargetKind,
		TargetID:        strings.TrimSpace(input.TargetID),
		TargetSourceRef: strings.TrimSpace(input.TargetSourceRef),
		ReasonCode:      reasonCode,
		Reason:          strings.TrimSpace(input.Reason),
		CreatedBy:       strings.TrimSpace(input.CreatedBy),
		CreatedAt:       now.UTC(),
		ExpiresAt:       input.ExpiresAt,
		Active:          true,
	}, nil
}

func FindActiveSuppression(records []SuppressionRecord, tenantID, suppressionID string, now time.Time) (SuppressionRecord, bool) {
	for _, record := range records {
		if record.SuppressionID == suppressionID && activeSuppressionForTenant(record, tenantID, now) {
			return record, true
		}
	}
	return SuppressionRecord{}, false
}

func RevokeSuppressionRecord(record SuppressionRecord, revokedAt time.Time) SuppressionRecord {
	if revokedAt.IsZero() {
		revokedAt = time.Now().UTC()
	}
	record.Active = false
	record.ExpiresAt = &revokedAt
	return record
}

func FilterSuppressedCandidates(candidates []DiscoveredCandidate, records []SuppressionRecord, now time.Time) []DiscoveredCandidate {
	out := make([]DiscoveredCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if SuppressionApplies(candidate, records, now) {
			continue
		}
		out = append(out, candidate)
	}
	return out
}

func SuppressionApplies(candidate DiscoveredCandidate, records []SuppressionRecord, now time.Time) bool {
	for _, record := range records {
		if !activeSuppressionForTenant(record, candidate.TenantID, now) {
			continue
		}
		if record.TargetKind == ProductResourceDiscoveredCandidate && record.TargetID == candidate.DiscoveredCandidateID {
			return true
		}
		if record.TargetSourceRef != "" && record.TargetSourceRef == candidateSourceRef(candidate) {
			return true
		}
	}
	return false
}

func activeSuppressionForTenant(record SuppressionRecord, tenantID string, now time.Time) bool {
	if !record.Active {
		return false
	}
	if strings.TrimSpace(record.TenantID) != strings.TrimSpace(tenantID) {
		return false
	}
	if record.ExpiresAt != nil && !record.ExpiresAt.After(now) {
		return false
	}
	return true
}

func candidateSourceRef(candidate DiscoveredCandidate) string {
	if candidate.SourceKind == "" || candidate.SourceID == "" {
		return ""
	}
	return fmt.Sprintf("%s:%s", candidate.SourceKind, candidate.SourceID)
}
