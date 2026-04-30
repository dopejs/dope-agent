package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

type EvaluationProductListResponse[T any] struct {
	TenantID string                 `json:"tenantId"`
	Page     evaluation.ProductPage `json:"page"`
	Items    []T                    `json:"items"`
}

type upsertDiscoveryPolicyRequest struct {
	Enabled              bool                    `json:"enabled"`
	SourceKinds          []evaluation.SourceKind `json:"sourceKinds"`
	WindowStart          time.Time               `json:"windowStart"`
	WindowEnd            time.Time               `json:"windowEnd"`
	MaxInspectedRecords  int                     `json:"maxInspectedRecords"`
	MaxEmittedCandidates int                     `json:"maxEmittedCandidates"`
	CostBudget           int                     `json:"costBudget"`
	SensitiveFieldRules  []string                `json:"sensitiveFieldRules,omitempty"`
	RetentionPolicyRef   string                  `json:"retentionPolicyRef,omitempty"`
	IdempotencyKey       string                  `json:"idempotencyKey,omitempty"`
}

type startDiscoveryRunRequest struct {
	PolicyID             string                  `json:"policyId,omitempty"`
	WindowStart          time.Time               `json:"windowStart,omitempty"`
	WindowEnd            time.Time               `json:"windowEnd,omitempty"`
	SourceKinds          []evaluation.SourceKind `json:"sourceKinds,omitempty"`
	MaxInspectedRecords  int                     `json:"maxInspectedRecords,omitempty"`
	MaxEmittedCandidates int                     `json:"maxEmittedCandidates,omitempty"`
	CostBudget           int                     `json:"costBudget,omitempty"`
	Cursor               string                  `json:"cursor,omitempty"`
	IdempotencyKey       string                  `json:"idempotencyKey,omitempty"`
}

type materializeProductFixtureRequest struct {
	FixtureID      string                        `json:"fixtureId,omitempty"`
	DisplayName    string                        `json:"displayName"`
	DomainClass    evaluation.FixtureDomainClass `json:"domainClass"`
	FixturePayload map[string]any                `json:"fixturePayload,omitempty"`
	ChangeSummary  string                        `json:"changeSummary,omitempty"`
	IdempotencyKey string                        `json:"idempotencyKey,omitempty"`
}

type createFixtureRevisionRequest struct {
	RevisionID         string         `json:"revisionId,omitempty"`
	FixturePayload     map[string]any `json:"fixturePayload,omitempty"`
	ContentSummary     string         `json:"contentSummary,omitempty"`
	ChangeSummary      string         `json:"changeSummary,omitempty"`
	SourceEvidenceRefs []string       `json:"sourceEvidenceRefs,omitempty"`
	IdempotencyKey     string         `json:"idempotencyKey,omitempty"`
}

type reviewProductFixtureRequest struct {
	RevisionID string                           `json:"revisionId"`
	Decision   evaluation.FixtureReviewDecision `json:"decision"`
	Reason     string                           `json:"reason,omitempty"`
}

type createCampaignRequest struct {
	CampaignID       string                           `json:"campaignId,omitempty"`
	DisplayName      string                           `json:"displayName"`
	ScopeSummary     string                           `json:"scopeSummary,omitempty"`
	SourceSelections []campaignSourceSelectionRequest `json:"sourceSelections,omitempty"`
	StartImmediately bool                             `json:"startImmediately,omitempty"`
	IdempotencyKey   string                           `json:"idempotencyKey,omitempty"`
}

type campaignSourceSelectionRequest struct {
	SourceType      evaluation.ProductResourceKind `json:"sourceType"`
	SourceID        string                         `json:"sourceId"`
	SourceSnapshot  map[string]any                 `json:"sourceSnapshot,omitempty"`
	SelectionReason string                         `json:"selectionReason,omitempty"`
}

type productFixtureMutationResponse struct {
	Fixture  evaluation.ProductManagedFixture `json:"fixture"`
	Revision evaluation.FixtureRevision       `json:"revision,omitempty"`
}

