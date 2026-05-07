package billing

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type DenialPayload struct {
	Code            string   `json:"code"`
	ReasonCode      string   `json:"reasonCode"`
	TenantID        string   `json:"tenantId"`
	Category        Category `json:"category,omitempty"`
	OperationKey    string   `json:"operationKey"`
	PeriodStart     string   `json:"periodStart,omitempty"`
	PeriodEnd       string   `json:"periodEnd,omitempty"`
	RequestedAmount int64    `json:"requestedAmount,omitempty"`
	RemainingAmount int64    `json:"remainingAmount,omitempty"`
	Message         string   `json:"message"`
}

type DenialError struct {
	Payload DenialPayload
}

type DenialLookupRepository interface {
	QuotaDenialByID(ctx context.Context, tenantID string, denialID string) (QuotaDenial, bool, error)
}

type EvidenceReferenceRepository interface {
	ListUsageEvidenceRefs(ctx context.Context, tenantID string, operationKey string, limit int) ([]string, error)
}

func (e DenialError) Error() string {
	if e.Payload.Message != "" {
		return e.Payload.Message
	}
	return "quota denied"
}

func NewQuotaExhaustedDenial(tenantID string, category Category, operationKey string, requested, remaining int64, period QuotaPeriod) DenialError {
	definition, _ := DefinitionFor(category)
	reason := definition.DenialReasonCode
	if reason == "" {
		reason = fmt.Sprintf("quota_denied:%s_exhausted", category)
	}
	return DenialError{Payload: DenialPayload{
		Code:            "quota_denied",
		ReasonCode:      reason,
		TenantID:        tenantID,
		Category:        category,
		OperationKey:    operationKey,
		PeriodStart:     period.PeriodStart.UTC().Format("2006-01-02T15:04:05Z"),
		PeriodEnd:       period.PeriodEnd.UTC().Format("2006-01-02T15:04:05Z"),
		RequestedAmount: requested,
		RemainingAmount: remaining,
		Message:         fmt.Sprintf("Quota exhausted for %s.", category),
	}}
}

func NewQuotaStateUnavailableDenial(tenantID, operationKey string) DenialError {
	return DenialError{Payload: DenialPayload{
		Code:         "quota_denied",
		ReasonCode:   ReasonQuotaStateUnavailable,
		TenantID:     tenantID,
		OperationKey: operationKey,
		Message:      "Quota state is unavailable; hosted work cannot start.",
	}}
}

func ProjectDenialDetail(denial QuotaDenial, restriction *AbuseRestrictionSummary) QuotaDenialDetail {
	classification := ClassifyDenial(denial, restriction)
	status := statusForDenialClassification(classification)
	detail := QuotaDenialDetail{
		DenialID:          denial.DenialID,
		TenantID:          denial.TenantID,
		OperationRef:      SafeOperationRef(denial.OperationKey),
		OperationKey:      denial.OperationKey,
		GuardedEntryPoint: denial.GuardedEntryPoint,
		Category:          denial.Category,
		ReasonCode:        denial.ReasonCode,
		Classification:    classification,
		RequestedAmount:   denial.RequestedAmount,
		RemainingAmount:   denial.RemainingAmount,
		RecoveryActions:   RecoveryActionsForQuotaStatus(status, NearLimitReasonNone),
		Restriction:       restriction,
		CreatedAt:         denial.CreatedAt,
	}
	if classification == DenialClassificationAbuseRestriction && detail.Restriction == nil {
		detail.Restriction = &AbuseRestrictionSummary{
			Status:                AbuseRestrictionStatusActive,
			AffectedCategory:      denial.Category,
			RecoveryAction:        RecoveryActionContactSupport,
			VisibleReasonCode:     denial.ReasonCode,
			SupportContactAllowed: true,
		}
	}
	return detail
}

func (m *Manager) DenialDetail(ctx context.Context, tenantID string, denialID string) (QuotaDenialDetail, bool, error) {
	if m == nil || m.repo == nil {
		return QuotaDenialDetail{}, false, nil
	}
	repo, ok := m.repo.(DenialLookupRepository)
	if !ok {
		return QuotaDenialDetail{}, false, nil
	}
	denial, found, err := repo.QuotaDenialByID(ctx, tenantID, denialID)
	if err != nil || !found {
		return QuotaDenialDetail{}, found, err
	}
	restriction, err := m.restrictionForDenial(ctx, tenantID, denial)
	if err != nil {
		return QuotaDenialDetail{}, false, err
	}
	return ProjectDenialDetail(denial, restriction), true, nil
}

