package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func TestEvaluationProductFixturePermissionDenialsAndLifecycleRoutes(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	adminCtx := tenantctx.WithContext(context.Background(), identity.TenantContext{
		TenantID:    "ten_fixture_api",
		PrincipalID: "prn_admin",
		Permissions: []identity.Permission{
			identity.PermissionEvaluationFixtureRead,
			identity.PermissionEvaluationFixtureManage,
			identity.PermissionEvaluationFixtureReview,
			identity.PermissionEvaluationFixtureSuppress,
		},
	})
	viewerCtx := tenantctx.WithContext(context.Background(), identity.TenantContext{
		TenantID:    "ten_fixture_api",
		PrincipalID: "prn_viewer",
		Permissions: []identity.Permission{
			identity.PermissionEvaluationFixtureRead,
		},
	})
	seedFixtureCandidate(t, sqliteStore, adminCtx, now)

	deniedCreate := httptest.NewRequest(http.MethodPost, "/v1/evaluation/discovered-candidates/candidate_fixture_api/product-fixtures", jsonBody(map[string]any{
		"displayName":    "Denied Fixture",
		"domainClass":    "schedule",
		"fixturePayload": map[string]any{"goal": "safe"},
	})).WithContext(viewerCtx)
	deniedCreateRec := httptest.NewRecorder()
	handleEvaluationRoutes(nil, nil, nil, sqliteStore, deniedCreateRec, deniedCreate)
	if deniedCreateRec.Code != http.StatusForbidden || !strings.Contains(deniedCreateRec.Body.String(), "evaluation.fixture.manage") {
		t.Fatalf("denied create status=%d body=%s", deniedCreateRec.Code, deniedCreateRec.Body.String())
	}

	create := httptest.NewRequest(http.MethodPost, "/v1/evaluation/discovered-candidates/candidate_fixture_api/product-fixtures", jsonBody(map[string]any{
		"fixtureId":      "product_fixture_api",
		"displayName":    "Product Fixture API",
		"domainClass":    "schedule",
		"fixturePayload": map[string]any{"goal": "safe"},
		"changeSummary":  "initial",
	})).WithContext(adminCtx)
	createRec := httptest.NewRecorder()
	handleEvaluationRoutes(nil, nil, nil, sqliteStore, createRec, create)
	if createRec.Code != http.StatusCreated || !strings.Contains(createRec.Body.String(), "product_fixture_api") {
		t.Fatalf("create status=%d body=%s", createRec.Code, createRec.Body.String())
	}

	var created productFixtureMutationResponse
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	deniedRevision := httptest.NewRequest(http.MethodPost, "/v1/evaluation/product-fixtures/product_fixture_api/revisions", jsonBody(map[string]any{
		"fixturePayload": map[string]any{"goal": "viewer edit"},
		"changeSummary":  "viewer edit",
	})).WithContext(viewerCtx)
	deniedRevisionRec := httptest.NewRecorder()
	handleEvaluationRoutes(nil, nil, nil, sqliteStore, deniedRevisionRec, deniedRevision)
	if deniedRevisionRec.Code != http.StatusForbidden || !strings.Contains(deniedRevisionRec.Body.String(), "evaluation.fixture.manage") {
		t.Fatalf("denied revision status=%d body=%s", deniedRevisionRec.Code, deniedRevisionRec.Body.String())
	}

	deniedReview := httptest.NewRequest(http.MethodPost, "/v1/evaluation/product-fixtures/product_fixture_api/review", jsonBody(map[string]any{
		"revisionId": created.Revision.RevisionID,
		"decision":   "approved",
	})).WithContext(viewerCtx)
	deniedReviewRec := httptest.NewRecorder()
	handleEvaluationRoutes(nil, nil, nil, sqliteStore, deniedReviewRec, deniedReview)
	if deniedReviewRec.Code != http.StatusForbidden || !strings.Contains(deniedReviewRec.Body.String(), "evaluation.fixture.review") {
		t.Fatalf("denied review status=%d body=%s", deniedReviewRec.Code, deniedReviewRec.Body.String())
	}

	review := httptest.NewRequest(http.MethodPost, "/v1/evaluation/product-fixtures/product_fixture_api/review", jsonBody(map[string]any{
		"revisionId": created.Revision.RevisionID,
		"decision":   "approved",
	})).WithContext(adminCtx)
	reviewRec := httptest.NewRecorder()
	handleEvaluationRoutes(nil, nil, nil, sqliteStore, reviewRec, review)
	if reviewRec.Code != http.StatusOK || !strings.Contains(reviewRec.Body.String(), `"reviewState":"approved"`) {
		t.Fatalf("review status=%d body=%s", reviewRec.Code, reviewRec.Body.String())
	}

	deniedSuppress := httptest.NewRequest(http.MethodPost, "/v1/evaluation/product-fixtures/product_fixture_api/suppress", nil).WithContext(viewerCtx)
	deniedSuppressRec := httptest.NewRecorder()
	handleEvaluationRoutes(nil, nil, nil, sqliteStore, deniedSuppressRec, deniedSuppress)
	if deniedSuppressRec.Code != http.StatusForbidden || !strings.Contains(deniedSuppressRec.Body.String(), "evaluation.fixture.suppress") {
		t.Fatalf("denied suppress status=%d body=%s", deniedSuppressRec.Code, deniedSuppressRec.Body.String())
	}
}

func seedFixtureCandidate(t *testing.T, sqliteStore *store.SQLiteStore, ctx context.Context, now time.Time) {
	t.Helper()
	run := evaluation.DiscoveryRun{
		DiscoveryRunID:       "discovery_run_fixture_api",
		TenantID:             "ten_fixture_api",
		Status:               evaluation.ProductStatusCompleted,
		SourceKinds:          []evaluation.SourceKind{evaluation.SourceKindRun},
		WindowStart:          now.Add(-time.Hour),
		WindowEnd:            now,
		MaxInspectedRecords:  10,
		MaxEmittedCandidates: 2,
		CostBudget:           5,
		StartedAt:            now,
		UpdatedAt:            now,
	}
	if err := sqliteStore.SaveDiscoveryRun(ctx, run); err != nil {
		t.Fatalf("SaveDiscoveryRun: %v", err)
	}
	candidate := evaluation.DiscoveredCandidate{
		DiscoveredCandidateID: "candidate_fixture_api",
		TenantID:              "ten_fixture_api",
		DiscoveryRunID:        run.DiscoveryRunID,
		SourceKind:            evaluation.SourceKindRun,
		SourceID:              "run_fixture_api",
		SourceRefs:            []evaluation.SourceRef{{Kind: evaluation.SourceKindRun, ID: "run_fixture_api"}},
		Score:                 0.9,
		ScoreBand:             evaluation.ScoreBandHigh,
		RedactionStatus:       evaluation.RedactionStatusRedacted,
		ReadinessStatus:       evaluation.ReadinessFullyReplayable,
		SuppressionState:      evaluation.SuppressionStateNone,
		RetentionState:        evaluation.RetentionStateActive,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	evidence := evaluation.CandidateEvidence{
		EvidenceID:             "evidence_fixture_api",
		TenantID:               "ten_fixture_api",
		DiscoveredCandidateID:  candidate.DiscoveredCandidateID,
		RedactedPayload:        map[string]any{"goal": "safe"},
		MaterializationAllowed: true,
		RetentionState:         evaluation.RetentionStateActive,
		CreatedAt:              now,
	}
	if err := sqliteStore.SaveDiscoveredCandidate(ctx, candidate, evidence); err != nil {
		t.Fatalf("SaveDiscoveredCandidate: %v", err)
	}
}