func handleEvaluationProductRoutes(sqliteStore *store.SQLiteStore, path string, w http.ResponseWriter, r *http.Request) bool {
	if !isEvaluationProductPath(path) {
		return false
	}
	if path == "retention/apply" && isEvaluationProductMutation(r) {
		writeError(w, http.StatusNotImplemented, "evaluation product mutations are not enabled")
		return true
	}
	if sqliteStore == nil {
		writeError(w, http.StatusInternalServerError, "evaluation product store is not configured")
		return true
	}
	tenantID, ok := evaluationProductTenantIDFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, evaluation.ErrEvaluationProductTenantRequired.Error())
		return true
	}

	switch {
	case path == "discovery-policies":
		handleEvaluationProductDiscoveryPolicies(sqliteStore, tenantID, w, r)
	case strings.HasPrefix(path, "discovery-policies/"):
		handleEvaluationProductDiscoveryPolicyRoutes(sqliteStore, tenantID, strings.TrimPrefix(path, "discovery-policies/"), w, r)
	case path == "discovery-runs":
		handleEvaluationProductDiscoveryRuns(sqliteStore, tenantID, w, r)
	case strings.HasPrefix(path, "discovery-runs/"):
		handleEvaluationProductDiscoveryRunRoutes(sqliteStore, tenantID, strings.TrimPrefix(path, "discovery-runs/"), w, r)
	case path == "discovered-candidates":
		handleEvaluationProductDiscoveredCandidates(sqliteStore, tenantID, w, r)
	case strings.HasPrefix(path, "discovered-candidates/"):
		handleEvaluationProductDiscoveredCandidateRoutes(sqliteStore, tenantID, strings.TrimPrefix(path, "discovered-candidates/"), w, r)
	case path == "product-fixtures":
		handleEvaluationProductFixtures(sqliteStore, tenantID, w, r)
	case strings.HasPrefix(path, "product-fixtures/"):
		handleEvaluationProductFixtureRoutes(sqliteStore, tenantID, strings.TrimPrefix(path, "product-fixtures/"), w, r)
	case path == "suppressions":
		handleEvaluationProductSuppressions(sqliteStore, tenantID, w, r)
	case path == "campaigns":
		handleEvaluationProductCampaigns(sqliteStore, tenantID, w, r)
	case path == "dashboard":
		handleEvaluationProductDashboard(sqliteStore, tenantID, w, r)
	case strings.HasPrefix(path, "tool-call-inspections/"):
		handleEvaluationProductToolCallInspectionRoutes(sqliteStore, tenantID, strings.TrimPrefix(path, "tool-call-inspections/"), w, r)
	case path == "retention/apply":
		writeError(w, http.StatusNotImplemented, "evaluation product mutations are not enabled")
	case strings.HasPrefix(path, "campaigns/"):
		handleEvaluationProductCampaignRoutes(sqliteStore, tenantID, strings.TrimPrefix(path, "campaigns/"), w, r)
	default:
		writeError(w, http.StatusNotFound, "evaluation product route not found")
	}
	return true
}

func handleEvaluationProductDiscoveryPolicies(sqliteStore *store.SQLiteStore, tenantID string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var enabled *bool
	if raw := r.URL.Query().Get("enabled"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "enabled must be a boolean")
			return
		}
		enabled = &value
	}
	filter := evaluation.DiscoveryPolicyFilter{
		ProductListFilter: evaluation.ProductListFilter{TenantID: tenantID, Cursor: r.URL.Query().Get("cursor"), Limit: queryInt(r, "limit")},
		Enabled:           enabled,
	}
	items, err := sqliteStore.ListDiscoveryPolicies(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, EvaluationProductListResponse[evaluation.DiscoveryPolicy]{TenantID: tenantID, Page: productPageFromRequest(r), Items: items})
}