func (m *Manager) EvidenceExport(ctx context.Context, tenantID string, denialID string, generatedByPrincipalID string, hosted bool) (BillingEvidenceExport, bool, error) {
	detail, found, err := m.DenialDetail(ctx, tenantID, denialID)
	if err != nil || !found {
		return BillingEvidenceExport{}, found, err
	}
	dashboard, err := m.QuotaDashboard(ctx, tenantID, hosted)
	if err != nil {
		return BillingEvidenceExport{}, false, err
	}
	usageSnapshot := make([]QuotaStatusItem, 0)
	for _, section := range dashboard.Sections {
		usageSnapshot = append(usageSnapshot, section.Items...)
	}
	state := effectiveLimitStateForDenial(dashboard, detail)
	export := BuildEvidenceExport(generatedByPrincipalID, detail, usageSnapshot, state)
	export.AuditRefs = append([]string{"quota_denial:" + detail.DenialID}, export.AuditRefs...)
	if m != nil && m.repo != nil {
		if evidenceRepo, ok := m.repo.(EvidenceReferenceRepository); ok {
			refs, err := evidenceRepo.ListUsageEvidenceRefs(ctx, tenantID, detail.OperationKey, 100)
			if err != nil {
				return BillingEvidenceExport{}, false, err
			}
			export.AuditRefs = append(export.AuditRefs, refs...)
		}
	}
	return export, true, nil
}

func (m *Manager) restrictionForDenial(ctx context.Context, tenantID string, denial QuotaDenial) (*AbuseRestrictionSummary, error) {
	if m == nil || m.repo == nil || denial.Category == "" || ClassifyDenial(denial, nil) != DenialClassificationAbuseRestriction {
		return nil, nil
	}
	repo, ok := m.repo.(AbuseRestrictionRepository)
	if !ok {
		return nil, nil
	}
	at := denial.CreatedAt
	if at.IsZero() {
		at = time.Now().UTC()
		if m.now != nil {
			at = m.now().UTC()
		}
	}
	restrictions, err := repo.ListAbuseRestrictions(ctx, tenantID, at)
	if err != nil {
		return nil, err
	}
	for _, record := range restrictions {
		if record.AffectedCategory != denial.Category {
			continue
		}
		if record.VisibleReasonCode != "" && denial.ReasonCode != "" && record.VisibleReasonCode != denial.ReasonCode {
			continue
		}
		summary := record.Summary()
		return &summary, nil
	}
	return nil, nil
}

func ClassifyDenial(denial QuotaDenial, restriction *AbuseRestrictionSummary) DenialClassification {
	reason := strings.ToLower(denial.ReasonCode)
	switch {
	case restriction != nil || strings.Contains(reason, "abuse_restriction"):
		return DenialClassificationAbuseRestriction
	case reason == ReasonQuotaStateUnavailable:
		return DenialClassificationQuotaStateUnavailable
	case strings.Contains(reason, string(ReservationStatusOperatorActionNeeded)) || strings.Contains(reason, "operator_action_needed"):
		return DenialClassificationOperatorActionNeeded
	case strings.Contains(reason, "unauthorized"):
		return DenialClassificationUnauthorized
	default:
		return DenialClassificationQuotaExhaustion
	}
}

func statusForDenialClassification(classification DenialClassification) QuotaStatus {
	switch classification {
	case DenialClassificationAbuseRestriction:
		return QuotaStatusRestricted
	case DenialClassificationQuotaStateUnavailable, DenialClassificationOperatorActionNeeded:
		return QuotaStatusUnavailable
	case DenialClassificationUnauthorized:
		return QuotaStatusUnavailable
	default:
		return QuotaStatusExhausted
	}
}

