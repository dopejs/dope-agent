package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func TestLiveValidationKillSwitchSetListAndBlocksStart(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	manager := livevalidation.NewManager(livevalidation.Dependencies{Enabled: true, Store: sqliteStore, Clock: func() time.Time { return now }})
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{
		TenantID:    "ten_1",
		PrincipalID: "prn_admin",
		Role:        identity.RoleAdmin,
		Permissions: identity.PermissionsForRole(identity.RoleAdmin, identity.StatusActive),
	})

	setReq := httptest.NewRequest(http.MethodPost, "/v1/live-validations/kill-switches", bytes.NewBufferString(`{"scope":"tenant","enabled":true,"reason":"containment"}`)).WithContext(ctx)
	setReq.Header.Set("Content-Type", "application/json")
	setResp := httptest.NewRecorder()
	handleLiveValidationRoutes(manager, nil, sqliteStore, setResp, setReq)
	if setResp.Code != http.StatusOK {
		t.Fatalf("set status=%d body=%s", setResp.Code, setResp.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/live-validations/kill-switches", nil).WithContext(ctx)
	listResp := httptest.NewRecorder()
	handleLiveValidationRoutes(manager, nil, sqliteStore, listResp, listReq)
	if listResp.Code != http.StatusOK || !bytes.Contains(listResp.Body.Bytes(), []byte(`"enabled":true`)) {
		t.Fatalf("list status=%d body=%s", listResp.Code, listResp.Body.String())
	}

	startReq := httptest.NewRequest(http.MethodPost, "/v1/live-validations", bytes.NewBufferString(liveValidationStartBody("lv_kill_switch_route", "daemon.inspection.read", nil))).WithContext(ctx)
	startReq.Header.Set("Content-Type", "application/json")
	startResp := httptest.NewRecorder()
	handleLiveValidationRoutes(manager, nil, sqliteStore, startResp, startReq)
	if startResp.Code != http.StatusConflict || !bytes.Contains(startResp.Body.Bytes(), []byte(`kill_switch`)) {
		t.Fatalf("start status=%d body=%s", startResp.Code, startResp.Body.String())
	}
}
