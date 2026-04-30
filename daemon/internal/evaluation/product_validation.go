package evaluation

import (
	"errors"
	"strings"
)

const (
	DefaultProductPageLimit = 50
	MaxProductPageLimit     = 200
)

var (
	ErrEvaluationProductTenantRequired  = errors.New("evaluation product tenant required")
	ErrEvaluationProductInvalidBounds   = errors.New("evaluation product bounds invalid")
	ErrEvaluationProductRedactionFailed = errors.New("evaluation product redaction failed")
)

func ValidateTenantScopedProductRequest(tenantID string) error {
	if strings.TrimSpace(tenantID) == "" {
		return ErrEvaluationProductTenantRequired
	}
	return nil
}

func NormalizeProductLimit(limit int) int {
	if limit <= 0 {
		return DefaultProductPageLimit
	}
	if limit > MaxProductPageLimit {
		return MaxProductPageLimit
	}
	return limit
}

func ValidateDiscoveryPolicy(policy DiscoveryPolicy) error {
	if err := ValidateTenantScopedProductRequest(policy.TenantID); err != nil {
		return err
	}
	if policy.MaxInspectedRecords <= 0 || policy.MaxEmittedCandidates <= 0 || policy.CostBudget <= 0 {
		return ErrEvaluationProductInvalidBounds
	}
	if !policy.WindowStart.IsZero() && !policy.WindowEnd.IsZero() && !policy.WindowStart.Before(policy.WindowEnd) {
		return ErrEvaluationProductInvalidBounds
	}
	return nil
}
