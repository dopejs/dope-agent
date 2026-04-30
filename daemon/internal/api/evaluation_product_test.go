package api

import (
	"bytes"
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

func TestEvaluationProductRoutesListTenantScopedPoliciesWithoutManager(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	now := time.Now().UTC()
	if err := sqliteStore.UpsertDiscoveryPolicy(context.Background(), evaluation.DiscoveryPolicy{
		PolicyID:             "policy_api",
		TenantID:             "ten_api",
		Enabled:              true,
		SourceKinds:          []evaluation.SourceKind{evaluation.SourceKindRun},
		WindowStart:          now.Add(-time.Hour),
		WindowEnd:            now,
		MaxInspectedRecords:  10,
		MaxEmittedCandidates: 2,
		CostBudget:           5,
		CreatedAt:            now,
		UpdatedAt:            now,
	}); err != nil {
		t.Fatalf("UpsertDiscoveryPolicy: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/evaluation/discovery-policies", nil)
	req = req.WithContext(tenantctx.WithContext(req.Context(), identity.TenantContext{TenantID: "ten_api", PrincipalID: "prn_api"}))
	rec := httptest.NewRecorder()

	handleEvaluationRoutes(nil, nil, nil, sqliteStore, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !containsAll(body, "policy_api", "ten_api") {
		t.Fatalf("response missing tenant policy: %s", body)
	}
}

func TestEvaluationProductDiscoveryAPIRoutes(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_api", PrincipalID: "prn_api"})
	now := time.Now().UTC()
	putPolicy := httptest.NewRequest(http.MethodPut, "/v1/evaluation/discovery-policies/policy_api_1", jsonBody(map[string]any{
		"enabled":              true,
		"sourceKinds":          []string{"run"},
		"windowStart":          now.Add(-time.Hour).Format(time.RFC3339Nano),
		"windowEnd":            now.Format(time.RFC3339Nano),
		"maxInspectedRecords":  10,
		"maxEmittedCandidates": 2,
		"costBudget":           5,
	}))
	putPolicy = putPolicy.WithContext(ctx)
	putPolicyRec := httptest.NewRecorder()
	handleEvaluationRoutes(nil, nil, nil, sqliteStore, putPolicyRec, putPolicy)
	if putPolicyRec.Code != http.StatusOK {
		t.Fatalf("PUT policy status=%d body=%s", putPolicyRec.Code, putPolicyRec.Body.String())
	}

	getPolicy := httptest.NewRequest(http.MethodGet, "/v1/evaluation/discovery-policies/policy_api_1", nil).WithContext(ctx)
	getPolicyRec := httptest.NewRecorder()
	handleEvaluationRoutes(nil, nil, nil, sqliteStore, getPolicyRec, getPolicy)
	if getPolicyRec.Code != http.StatusOK || !strings.Contains(getPolicyRec.Body.String(), "policy_api_1") {
		t.Fatalf("GET policy status=%d body=%s", getPolicyRec.Code, getPolicyRec.Body.String())
	}

	startRun := httptest.NewRequest(http.MethodPost, "/v1/evaluation/discovery-runs", jsonBody(map[string]any{
		"policyId":       "policy_api_1",
		"idempotencyKey": "idem_api_1",
	}))
	startRun = startRun.WithContext(ctx)
	startRunRec := httptest.NewRecorder()
	handleEvaluationRoutes(nil, nil, nil, sqliteStore, startRunRec, startRun)
	if startRunRec.Code != http.StatusAccepted || !strings.Contains(startRunRec.Body.String(), "idem_api_1") {
		t.Fatalf("POST run status=%d body=%s", startRunRec.Code, startRunRec.Body.String())
	}

	var run evaluation.DiscoveryRun
	if err := json.Unmarshal(startRunRec.Body.Bytes(), &run); err != nil {
		t.Fatalf("decode run: %v", err)
	}
	if run.DiscoveryRunID == "" {
		t.Fatalf("run id missing: %+v", run)
	}
	candidate := evaluation.DiscoveredCandidate{
		DiscoveredCandidateID: "candidate_api_1",
		TenantID:              "ten_api",
		DiscoveryRunID:        run.DiscoveryRunID,
		SourceKind:            evaluation.SourceKindRun,
		SourceID:              "run_source_1",
		Score:                 0.9,
		ScoreBand:             evaluation.ScoreBandHigh,
		RedactionStatus:       evaluation.RedactionStatusRedacted,
		ReadinessStatus:       evaluation.ReadinessFullyReplayable,
		SuppressionState:      evaluation.SuppressionStateNone,
		RetentionState:        evaluation.RetentionStateActive,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := sqliteStore.SaveDiscoveredCandidate(ctx, candidate, evaluation.CandidateEvidence{
		EvidenceID:            "evidence_api_1",
		TenantID:              "ten_api",
		DiscoveredCandidateID: candidate.DiscoveredCandidateID,
		RetentionState:        evaluation.RetentionStateActive,
		CreatedAt:             now,
	}); err != nil {
		t.Fatalf("SaveDiscoveredCandidate: %v", err)
	}

	getCandidate := httptest.NewRequest(http.MethodGet, "/v1/evaluation/discovered-candidates/candidate_api_1", nil).WithContext(ctx)
	getCandidateRec := httptest.NewRecorder()
	handleEvaluationRoutes(nil, nil, nil, sqliteStore, getCandidateRec, getCandidate)
	if getCandidateRec.Code != http.StatusOK || !strings.Contains(getCandidateRec.Body.String(), "candidate_api_1") {
		t.Fatalf("GET candidate status=%d body=%s", getCandidateRec.Code, getCandidateRec.Body.String())
	}

	createSuppression := httptest.NewRequest(http.MethodPost, "/v1/evaluation/suppressions", jsonBody(map[string]any{
		"suppressionId": "suppression_api_1",
		"targetKind":    "discovered_candidate",
		"targetId":      "candidate_api_1",
		"reasonCode":    "operator_hidden",
		"reason":        "hidden in test",
	}))
	createSuppression = createSuppression.WithContext(ctx)
	suppressionRec := httptest.NewRecorder()
	handleEvaluationRoutes(nil, nil, nil, sqliteStore, suppressionRec, createSuppression)
	if suppressionRec.Code != http.StatusCreated || !strings.Contains(suppressionRec.Body.String(), "suppression_api_1") {
		t.Fatalf("POST suppression status=%d body=%s", suppressionRec.Code, suppressionRec.Body.String())
	}
}

func TestEvaluationProductRoutesDoNotEnableMutationsAtScaffold(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/v1/evaluation/retention/apply", nil)
	req = req.WithContext(tenantctx.WithContext(req.Context(), identity.TenantContext{TenantID: "ten_api", PrincipalID: "prn_api"}))
	rec := httptest.NewRecorder()

	handleEvaluationRoutes(nil, nil, nil, nil, rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func containsAll(value string, needles ...string) bool {
	for _, needle := range needles {
		if !strings.Contains(value, needle) {
			return false
		}
	}
	return true
}

func jsonBody(value any) *bytes.Reader {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return bytes.NewReader(encoded)
}