func handleEvaluationProductDiscoveryPolicyRoutes(sqliteStore *store.SQLiteStore, tenantID, path string, w http.ResponseWriter, r *http.Request) {
	policyID := strings.Trim(path, "/")
	if policyID == "" || strings.Contains(policyID, "/") {
		writeError(w, http.StatusNotFound, "discovery policy route not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, ok, err := sqliteStore.GetDiscoveryPolicy(r.Context(), tenantID, policyID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			writeError(w, http.StatusNotFound, "discovery policy not found")
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodPut:
		var input upsertDiscoveryPolicyRequest
		if err := decodeOptionalJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		now := time.Now().UTC()
		item := evaluation.DiscoveryPolicy{
			PolicyID:             policyID,
			TenantID:             tenantID,
			Enabled:              input.Enabled,
			SourceKinds:          input.SourceKinds,
			WindowStart:          input.WindowStart,
			WindowEnd:            input.WindowEnd,
			MaxInspectedRecords:  input.MaxInspectedRecords,
			MaxEmittedCandidates: input.MaxEmittedCandidates,
			CostBudget:           input.CostBudget,
			SensitiveFieldRules:  input.SensitiveFieldRules,
			RetentionPolicyRef:   input.RetentionPolicyRef,
			CreatedBy:            evaluationProductPrincipalIDFromRequest(r),
			CreatedAt:            now,
			UpdatedAt:            now,
		}
		if err := sqliteStore.UpsertDiscoveryPolicy(r.Context(), item); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, item)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleEvaluationProductDiscoveryRuns(sqliteStore *store.SQLiteStore, tenantID string, w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var input startDiscoveryRunRequest
		if err := decodeOptionalJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		now := time.Now().UTC()
		policy := evaluation.DiscoveryPolicy{
			PolicyID:             input.PolicyID,
			TenantID:             tenantID,
			Enabled:              true,
			SourceKinds:          input.SourceKinds,
			WindowStart:          input.WindowStart,
			WindowEnd:            input.WindowEnd,
			MaxInspectedRecords:  input.MaxInspectedRecords,
			MaxEmittedCandidates: input.MaxEmittedCandidates,
			CostBudget:           input.CostBudget,
		}
		if input.PolicyID != "" {
			existing, ok, err := sqliteStore.GetDiscoveryPolicy(r.Context(), tenantID, input.PolicyID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !ok {
				writeError(w, http.StatusNotFound, "discovery policy not found")
				return
			}
			policy = existing
		}
		run, err := evaluation.BuildDiscoveryRunFromPolicy(policy, evaluation.StartDiscoveryRunInput{
			WindowStart:          input.WindowStart,
			WindowEnd:            input.WindowEnd,
			SourceKinds:          input.SourceKinds,
			MaxInspectedRecords:  input.MaxInspectedRecords,
			MaxEmittedCandidates: input.MaxEmittedCandidates,
			CostBudget:           input.CostBudget,
			Cursor:               input.Cursor,
			StartedBy:            evaluationProductPrincipalIDFromRequest(r),
			IdempotencyKey:       input.IdempotencyKey,
		}, now)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := sqliteStore.SaveDiscoveryRun(r.Context(), run); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, run)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	items, err := sqliteStore.ListDiscoveryRuns(r.Context(), evaluation.DiscoveryRunFilter{
		ProductListFilter: evaluation.ProductListFilter{TenantID: tenantID, Cursor: r.URL.Query().Get("cursor"), Limit: queryInt(r, "limit")},
		Status:            evaluation.ProductLifecycleStatus(r.URL.Query().Get("status")),
		SourceKind:        evaluation.SourceKind(r.URL.Query().Get("sourceKind")),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, EvaluationProductListResponse[evaluation.DiscoveryRun]{TenantID: tenantID, Page: productPageFromRequest(r), Items: items})
}

func handleEvaluationProductDiscoveryRunRoutes(sqliteStore *store.SQLiteStore, tenantID, path string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	discoveryRunID := strings.Trim(path, "/")
	item, ok, err := sqliteStore.GetDiscoveryRun(r.Context(), tenantID, discoveryRunID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "discovery run not found")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func handleEvaluationProductDiscoveredCandidates(sqliteStore *store.SQLiteStore, tenantID string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	items, err := sqliteStore.ListDiscoveredCandidates(r.Context(), evaluation.DiscoveredCandidateFilter{
		ProductListFilter: evaluation.ProductListFilter{TenantID: tenantID, Cursor: r.URL.Query().Get("cursor"), Limit: queryInt(r, "limit")},
		DiscoveryRunID:    r.URL.Query().Get("discoveryRunId"),
		SourceKind:        evaluation.SourceKind(r.URL.Query().Get("sourceKind")),
		ReadinessStatus:   evaluation.ReadinessStatus(r.URL.Query().Get("readinessStatus")),
		SuppressionState:  evaluation.SuppressionState(r.URL.Query().Get("suppressionState")),
		ScoreBand:         evaluation.ScoreBand(r.URL.Query().Get("scoreBand")),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, EvaluationProductListResponse[evaluation.DiscoveredCandidate]{TenantID: tenantID, Page: productPageFromRequest(r), Items: items})
}

func handleEvaluationProductDiscoveredCandidateRoutes(sqliteStore *store.SQLiteStore, tenantID, path string, w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 2 && parts[1] == "product-fixtures" {
		handleEvaluationProductFixtureMaterialization(sqliteStore, tenantID, parts[0], w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	discoveredCandidateID := strings.Trim(path, "/")
	item, ok, err := sqliteStore.GetDiscoveredCandidate(r.Context(), tenantID, discoveredCandidateID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "discovered candidate not found")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func handleEvaluationProductFixtureMaterialization(sqliteStore *store.SQLiteStore, tenantID, discoveredCandidateID string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !evaluationProductRequestHasPermission(r, identity.PermissionEvaluationFixtureManage) {
		writeError(w, http.StatusForbidden, "evaluation.fixture.manage is required")
		return
	}
	var input materializeProductFixtureRequest
	if err := decodeOptionalJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	candidate, ok, err := sqliteStore.GetDiscoveredCandidate(r.Context(), tenantID, discoveredCandidateID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "discovered candidate not found")
		return
	}
	evidence, ok, err := sqliteStore.GetLatestCandidateEvidence(r.Context(), tenantID, discoveredCandidateID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusBadRequest, "candidate evidence not found")
		return
	}
	fixture, revision, err := evaluation.CreateProductFixtureFromCandidate(evaluation.ProductFixtureInput{
		FixtureID:       input.FixtureID,
		TenantID:        tenantID,
		DisplayName:     input.DisplayName,
		DomainClass:     input.DomainClass,
		SourceCandidate: candidate,
		SourceEvidence:  evidence,
		FixturePayload:  input.FixturePayload,
		ChangeSummary:   input.ChangeSummary,
		CreatedBy:       evaluationProductPrincipalIDFromRequest(r),
		IdempotencyKey:  input.IdempotencyKey,
	}, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := sqliteStore.UpsertProductFixture(r.Context(), fixture); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := sqliteStore.SaveFixtureRevision(r.Context(), revision); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, productFixtureMutationResponse{Fixture: fixture, Revision: revision})
}

func handleEvaluationProductFixtures(sqliteStore *store.SQLiteStore, tenantID string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !evaluationProductRequestHasPermission(r, identity.PermissionEvaluationFixtureRead) {
		writeError(w, http.StatusForbidden, "evaluation.fixture.read is required")
		return
	}
	items, err := sqliteStore.ListProductFixtures(r.Context(), evaluation.ProductListFilter{TenantID: tenantID, Cursor: r.URL.Query().Get("cursor"), Limit: queryInt(r, "limit")})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, EvaluationProductListResponse[evaluation.ProductManagedFixture]{TenantID: tenantID, Page: productPageFromRequest(r), Items: items})
}

func handleEvaluationProductFixtureRoutes(sqliteStore *store.SQLiteStore, tenantID, path string, w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "product fixture route not found")
		return
	}
	fixtureID := parts[0]
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !evaluationProductRequestHasPermission(r, identity.PermissionEvaluationFixtureRead) {
			writeError(w, http.StatusForbidden, "evaluation.fixture.read is required")
			return
		}
		item, ok, err := sqliteStore.GetProductFixture(r.Context(), tenantID, fixtureID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			writeError(w, http.StatusNotFound, "product fixture not found")
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}
	switch parts[1] {
	case "revisions":
		handleEvaluationProductFixtureRevisions(sqliteStore, tenantID, fixtureID, w, r)
	case "review":
		handleEvaluationProductFixtureReview(sqliteStore, tenantID, fixtureID, w, r)
	case "suppress":
		handleEvaluationProductFixtureSuppress(sqliteStore, tenantID, fixtureID, w, r)
	default:
		writeError(w, http.StatusNotFound, "product fixture route not found")
	}
}

func handleEvaluationProductFixtureRevisions(sqliteStore *store.SQLiteStore, tenantID, fixtureID string, w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if !evaluationProductRequestHasPermission(r, identity.PermissionEvaluationFixtureRead) {
			writeError(w, http.StatusForbidden, "evaluation.fixture.read is required")
			return
		}
		items, err := sqliteStore.ListFixtureRevisions(r.Context(), tenantID, fixtureID, queryInt(r, "limit"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, EvaluationProductListResponse[evaluation.FixtureRevision]{TenantID: tenantID, Page: productPageFromRequest(r), Items: items})
	case http.MethodPost:
		if !evaluationProductRequestHasPermission(r, identity.PermissionEvaluationFixtureManage) {
			writeError(w, http.StatusForbidden, "evaluation.fixture.manage is required")
			return
		}
		var input createFixtureRevisionRequest
		if err := decodeOptionalJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		fixture, ok, err := sqliteStore.GetProductFixture(r.Context(), tenantID, fixtureID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			writeError(w, http.StatusNotFound, "product fixture not found")
			return
		}
		revisions, err := sqliteStore.ListFixtureRevisions(r.Context(), tenantID, fixtureID, 1)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		nextRevisionNumber := 1
		if len(revisions) > 0 {
			nextRevisionNumber = revisions[0].RevisionNumber + 1
		}
		updated, revision, err := evaluation.CreateProductFixtureRevision(fixture, evaluation.FixtureRevisionInput{
			RevisionID:         input.RevisionID,
			FixturePayload:     input.FixturePayload,
			ContentSummary:     input.ContentSummary,
			ChangeSummary:      input.ChangeSummary,
			SourceEvidenceRefs: input.SourceEvidenceRefs,
			RedactionStatus:    evaluation.RedactionStatusClean,
			CreatedBy:          evaluationProductPrincipalIDFromRequest(r),
		}, nextRevisionNumber, time.Now().UTC())
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := sqliteStore.UpsertProductFixture(r.Context(), updated); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := sqliteStore.SaveFixtureRevision(r.Context(), revision); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, productFixtureMutationResponse{Fixture: updated, Revision: revision})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleEvaluationProductFixtureReview(sqliteStore *store.SQLiteStore, tenantID, fixtureID string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !evaluationProductRequestHasPermission(r, identity.PermissionEvaluationFixtureReview) {
		writeError(w, http.StatusForbidden, "evaluation.fixture.review is required")
		return
	}
	var input reviewProductFixtureRequest
	if err := decodeOptionalJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	fixture, ok, err := sqliteStore.GetProductFixture(r.Context(), tenantID, fixtureID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "product fixture not found")
		return
	}
	updated, err := evaluation.ReviewProductFixture(fixture, input.RevisionID, input.Decision, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := sqliteStore.UpsertProductFixture(r.Context(), updated); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, productFixtureMutationResponse{Fixture: updated})
}

func handleEvaluationProductFixtureSuppress(sqliteStore *store.SQLiteStore, tenantID, fixtureID string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !evaluationProductRequestHasPermission(r, identity.PermissionEvaluationFixtureSuppress) {
		writeError(w, http.StatusForbidden, "evaluation.fixture.suppress is required")
		return
	}
	fixture, ok, err := sqliteStore.GetProductFixture(r.Context(), tenantID, fixtureID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "product fixture not found")
		return
	}
	updated, err := evaluation.SuppressProductFixture(fixture, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := sqliteStore.UpsertProductFixture(r.Context(), updated); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, productFixtureMutationResponse{Fixture: updated})
}

func handleEvaluationProductSuppressions(sqliteStore *store.SQLiteStore, tenantID string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var item evaluation.SuppressionRecord
	if err := decodeOptionalJSON(r, &item); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	now := time.Now().UTC()
	item.TenantID = tenantID
	if item.SuppressionID == "" {
		item.SuppressionID = "suppression_" + strconv.FormatInt(now.UnixNano(), 10)
	}
	if item.CreatedBy == "" {
		item.CreatedBy = evaluationProductPrincipalIDFromRequest(r)
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.Active = true
	if err := sqliteStore.CreateSuppression(r.Context(), item); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func handleEvaluationProductCampaigns(sqliteStore *store.SQLiteStore, tenantID string, w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if !evaluationProductRequestHasPermission(r, identity.PermissionEvaluationCampaignManage) {
			writeError(w, http.StatusForbidden, "evaluation.campaign.manage is required")
			return
		}
		var input createCampaignRequest
		if err := decodeOptionalJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		selections, err := campaignSourceSelectionsFromRequest(sqliteStore, r, tenantID, input.SourceSelections)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		campaign, items, err := evaluation.CreateReplayCampaign(evaluation.CreateCampaignInput{
			CampaignID:       input.CampaignID,
			TenantID:         tenantID,
			DisplayName:      input.DisplayName,
			ScopeSummary:     input.ScopeSummary,
			StartedBy:        evaluationProductPrincipalIDFromRequest(r),
			IdempotencyKey:   input.IdempotencyKey,
			SourceSelections: selections,
			StartImmediately: input.StartImmediately,
		}, time.Now().UTC())
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := sqliteStore.SaveReplayCampaign(r.Context(), campaign); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, item := range items {
			if err := sqliteStore.SaveCampaignItem(r.Context(), item); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		writeJSON(w, http.StatusCreated, campaign)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !evaluationProductRequestHasPermission(r, identity.PermissionEvaluationCampaignRead) {
		writeError(w, http.StatusForbidden, "evaluation.campaign.read is required")
		return
	}
	items, err := sqliteStore.ListReplayCampaigns(r.Context(), evaluation.ProductListFilter{TenantID: tenantID, Cursor: r.URL.Query().Get("cursor"), Limit: queryInt(r, "limit")})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, EvaluationProductListResponse[evaluation.ReplayCampaign]{TenantID: tenantID, Page: productPageFromRequest(r), Items: items})
}

func handleEvaluationProductDashboard(sqliteStore *store.SQLiteStore, tenantID string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !evaluationProductRequestHasPermission(r, identity.PermissionEvaluationDashboardRead) {
		writeError(w, http.StatusForbidden, "evaluation.dashboard.read is required")
		return
	}
	items, err := sqliteStore.ListDashboardProjections(r.Context(), evaluation.ProductListFilter{TenantID: tenantID, Cursor: r.URL.Query().Get("cursor"), Limit: queryInt(r, "limit")})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, EvaluationProductListResponse[evaluation.DashboardProjection]{TenantID: tenantID, Page: productPageFromRequest(r), Items: items})
}

func handleEvaluationProductCampaignRoutes(sqliteStore *store.SQLiteStore, tenantID, path string, w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 1 && parts[0] != "" {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !evaluationProductRequestHasPermission(r, identity.PermissionEvaluationCampaignRead) {
			writeError(w, http.StatusForbidden, "evaluation.campaign.read is required")
			return
		}
		item, ok, err := sqliteStore.GetReplayCampaign(r.Context(), tenantID, parts[0])
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			writeError(w, http.StatusNotFound, "campaign not found")
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}
	if len(parts) == 2 && parts[1] == "items" {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !evaluationProductRequestHasPermission(r, identity.PermissionEvaluationCampaignRead) {
			writeError(w, http.StatusForbidden, "evaluation.campaign.read is required")
			return
		}
		items, err := sqliteStore.ListCampaignItems(r.Context(), evaluation.ProductListFilter{TenantID: tenantID, Cursor: r.URL.Query().Get("cursor"), Limit: queryInt(r, "limit")}, parts[0])
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, EvaluationProductListResponse[evaluation.CampaignItem]{TenantID: tenantID, Page: productPageFromRequest(r), Items: items})
		return
	}
	if len(parts) == 2 && parts[1] == "attempt-groups" {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !evaluationProductRequestHasPermission(r, identity.PermissionEvaluationCampaignRead) {
			writeError(w, http.StatusForbidden, "evaluation.campaign.read is required")
			return
		}
		items, err := sqliteStore.ListCampaignAttemptGroups(r.Context(), evaluation.ProductListFilter{TenantID: tenantID, Cursor: r.URL.Query().Get("cursor"), Limit: queryInt(r, "limit")}, parts[0])
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, EvaluationProductListResponse[evaluation.CampaignAttemptGroup]{TenantID: tenantID, Page: productPageFromRequest(r), Items: items})
		return
	}
	if len(parts) == 2 && (parts[1] == "start" || parts[1] == "complete" || parts[1] == "cancel" || parts[1] == "publish-results") {
		handleEvaluationProductCampaignTransition(sqliteStore, tenantID, parts[0], parts[1], w, r)
		return
	}
	if len(parts) == 2 && parts[1] == "tool-call-inspections" {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !evaluationProductRequestHasPermission(r, identity.PermissionEvaluationInspectionRead) {
			writeError(w, http.StatusForbidden, "evaluation.inspection.read is required")
			return
		}
		items, err := sqliteStore.ListToolCallInspections(r.Context(), evaluation.ProductListFilter{TenantID: tenantID, Cursor: r.URL.Query().Get("cursor"), Limit: queryInt(r, "limit")}, parts[0])
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, EvaluationProductListResponse[evaluation.ToolCallInspection]{TenantID: tenantID, Page: productPageFromRequest(r), Items: items})
		return
	}
	writeError(w, http.StatusNotFound, "evaluation product campaign route not found")
}

