package integrations

import (
	"testing"
	"time"
)

func TestFakeBackendFaultDrillsClassifyRequiredFailures(t *testing.T) {
	backend := FakeBackend{}
	resource := Resource{
		IntegrationID: "calendar-a", DomainKind: "calendar", DisplayName: "Calendar A",
		EnvironmentScope: "test", BackendBinding: BackendBinding{BackendKind: BackendKindFakeLocal},
		CreatedAt: time.Now(), UpdatedAt: time.Now(), LastTransitionAt: time.Now(),
	}
	required := []FakeFaultType{
		FakeFaultTransient5xx,
		FakeFaultRateLimit,
		FakeFaultAuthExpiry,
		FakeFaultProviderUnavailable,
		FakeFaultSlowResponse,
		FakeFaultMalformedResponse,
	}
	for _, faultType := range required {
		result := backend.RunFaultDrill(resource, faultType)
		if result.ObservedClassification == "" {
			t.Fatalf("fault %s was not classified", faultType)
		}
		if result.ObservedClassification == "retry_exhausted" && !result.OperatorActionNeeded {
			t.Fatalf("retry exhaustion for %s must expose operator action", faultType)
		}
	}
}
