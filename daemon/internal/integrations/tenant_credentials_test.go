package integrations_test

import (
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
)

func TestR37IntegrationSameExternalAccountIsTenantLocal(t *testing.T) {
	manager := integrations.NewManager("test")
	tenantA, err := manager.Create(integrations.CreateInput{
		TenantID:      "ten_r37_a",
		IntegrationID: "calendar-a",
		DomainKind:    "calendar",
		DisplayName:   "Calendar A",
		AccountBinding: integrations.AccountBinding{
			AccountKey:        "same-human@example.com",
			ExternalAccountID: "acct_same",
			AccountLabel:      "same-human@example.com",
		},
		BackendBinding: integrations.BackendBinding{BackendKind: integrations.BackendKindFakeLocal},
	})
	if err != nil {
		t.Fatalf("create tenant A integration: %v", err)
	}
	tenantB, err := manager.Create(integrations.CreateInput{
		TenantID:      "ten_r37_b",
		IntegrationID: "calendar-b",
		DomainKind:    "calendar",
		DisplayName:   "Calendar B",
		AccountBinding: integrations.AccountBinding{
			AccountKey:        "same-human@example.com",
			ExternalAccountID: "acct_same",
			AccountLabel:      "same-human@example.com",
		},
		BackendBinding: integrations.BackendBinding{BackendKind: integrations.BackendKindFakeLocal},
	})
	if err != nil {
		t.Fatalf("create tenant B integration: %v", err)
	}
	if tenantA.TenantID == tenantB.TenantID {
		t.Fatalf("fixtures did not create distinct tenant ownership: A=%q B=%q", tenantA.TenantID, tenantB.TenantID)
	}
	if got := manager.ListForTenant("ten_r37_a"); len(got) != 1 || got[0].IntegrationID != tenantA.IntegrationID {
		t.Fatalf("tenant A list leaked or missed integration: %#v", got)
	}
	if got := manager.ListForTenant("ten_r37_b"); len(got) != 1 || got[0].IntegrationID != tenantB.IntegrationID {
		t.Fatalf("tenant B list leaked or missed integration: %#v", got)
	}
	if _, ok := manager.GetForTenant(tenantA.IntegrationID, "ten_r37_b"); ok {
		t.Fatal("tenant B could read tenant A integration by id")
	}
}

func TestR37IntegrationCanonicalDefaultIsTenantLocal(t *testing.T) {
	manager := integrations.NewManager("test")
	_, err := manager.Create(integrations.CreateInput{
		TenantID:         "ten_r37_a",
		IntegrationID:    "mail-a",
		DomainKind:       "mail",
		DisplayName:      "Mail A",
		AccountBinding:   integrations.AccountBinding{AccountKey: "same-human@example.com"},
		BackendBinding:   integrations.BackendBinding{BackendKind: integrations.BackendKindFakeLocal},
		CanonicalDefault: true,
	})
	if err != nil {
		t.Fatalf("create tenant A default: %v", err)
	}
	_, err = manager.Create(integrations.CreateInput{
		TenantID:         "ten_r37_b",
		IntegrationID:    "mail-b",
		DomainKind:       "mail",
		DisplayName:      "Mail B",
		AccountBinding:   integrations.AccountBinding{AccountKey: "same-human@example.com"},
		BackendBinding:   integrations.BackendBinding{BackendKind: integrations.BackendKindFakeLocal},
		CanonicalDefault: true,
	})
	if err != nil {
		t.Fatalf("create tenant B default: %v", err)
	}
	if got := manager.ListForTenant("ten_r37_a"); len(got) != 1 || !got[0].CanonicalDefault {
		t.Fatalf("tenant A default was demoted by tenant B: %#v", got)
	}
	if got := manager.ListForTenant("ten_r37_b"); len(got) != 1 || !got[0].CanonicalDefault {
		t.Fatalf("tenant B default missing: %#v", got)
	}
}