func handleEvaluationProductCampaignTransition(sqliteStore *store.SQLiteStore, tenantID, campaignID, action string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !evaluationProductRequestHasPermission(r, identity.PermissionEvaluationCampaignManage) {
		writeError(w, http.StatusForbidden, "evaluation.campaign.manage is required")
		return
	}
	campaign, ok, err := sqliteStore.GetReplayCampaign(r.Context(), tenantID, campaignID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "campaign not found")
		return
	}
	transition := evaluation.CampaignTransition(action)
	if action == "publish-results" {
		transition = evaluation.CampaignTransitionPublish
	}
	updated, err := evaluation.TransitionReplayCampaign(campaign, transition, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := sqliteStore.SaveReplayCampaign(r.Context(), updated); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func handleEvaluationProductToolCallInspectionRoutes(sqliteStore *store.SQLiteStore, tenantID, path string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !evaluationProductRequestHasPermission(r, identity.PermissionEvaluationInspectionRead) {
		writeError(w, http.StatusForbidden, "evaluation.inspection.read is required")
		return
	}
	inspectionID := strings.Trim(path, "/")
	if inspectionID == "" || strings.Contains(inspectionID, "/") {
		writeError(w, http.StatusNotFound, "tool-call inspection route not found")
		return
	}
	item, ok, err := sqliteStore.GetToolCallInspection(r.Context(), tenantID, inspectionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "tool-call inspection not found")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func campaignSourceSelectionsFromRequest(sqliteStore *store.SQLiteStore, r *http.Request, tenantID string, inputs []campaignSourceSelectionRequest) ([]evaluation.CampaignSourceSelection, error) {
	selections := make([]evaluation.CampaignSourceSelection, 0, len(inputs))
	for _, input := range inputs {
		selection := evaluation.CampaignSourceSelection{
			SourceType:      input.SourceType,
			SourceID:        input.SourceID,
			TenantID:        tenantID,
			SourceSnapshot:  input.SourceSnapshot,
			SelectionReason: input.SelectionReason,
		}
		switch input.SourceType {
		case evaluation.ProductResourceProductFixture:
			fixture, ok, err := sqliteStore.GetProductFixture(r.Context(), tenantID, input.SourceID)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, evaluation.ErrEvaluationCampaignSelectionInvalid
			}
			selection.SuppressionState = fixture.SuppressionState
			selection.RetentionState = fixture.RetentionState
			selection.ReviewState = fixture.ReviewState
			selection.SourceSnapshot = map[string]any{
				"fixtureId":         fixture.FixtureID,
				"displayName":       fixture.DisplayName,
				"currentRevisionId": fixture.CurrentRevisionID,
				"reviewState":       fixture.ReviewState,
				"retentionState":    fixture.RetentionState,
				"suppressionState":  fixture.SuppressionState,
			}
		case evaluation.ProductResourceDiscoveredCandidate:
			candidate, ok, err := sqliteStore.GetDiscoveredCandidate(r.Context(), tenantID, input.SourceID)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, evaluation.ErrEvaluationCampaignSelectionInvalid
			}
			selection.SuppressionState = candidate.SuppressionState
			selection.RetentionState = candidate.RetentionState
			selection.SourceSnapshot = map[string]any{
				"discoveredCandidateId": candidate.DiscoveredCandidateID,
				"sourceKind":            candidate.SourceKind,
				"sourceId":              candidate.SourceID,
				"score":                 candidate.Score,
				"scoreBand":             candidate.ScoreBand,
				"readinessStatus":       candidate.ReadinessStatus,
				"retentionState":        candidate.RetentionState,
				"suppressionState":      candidate.SuppressionState,
			}
		default:
			if selection.RetentionState == "" {
				selection.RetentionState = evaluation.RetentionStateActive
			}
			if selection.SuppressionState == "" {
				selection.SuppressionState = evaluation.SuppressionStateNone
			}
		}
		selections = append(selections, selection)
	}
	return selections, nil
}

