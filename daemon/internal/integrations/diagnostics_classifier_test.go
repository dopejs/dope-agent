package integrations_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
)

type providerFixtureFile struct {
	ProviderKind string `json:"providerKind"`
	Fixtures     []struct {
		Name                     string         `json:"name"`
		DomainKind               string         `json:"domainKind"`
		ProviderEvidence         map[string]any `json:"providerEvidence"`
		FaultType                string         `json:"faultType"`
		ExpectedReasonCode       string         `json:"expectedReasonCode"`
		ExpectedRetrySafety      string         `json:"expectedRetrySafety"`
		ExpectedRemediationOwner string         `json:"expectedRemediationOwner"`
	} `json:"fixtures"`
}

func TestFeishuLarkDiagnosticClassifierFixtures(t *testing.T) {
	t.Parallel()

	fixtures := readProviderFixture(t, "feishu_lark_reason_codes.json")
	assertFixtureReasonCoverage(t, fixtures, []integrations.DiagnosticReasonCode{
		integrations.ReasonHealthy,
		integrations.ReasonAppAuthorizationMissing,
		integrations.ReasonBotAuthorizationMissing,
		integrations.ReasonUserAuthorizationMissing,
		integrations.ReasonTenantApprovalPending,
		integrations.ReasonScopeMissing,
		integrations.ReasonTokenMissing,
		integrations.ReasonTokenExpired,
		integrations.ReasonTokenRevoked,
		integrations.ReasonRefreshCredentialsMissing,
		integrations.ReasonTokenRefreshFailed,
		integrations.ReasonTenantMismatch,
		integrations.ReasonRateLimited,
		integrations.ReasonProviderUnavailable,
		integrations.ReasonNetworkFailed,
		integrations.ReasonTransientProviderFailure,
		integrations.ReasonRedactionFailedClosed,
	})
	for _, fixture := range fixtures.Fixtures {
		fixture := fixture
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			classification := integrations.ClassifyProviderEvidence(integrations.ProviderEvidenceFromMap(fixtures.ProviderKind, fixture.DomainKind, fixture.ProviderEvidence))
			assertClassification(t, classification, fixture.ExpectedReasonCode, fixture.ExpectedRetrySafety, fixture.ExpectedRemediationOwner)
		})
	}
}

func assertFixtureReasonCoverage(t *testing.T, fixtures providerFixtureFile, required []integrations.DiagnosticReasonCode) {
	t.Helper()
	seen := map[string]bool{}
	for _, fixture := range fixtures.Fixtures {
		seen[fixture.ExpectedReasonCode] = true
	}
	for _, reason := range required {
		if !seen[string(reason)] {
			t.Fatalf("fixture %s missing required reason code %s", fixtures.ProviderKind, reason)
		}
	}
}

func TestFakeBackendDiagnosticClassifierFixtures(t *testing.T) {
	t.Parallel()

	fixtures := readProviderFixture(t, "fake_backend_reason_codes.json")
	for _, fixture := range fixtures.Fixtures {
		fixture := fixture
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			classification := integrations.ClassifyFakeBackendFault(integrations.FakeFaultType(fixture.FaultType), fixture.DomainKind)
			assertClassification(t, classification, fixture.ExpectedReasonCode, fixture.ExpectedRetrySafety, fixture.ExpectedRemediationOwner)
		})
	}
}

func TestDiagnosticClassifierAmbiguousCommitAndUnsafeRetry(t *testing.T) {
	t.Parallel()

	ambiguous := integrations.ClassifyProviderEvidence(integrations.ProviderDiagnosticEvidence{
		ProviderKind:    "feishu_lark",
		DomainKind:      "calendar",
		CommitAmbiguous: true,
	})
	assertClassification(t, ambiguous, string(integrations.ReasonAmbiguousDownstreamCommit), string(integrations.RetrySafetyUnsafeToRetry), string(integrations.RemediationOwnerOperator))
	if !ambiguous.Ambiguous {
		t.Fatalf("expected ambiguous classification: %+v", ambiguous)
	}

	unsafe := integrations.ClassifyProviderEvidence(integrations.ProviderDiagnosticEvidence{
		ProviderKind:   "feishu_lark",
		DomainKind:     "calendar",
		SideEffecting:  true,
		Message:        "retry unsafe after downstream timeout",
		OperationClass: "create_event",
	})
	assertClassification(t, unsafe, string(integrations.ReasonUnsafeToRetry), string(integrations.RetrySafetyUnsafeToRetry), string(integrations.RemediationOwnerOperator))
}

func readProviderFixture(t *testing.T, name string) providerFixtureFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "diagnostics", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	var fixtures providerFixtureFile
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatalf("decode fixture %s: %v", name, err)
	}
	return fixtures
}

func assertClassification(t *testing.T, got integrations.ProviderErrorClassification, reasonCode, retrySafety, remediationOwner string) {
	t.Helper()
	if string(got.ReasonCode) != reasonCode || string(got.RetrySafety) != retrySafety || string(got.RemediationOwner) != remediationOwner {
		t.Fatalf("classification mismatch got=%+v want reason=%s retry=%s owner=%s", got, reasonCode, retrySafety, remediationOwner)
	}
}
