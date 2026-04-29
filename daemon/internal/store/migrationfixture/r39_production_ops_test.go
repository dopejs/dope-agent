package migrationfixture

import "testing"

func TestR39ProductionOpsFixtureHasThreeDistinctTenantStates(t *testing.T) {
	fixture := BuildR39ProductionOpsFixture()
	if len(fixture.Tenants) < 3 {
		t.Fatalf("expected at least three tenants, got %d", len(fixture.Tenants))
	}
	seenTenants := map[string]bool{}
	seenQuota := map[string]bool{}
	seenWork := map[string]bool{}
	for _, tenant := range fixture.Tenants {
		if tenant.TenantID == "" {
			t.Fatalf("tenant id is required")
		}
		if seenTenants[tenant.TenantID] {
			t.Fatalf("duplicate tenant id %q", tenant.TenantID)
		}
		if len(tenant.CredentialRefs) == 0 {
			t.Fatalf("tenant %s missing credential refs", tenant.TenantID)
		}
		seenTenants[tenant.TenantID] = true
		seenQuota[tenant.QuotaState] = true
		seenWork[tenant.WorkState] = true
	}
	if len(seenQuota) < 3 {
		t.Fatalf("expected distinct quota states, got %v", seenQuota)
	}
	if len(seenWork) < 3 {
		t.Fatalf("expected distinct work states, got %v", seenWork)
	}
	if len(fixture.RawCredentialValues) != 0 {
		t.Fatalf("fixture must not expose raw credential values")
	}
}

func TestR39ProductionOpsSQLiteFixtureRestoresTenantDataAndCredentialRemediation(t *testing.T) {
	sourcePath := t.TempDir() + "/source.sqlite"
	restoredPath := t.TempDir() + "/restored.sqlite"

	fixture, err := BuildR39ProductionOpsSQLiteFixture(sourcePath)
	if err != nil {
		t.Fatalf("build sqlite fixture: %v", err)
	}
	if err := CopyR39ProductionOpsSQLiteFixture(sourcePath, restoredPath); err != nil {
		t.Fatalf("copy sqlite fixture: %v", err)
	}
	if err := ValidateR39ProductionOpsSQLiteRestore(restoredPath, fixture); err != nil {
		t.Fatalf("validate restored sqlite fixture: %v", err)
	}
}
