package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/profiles"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

func TestThreadDetailProjectsActiveProfileEvidence(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()
	ctx := withTenantContext(httptest.NewRequest(http.MethodGet, "/", nil).Context(), identity.TenantContext{
		TenantID:    "ten_thread_profile_projection",
		PrincipalID: "prn_viewer",
		Permissions: []identity.Permission{identity.PermissionCredentialsInspect, identity.PermissionProfilesInspect},
	})
	admin := identity.TenantContext{TenantID: "ten_thread_profile_projection", PrincipalID: "prn_admin", Permissions: []identity.Permission{identity.PermissionProfilesInspect, identity.PermissionProfilesManage}}
	result, err := sqliteStore.CreateAgentProfile(ctx, admin, profiles.MutationInput{DisplayName: "Thread Agent", Persona: profiles.Persona{SafeSummary: "thread profile"}, Activate: true})
	if err != nil {
		t.Fatalf("CreateAgentProfile returned error: %v", err)
	}
	_, selection, _, err := sqliteStore.ActiveAgentProfileSelection(ctx, admin.TenantID)
	if err != nil {
		t.Fatalf("ActiveAgentProfileSelection returned error: %v", err)
	}
	now := time.Now().UTC()
	if err := sqliteStore.UpsertThread(ctx, threads.Thread{ThreadID: "thr_profile_detail", TenantID: admin.TenantID, LifecycleState: threads.LifecycleStateActive, CurrentSessionSegmentID: "seg_profile_detail", SourceKind: threads.SourceKindShell, LastActivityAt: now, CreatedAt: now, UpdatedAt: now, RedactionStatus: threads.RedactionStatusRedacted}); err != nil {
		t.Fatalf("UpsertThread returned error: %v", err)
	}
	if _, err := sqliteStore.RecordRuntimeProfileProjection(ctx, profiles.BuildRuntimeProjection(result.Profile, selection, profiles.RuntimeProjectionInput{ResourceKind: profiles.RuntimeResourceThread, ResourceID: "thr_profile_detail", ThreadID: "thr_profile_detail", SessionID: "seg_profile_detail"})); err != nil {
		t.Fatalf("RecordRuntimeProfileProjection returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/threads/thr_profile_detail", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handleThreadLifecycleRoutes(sqliteStore, events.NewBus(), rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("thread detail status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"activeProfileProjection"`) || !strings.Contains(rec.Body.String(), `"profileId":"`+result.Profile.ProfileID+`"`) {
		t.Fatalf("expected active profile projection in thread detail, body=%s", rec.Body.String())
	}

	viewerOnlyCtx := withTenantContext(httptest.NewRequest(http.MethodGet, "/", nil).Context(), identity.TenantContext{
		TenantID:    "ten_thread_profile_projection",
		PrincipalID: "prn_thread_only_viewer",
		Permissions: []identity.Permission{identity.PermissionCredentialsInspect},
	})
	viewerOnlyReq := httptest.NewRequest(http.MethodGet, "/v1/threads/thr_profile_detail", nil).WithContext(viewerOnlyCtx)
	viewerOnlyRec := httptest.NewRecorder()
	handleThreadLifecycleRoutes(sqliteStore, events.NewBus(), viewerOnlyRec, viewerOnlyReq)
	if viewerOnlyRec.Code != http.StatusOK {
		t.Fatalf("viewer-only thread detail status=%d body=%s", viewerOnlyRec.Code, viewerOnlyRec.Body.String())
	}
	if strings.Contains(viewerOnlyRec.Body.String(), `"activeProfileProjection"`) || strings.Contains(viewerOnlyRec.Body.String(), result.Profile.ProfileID) {
		t.Fatalf("viewer without profiles.inspect must not see profile projection, body=%s", viewerOnlyRec.Body.String())
	}
}
