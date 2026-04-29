package billing

import "fmt"

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
