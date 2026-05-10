package matrix

import "testing"

func TestValidateHomeserverBindingRequiresExactlyOneTenantScopedBot(t *testing.T) {
	t.Parallel()

	binding := NormalizeHomeserverBinding("ten_matrix", "matrix-main", HomeserverBinding{
		HomeserverURL:      "https://matrix.example.org",
		BotUserID:          "@bot:example.org",
		AuthorizationState: AuthorizationValid,
		CapabilityState:    HomeserverCapabilityValid,
	})

	if binding.HomeserverBindingID == "" || binding.TenantID != "ten_matrix" || binding.ConnectorID != "matrix-main" {
		t.Fatalf("binding was not normalized tenant-safely: %+v", binding)
	}
	if err := ValidateHomeserverBinding(binding); err != nil {
		t.Fatalf("ValidateHomeserverBinding returned error: %v", err)
	}

	binding.BotUserID = ""
	if err := ValidateHomeserverBinding(binding); err == nil {
		t.Fatal("expected missing bot user to fail readiness")
	}
}
