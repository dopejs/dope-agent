package billing

import "context"

const LiveValidationPreflightEntryPoint = "Roadmap 38 live-validation preflight gate"

func ReserveLiveValidationPreflight(ctx context.Context, manager *Manager, tenantID, validationID, clientKey string, hosted bool) (ReserveResult, error) {
	operationKey := LiveValidationOperationKey(tenantID, validationID, clientKey)
	if manager == nil {
		if hosted {
			denial := NewQuotaStateUnavailableDenial(tenantID, operationKey).Payload
			return ReserveResult{Allowed: false, Denial: &denial}, ErrQuotaStateUnavailable
		}
		return ReserveResult{Allowed: true}, nil
	}
	return manager.Reserve(ctx, ReserveInput{
		TenantID:          tenantID,
		Category:          CategoryLiveValidationAttempts,
		Amount:            1,
		OperationKey:      operationKey,
		ReservationPoint:  LiveValidationPreflightEntryPoint,
		GuardedEntryPoint: LiveValidationPreflightEntryPoint,
		Hosted:            hosted,
	})
}