func SafeOperationRef(operationKey string) string {
	if operationKey == "" {
		return "operation:unknown"
	}
	parts := strings.Split(operationKey, ":")
	if len(parts) >= 4 && parts[0] == "tenant" {
		return strings.Join(parts[2:], ":")
	}
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], ":")
	}
	return operationKey
}

func BuildEvidenceExport(generatedByPrincipalID string, denial QuotaDenialDetail, usageSnapshot []QuotaStatusItem, effectiveLimitState map[string]any) BillingEvidenceExport {
	cleanState, redactions := redactEvidenceValue("$", effectiveLimitState)
	state, ok := cleanState.(map[string]any)
	if !ok {
		state = map[string]any{}
	}
	redactions = appendStandardEvidenceRedactions(redactions)
	auditRefs := make([]string, 0, 1)
	if denial.Restriction != nil && denial.Restriction.SourceAuditRef != "" {
		auditRefs = append(auditRefs, denial.Restriction.SourceAuditRef)
	}
	return BillingEvidenceExport{
		SchemaVersion:          "2026-05-07",
		ExportID:               "evidence_" + denial.DenialID,
		TenantID:               denial.TenantID,
		GeneratedAt:            time.Now().UTC(),
		GeneratedByPrincipalID: generatedByPrincipalID,
		Denial:                 denial,
		UsageSnapshot:          append([]QuotaStatusItem(nil), usageSnapshot...),
		EffectiveLimitState:    state,
		AuditRefs:              auditRefs,
		Redactions:             redactions,
	}
}

func appendStandardEvidenceRedactions(redactions []BillingEvidenceRedaction) []BillingEvidenceRedaction {
	standard := []BillingEvidenceRedaction{
		{Path: "$.rawAuditPayload", Reason: "raw_audit_payload_excluded", Replacement: "[EXCLUDED]"},
		{Path: "$.connectorPayload", Reason: "connector_payload", Replacement: "[REDACTED]"},
		{Path: "$.secrets", Reason: "secret", Replacement: "[REDACTED]"},
		{Path: "$.unrelatedRunContent", Reason: "unrelated_content_excluded", Replacement: "[EXCLUDED]"},
	}
	seen := make(map[string]bool, len(redactions)+len(standard))
	for _, item := range redactions {
		seen[item.Path] = true
	}
	for _, item := range standard {
		if !seen[item.Path] {
			redactions = append(redactions, item)
		}
	}
	return redactions
}

func effectiveLimitStateForDenial(dashboard TenantQuotaDashboard, denial QuotaDenialDetail) map[string]any {
	state := map[string]any{
		"plan": map[string]any{
			"planKey":           dashboard.Plan.PlanKey,
			"enforcementMode":   string(dashboard.Plan.EnforcementMode),
			"status":            string(dashboard.Plan.Status),
			"basePlanLabel":     dashboard.Plan.BasePlanLabel,
			"checkoutAvailable": dashboard.Plan.CheckoutAvailable,
		},
		"denialCategory": string(denial.Category),
	}
	for _, section := range dashboard.Sections {
		for _, item := range section.Items {
			if item.Category != denial.Category {
				continue
			}
			quota := map[string]any{
				"category":               string(item.Category),
				"unit":                   string(item.Unit),
				"status":                 string(item.Status),
				"baseLimit":              item.BaseLimit,
				"effectiveLimit":         item.EffectiveLimit,
				"limit":                  item.Limit,
				"remainingAmount":        item.RemainingAmount,
				"nearLimit":              item.NearLimit,
				"nearLimitReason":        string(item.NearLimitReason),
				"typicalOperationAmount": item.TypicalOperationAmount,
				"currentPeriod": map[string]any{
					"periodStart":      item.CurrentPeriod.PeriodStart,
					"periodEnd":        item.CurrentPeriod.PeriodEnd,
					"periodAnchor":     item.CurrentPeriod.PeriodAnchor,
					"consumedAmount":   item.CurrentPeriod.ConsumedAmount,
					"reservedAmount":   item.CurrentPeriod.ReservedAmount,
					"adjustedAmount":   item.CurrentPeriod.AdjustedAmount,
					"carryoverApplied": item.CurrentPeriod.CarryoverApplied,
					"remainingAmount":  item.CurrentPeriod.RemainingAmount,
					"overLimit":        item.CurrentPeriod.OverLimit,
				},
				"recoveryActions": recoveryActionsAsStrings(item.RecoveryActions),
			}
			if item.PreviousPeriod != nil {
				quota["previousPeriod"] = map[string]any{
					"periodStart":      item.PreviousPeriod.PeriodStart,
					"periodEnd":        item.PreviousPeriod.PeriodEnd,
					"periodAnchor":     item.PreviousPeriod.PeriodAnchor,
					"consumedAmount":   item.PreviousPeriod.ConsumedAmount,
					"reservedAmount":   item.PreviousPeriod.ReservedAmount,
					"adjustedAmount":   item.PreviousPeriod.AdjustedAmount,
					"carryoverApplied": item.PreviousPeriod.CarryoverApplied,
					"remainingAmount":  item.PreviousPeriod.RemainingAmount,
					"overLimit":        item.PreviousPeriod.OverLimit,
				}
			}
			if item.Override != nil {
				quota["override"] = map[string]any{
					"baseLimit":      item.Override.BaseLimit,
					"effectiveLimit": item.Override.EffectiveLimit,
					"reason":         item.Override.Reason,
					"effectiveAt":    item.Override.EffectiveAt,
					"expiresAt":      item.Override.ExpiresAt,
				}
			}
			if item.Restriction != nil {
				quota["restriction"] = abuseRestrictionEvidenceState(item.Restriction)
			} else if denial.Restriction != nil {
				quota["restriction"] = abuseRestrictionEvidenceState(denial.Restriction)
			}
			state["quota"] = quota
			return state
		}
	}
	if denial.Restriction != nil {
		state["restriction"] = abuseRestrictionEvidenceState(denial.Restriction)
	}
	return state
}

