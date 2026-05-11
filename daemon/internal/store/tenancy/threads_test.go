package tenancy

import "testing"

func TestThreadAccessScopeRejectsCrossTenantThread(t *testing.T) {
	scope := ThreadAccessScope{TenantID: "ten_1"}
	if !scope.Allows("ten_1") {
		t.Fatal("expected same tenant to be allowed")
	}
	if scope.Allows("ten_2") {
		t.Fatal("expected cross tenant to be denied")
	}
}
