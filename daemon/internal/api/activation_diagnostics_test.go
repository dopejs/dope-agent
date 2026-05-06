package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/activation"
	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/billing"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	storepkg "github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestActivationDiagnosticsRouteProjectsQuotaFailureMetadata(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	sqliteStore, err := storepkg.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	if err := sqliteStore.UpsertPrincipal(ctx, identity.Principal{PrincipalID: "prn_diag", PrincipalKind: identity.PrincipalKindUser, DisplayName: "Diagnostic User", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("UpsertPrincipal returned error: %v", err)
	}
	service := activation.NewService(activation.Dependencies{
		StateStore:       sqliteStore,
		Identity:         sqliteStore,
		Billing:          diagnosticQuotaProjector{},
		Audit:            sqliteStore,
		Now:              func() time.Time { return now },
		EnvironmentScope: "prod",
		Hosted:           true,
	})
	started := exerciseActivationPost(t, service, auth.AccessToken{TokenID: "tok_diag", PrincipalID: "prn_diag", Status: string(identity.StatusActive)}, `{"source":"signup"}`)

	req := httptest.NewRequest(http.MethodGet, "/v1/activation/diagnostics", nil)
	req = req.WithContext(withAuthenticatedToken(req.Context(), auth.AccessToken{TokenID: "tok_diag", PrincipalID: "prn_diag", Status: string(identity.StatusActive)}))
	req = req.WithContext(withTenantContext(req.Context(), identity.TenantContext{PrincipalID: "prn_diag", TokenID: "tok_diag", TenantID: started.Activation.TenantID}))
	rec := httptest.NewRecorder()

	handleActivationDiagnostics(service, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Items []activation.Diagnostic `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode diagnostics response: %v", err)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("expected one diagnostic, got %#v", payload.Items)
	}
	item := payload.Items[0]
	if item.Stage != activation.FailureStageQuotaBaseline || item.ReasonCode != activation.ReasonQuotaBaselineUnavailable || !item.Retryable || item.RemediationOwner != activation.RemediationOwnerOperator {
		t.Fatalf("unexpected diagnostic item: %#v", item)
	}
	if item.TenantID != started.Activation.TenantID || item.QuotaBaselineStatus != string(activation.QuotaBaselineStatusUnavailable) {
		t.Fatalf("expected tenant-scoped quota diagnostic, got %#v", item)
	}
	if string(rec.Body.Bytes()) == "" || containsAny(rec.Body.String(), []string{"authorization", "accessToken", "refreshToken", "transcript", "rawProviderPayload"}) {
		t.Fatalf("diagnostics response contains forbidden evidence: %s", rec.Body.String())
	}
}

type diagnosticQuotaProjector struct{}

func (diagnosticQuotaProjector) UsageSummary(context.Context, string, bool) (billing.UsageSummary, error) {
	return billing.UsageSummary{}, billing.ErrQuotaStateUnavailable
}

func containsAny(value string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