func isEvaluationProductPath(path string) bool {
	switch {
	case path == "discovery-policies",
		strings.HasPrefix(path, "discovery-policies/"),
		path == "discovery-runs",
		strings.HasPrefix(path, "discovery-runs/"),
		path == "discovered-candidates",
		strings.HasPrefix(path, "discovered-candidates/"),
		path == "product-fixtures",
		strings.HasPrefix(path, "product-fixtures/"),
		path == "suppressions",
		path == "campaigns",
		path == "dashboard",
		strings.HasPrefix(path, "tool-call-inspections/"),
		path == "retention/apply",
		strings.HasPrefix(path, "campaigns/"):
		return true
	default:
		return false
	}
}

func isEvaluationProductMutation(r *http.Request) bool {
	return r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch || r.Method == http.MethodDelete
}

func evaluationProductTenantIDFromRequest(r *http.Request) (string, bool) {
	if tenantContext, ok := tenantctx.FromContext(r.Context()); ok && strings.TrimSpace(tenantContext.TenantID) != "" {
		return strings.TrimSpace(tenantContext.TenantID), true
	}
	return "", false
}

func evaluationProductPrincipalIDFromRequest(r *http.Request) string {
	if tenantContext, ok := tenantctx.FromContext(r.Context()); ok {
		return strings.TrimSpace(tenantContext.PrincipalID)
	}
	return ""
}

func evaluationProductRequestHasPermission(r *http.Request, permission identity.Permission) bool {
	tenantContext, ok := tenantctx.FromContext(r.Context())
	if !ok {
		return false
	}
	for _, current := range tenantContext.Permissions {
		if current == permission || current == identity.PermissionEvaluationManage {
			return true
		}
	}
	return false
}

func productPageFromRequest(r *http.Request) evaluation.ProductPage {
	return evaluation.ProductPage{
		Cursor: r.URL.Query().Get("cursor"),
		Limit:  evaluation.NormalizeProductLimit(queryInt(r, "limit")),
	}
}
