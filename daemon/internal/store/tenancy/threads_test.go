package tenancy

import (
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

func TestThreadAccessScopeRejectsCrossTenantThread(t *testing.T) {
	scope := ThreadAccessScope{TenantID: "ten_1"}
	if !scope.Allows("ten_1") {
		t.Fatal("expected same tenant to be allowed")
	}
	if scope.Allows("ten_2") {
		t.Fatal("expected cross tenant to be denied")
	}
}

func TestThreadAccessScopeRejectsCrossTenantContinuityEvidence(t *testing.T) {
	scope := ThreadAccessScope{TenantID: "ten_1"}
	if !scope.AllowsContinuityTurn(threads.ContinuityTurn{TenantID: "ten_1"}) {
		t.Fatal("expected same-tenant continuity turn to be allowed")
	}
	if scope.AllowsContinuityTurn(threads.ContinuityTurn{TenantID: "ten_2"}) {
		t.Fatal("expected cross-tenant continuity turn to be denied")
	}
	if !scope.AllowsContinuityPreview(threads.ContinuityPreview{TenantID: "ten_1"}) {
		t.Fatal("expected same-tenant continuity preview to be allowed")
	}
	if scope.AllowsContinuityPreview(threads.ContinuityPreview{TenantID: "ten_2"}) {
		t.Fatal("expected cross-tenant continuity preview to be denied")
	}
}