func recoveryActionsAsStrings(actions []RecoveryAction) []any {
	out := make([]any, 0, len(actions))
	for _, action := range actions {
		out = append(out, string(action))
	}
	return out
}

func abuseRestrictionEvidenceState(restriction *AbuseRestrictionSummary) map[string]any {
	if restriction == nil {
		return nil
	}
	return map[string]any{
		"restrictionId":         restriction.RestrictionID,
		"status":                string(restriction.Status),
		"affectedCategory":      string(restriction.AffectedCategory),
		"recoveryAction":        string(restriction.RecoveryAction),
		"visibleReasonCode":     restriction.VisibleReasonCode,
		"sourceAuditRef":        restriction.SourceAuditRef,
		"supportContactAllowed": restriction.SupportContactAllowed,
		"startedAt":             restriction.StartedAt,
		"expiresAt":             restriction.ExpiresAt,
	}
}

func redactEvidenceValue(path string, source any) (any, []BillingEvidenceRedaction) {
	switch typed := source.(type) {
	case nil:
		return nil, nil
	case map[string]any:
		out := make(map[string]any, len(typed))
		var redactions []BillingEvidenceRedaction
		for key, value := range typed {
			childPath := path + "." + key
			if shouldRedactEvidenceKey(key) {
				redactions = append(redactions, BillingEvidenceRedaction{
					Path:        childPath,
					Reason:      redactionReasonForKey(key),
					Replacement: "[REDACTED]",
				})
				continue
			}
			clean, nestedRedactions := redactEvidenceValue(childPath, value)
			out[key] = clean
			redactions = append(redactions, nestedRedactions...)
		}
		return out, redactions
	case []any:
		out := make([]any, 0, len(typed))
		var redactions []BillingEvidenceRedaction
		for index, value := range typed {
			clean, nestedRedactions := redactEvidenceValue(fmt.Sprintf("%s[%d]", path, index), value)
			out = append(out, clean)
			redactions = append(redactions, nestedRedactions...)
		}
		return out, redactions
	default:
		return source, nil
	}
}

func shouldRedactEvidenceKey(key string) bool {
	normalized := strings.ToLower(key)
	return strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "credential") ||
		strings.Contains(normalized, "connectorpayload") ||
		strings.Contains(normalized, "payload")
}

func redactionReasonForKey(key string) string {
	normalized := strings.ToLower(key)
	if strings.Contains(normalized, "payload") {
		return "connector_payload"
	}
	return "secret"
}
