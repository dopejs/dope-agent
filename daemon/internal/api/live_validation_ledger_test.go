package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func TestLiveValidationLedgerReconciliationRetentionAndComparisonRoutes(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	manager := livevalidation.NewManager(livevalidation.Dependencies{Enabled: true, Store: sqliteStore, Clock: func() time.Time { return now }})
	attempt := livevalidation.Attempt{
		ValidationID:     "lv_ledger",
		TenantID:         "ten_1",
		CandidateID:      "candidate_1",
		EnvironmentScope: "test",
		Status:           livevalidation.AttemptStatusRunning,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := sqliteStore.UpsertLiveValidationAttempt(context.Background(), attempt); err != nil {
		t.Fatalf("UpsertLiveValidationAttempt: %v", err)
	}
	if _, err := manager.AppendLedgerEntry(context.Background(), livevalidation.SideEffectLedgerEntry{
		LedgerEntryID: "ledger_1",
		ValidationID:  attempt.ValidationID,
		TenantID:      attempt.TenantID,
		CandidateID:   attempt.CandidateID,
		ToolClass:     livevalidation.ToolClassMailSend,
		SafetyClass:   livevalidation.SafetyClassNonIdempotentMutation,
		ActionRef:     "send_1",
		Outcome:       livevalidation.LedgerOutcomeOperatorActionNeeded,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("AppendLedgerEntry: %v", err)
	}
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{
		TenantID:    "ten_1",
		PrincipalID: "prn_admin",
		Role:        identity.RoleAdmin,
		Permissions: identity.PermissionsForRole(identity.RoleAdmin, identity.StatusActive),
	})

	ledgerReq := httptest.NewRequest(http.MethodGet, "/v1/live-validations/lv_ledger/ledger", nil).WithContext(ctx)
	ledgerResp := httptest.NewRecorder()
	handleLiveValidationRoutes(manager, nil, sqliteStore, ledgerResp, ledgerReq)
	if ledgerResp.Code != http.StatusOK {
		t.Fatalf("ledger status=%d body=%s", ledgerResp.Code, ledgerResp.Body.String())
	}

	compareReq := httptest.NewRequest(http.MethodPost, "/v1/live-validations/lv_ledger/compare", nil).WithContext(ctx)
	compareResp := httptest.NewRecorder()
	handleLiveValidationRoutes(manager, nil, sqliteStore, compareResp, compareReq)
	if compareResp.Code != http.StatusAccepted {
		t.Fatalf("compare status=%d body=%s", compareResp.Code, compareResp.Body.String())
	}

	retentionReq := httptest.NewRequest(http.MethodGet, "/v1/live-validations/lv_ledger/retention", nil).WithContext(ctx)
	retentionResp := httptest.NewRecorder()
	handleLiveValidationRoutes(manager, nil, sqliteStore, retentionResp, retentionReq)
	if retentionResp.Code != http.StatusOK || !bytes.Contains(retentionResp.Body.Bytes(), []byte(`"mode":"indefinite"`)) {
		t.Fatalf("retention status=%d body=%s", retentionResp.Code, retentionResp.Body.String())
	}
	if _, err := manager.RecordAmbiguousCommit(context.Background(), livevalidation.AmbiguousCommit{
		AmbiguousCommitID: "amb_1",
		LedgerEntryID:     "ledger_1",
		ValidationID:      "lv_ledger",
		TenantID:          "ten_1",
		Cause:             livevalidation.AmbiguousCauseTimeout,
		CreatedAt:         now,
		UpdatedAt:         now,
	}); err != nil {
		t.Fatalf("RecordAmbiguousCommit: %v", err)
	}

	reconcileReq := httptest.NewRequest(http.MethodPost, "/v1/live-validations/lv_ledger/reconciliations/amb_1/resolve", bytes.NewBufferString(`{"resolution":"confirmed_committed","reason":"provider checked"}`)).WithContext(ctx)
	reconcileReq.Header.Set("Content-Type", "application/json")
	reconcileResp := httptest.NewRecorder()
	handleLiveValidationRoutes(manager, nil, sqliteStore, reconcileResp, reconcileReq)
	if reconcileResp.Code != http.StatusOK {
		t.Fatalf("reconcile status=%d body=%s", reconcileResp.Code, reconcileResp.Body.String())
	}
	var resolution livevalidation.ReconciliationResolution
	if err := json.Unmarshal(reconcileResp.Body.Bytes(), &resolution); err != nil {
		t.Fatalf("decode reconciliation: %v", err)
	}
	if resolution.ResolvedBy != "prn_admin" {
		t.Fatalf("resolution=%+v, want admin resolver", resolution)
	}
}
