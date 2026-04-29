package contracts_test

import (
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func TestLiveValidationStartResponseContractVariants(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	fixtures := []string{
		`{"attempt":{"validationId":"lv_blocked","candidateId":"candidate_1","requestedBy":"prn_1","environmentScope":"test","requestedScope":{"scopeId":"scope_1","validationId":"lv_blocked","approvalMode":"scope_level","declaredBy":"prn_1","declaredAt":"2026-04-29T10:00:00Z"},"status":"blocked","permissionDecision":{"allowed":false,"reasonCode":"permission_missing","checkedAt":"2026-04-29T10:00:00Z"},"quotaDecision":{"allowed":true,"checkedAt":"2026-04-29T10:00:00Z"},"killSwitchDecision":{"allowed":true,"checkedAt":"2026-04-29T10:00:00Z"},"approvalSummary":{"required":0,"approved":0,"denied":0,"expired":0,"pending":0},"ledgerSummary":{},"createdAt":"2026-04-29T10:00:00Z","updatedAt":"2026-04-29T10:00:00Z"},"denials":[{"reasonCode":"permission_missing","gate":"permission","message":"Missing live validation permission."}]}`,
		`{"attempt":{"validationId":"lv_awaiting","candidateId":"candidate_1","requestedBy":"prn_1","environmentScope":"test","requestedScope":{"scopeId":"scope_1","validationId":"lv_awaiting","approvalMode":"scope_level","declaredBy":"prn_1","declaredAt":"2026-04-29T10:00:00Z"},"status":"awaiting_approval","permissionDecision":{"allowed":true,"checkedAt":"2026-04-29T10:00:00Z"},"quotaDecision":{"allowed":true,"checkedAt":"2026-04-29T10:00:00Z"},"killSwitchDecision":{"allowed":true,"checkedAt":"2026-04-29T10:00:00Z"},"approvalSummary":{"required":1,"approved":0,"denied":0,"expired":0,"pending":1},"ledgerSummary":{},"createdAt":"2026-04-29T10:00:00Z","updatedAt":"2026-04-29T10:00:00Z"}}`,
		`{"attempt":{"validationId":"lv_running","candidateId":"candidate_1","requestedBy":"prn_1","environmentScope":"test","requestedScope":{"scopeId":"scope_1","validationId":"lv_running","approvalMode":"scope_level","declaredBy":"prn_1","declaredAt":"2026-04-29T10:00:00Z"},"status":"running","permissionDecision":{"allowed":true,"checkedAt":"2026-04-29T10:00:00Z"},"quotaDecision":{"allowed":true,"checkedAt":"2026-04-29T10:00:00Z"},"killSwitchDecision":{"allowed":true,"checkedAt":"2026-04-29T10:00:00Z"},"approvalSummary":{"required":1,"approved":1,"denied":0,"expired":0,"pending":0},"ledgerSummary":{},"createdAt":"2026-04-29T10:00:00Z","startedAt":"2026-04-29T10:00:00Z","updatedAt":"2026-04-29T10:00:00Z"}}`,
	}
	for _, fixture := range fixtures {
		if err := validator.ValidateRelative("schemas/api/create-live-validation.response.schema.json", []byte(fixture)); err != nil {
			t.Fatalf("ValidateRelative(create-live-validation.response) returned error: %v", err)
		}
	}
}
