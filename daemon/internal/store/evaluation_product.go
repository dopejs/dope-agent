package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
)

func (s *SQLiteStore) UpsertDiscoveryPolicy(ctx context.Context, item evaluation.DiscoveryPolicy) error {
	if s == nil {
		return nil
	}
	tenantID, err := s.evaluationProductTenantID(ctx, item.TenantID)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	if err := evaluation.ValidateDiscoveryPolicy(item); err != nil {
		return err
	}
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal evaluation discovery policy: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO evaluation_discovery_policies (
			policy_id, tenant_id, enabled, window_start, window_end, max_inspected_records,
			max_emitted_candidates, cost_budget, created_by, created_at, updated_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(policy_id) DO UPDATE SET
			tenant_id = COALESCE(evaluation_discovery_policies.tenant_id, excluded.tenant_id),
			enabled = excluded.enabled,
			window_start = excluded.window_start,
			window_end = excluded.window_end,
			max_inspected_records = excluded.max_inspected_records,
			max_emitted_candidates = excluded.max_emitted_candidates,
			cost_budget = excluded.cost_budget,
			created_by = excluded.created_by,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json
	`, item.PolicyID, tenantID, boolToInt(item.Enabled), formatProductTime(item.WindowStart), formatProductTime(item.WindowEnd),
		item.MaxInspectedRecords, item.MaxEmittedCandidates, item.CostBudget, nullString(item.CreatedBy),
		formatProductTime(item.CreatedAt), formatProductTime(item.UpdatedAt), string(document))
	if err != nil {
		return fmt.Errorf("upsert evaluation discovery policy %s: %w", item.PolicyID, err)
	}
	return nil
}

func (s *SQLiteStore) ListDiscoveryPolicies(ctx context.Context, filter evaluation.DiscoveryPolicyFilter) ([]evaluation.DiscoveryPolicy, error) {
	tenantID, err := s.evaluationProductFilterTenantID(ctx, filter.TenantID)
	if err != nil {
		return nil, err
	}
	query := `SELECT document_json FROM evaluation_discovery_policies WHERE tenant_id = ?`
	args := []any{tenantID}
	if filter.Enabled != nil {
		query += ` AND enabled = ?`
		args = append(args, boolToInt(*filter.Enabled))
	}
	if filter.Cursor != "" {
		query += ` AND policy_id < ?`
		args = append(args, filter.Cursor)
	}
	query += ` ORDER BY updated_at DESC, policy_id DESC LIMIT ?`
	args = append(args, evaluation.NormalizeProductLimit(filter.Limit))
	return scanEvaluationProductDocuments[evaluation.DiscoveryPolicy](ctx, s.db, query, args, "discovery policies")
}

func (s *SQLiteStore) GetDiscoveryPolicy(ctx context.Context, tenantID, policyID string) (evaluation.DiscoveryPolicy, bool, error) {
	return getEvaluationProductDocument[evaluation.DiscoveryPolicy](ctx, s.db, "evaluation_discovery_policies", "policy_id", tenantID, policyID, "discovery policy")
}

func (s *SQLiteStore) SaveDiscoveryRun(ctx context.Context, item evaluation.DiscoveryRun) error {
	if s == nil {
		return nil
	}
	tenantID, err := s.evaluationProductTenantID(ctx, item.TenantID)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal evaluation discovery run: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO evaluation_discovery_runs (
			discovery_run_id, tenant_id, policy_id, status, cursor, window_start, window_end,
			max_inspected_records, max_emitted_candidates, cost_budget, inspected_records,
			emitted_candidates, started_by, started_at, completed_at, updated_at,
			idempotency_key, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(discovery_run_id) DO UPDATE SET
			tenant_id = COALESCE(evaluation_discovery_runs.tenant_id, excluded.tenant_id),
			policy_id = excluded.policy_id,
			status = excluded.status,
			cursor = excluded.cursor,
			inspected_records = excluded.inspected_records,
			emitted_candidates = excluded.emitted_candidates,
			completed_at = excluded.completed_at,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json
	`, item.DiscoveryRunID, tenantID, nullString(item.PolicyID), string(item.Status), nullString(item.Cursor),
		formatProductTime(item.WindowStart), formatProductTime(item.WindowEnd), item.MaxInspectedRecords,
		item.MaxEmittedCandidates, item.CostBudget, item.InspectedRecords, item.EmittedCandidates,
		nullString(item.StartedBy), formatProductTime(item.StartedAt), nullableTimeString(item.CompletedAt),
		formatProductTime(item.UpdatedAt), nullString(item.IdempotencyKey), string(document))
	if err != nil {
		return fmt.Errorf("save evaluation discovery run %s: %w", item.DiscoveryRunID, err)
	}
	return nil
}

func (s *SQLiteStore) ListDiscoveryRuns(ctx context.Context, filter evaluation.DiscoveryRunFilter) ([]evaluation.DiscoveryRun, error) {
	tenantID, err := s.evaluationProductFilterTenantID(ctx, filter.TenantID)
	if err != nil {
		return nil, err
	}
	query := `SELECT document_json FROM evaluation_discovery_runs WHERE tenant_id = ?`
	args := []any{tenantID}
	if filter.Status != "" {
		query += ` AND status = ?`
		args = append(args, string(filter.Status))
	}
	if filter.Cursor != "" {
		query += ` AND discovery_run_id < ?`
		args = append(args, filter.Cursor)
	}
	query += ` ORDER BY updated_at DESC, discovery_run_id DESC LIMIT ?`
	args = append(args, evaluation.NormalizeProductLimit(filter.Limit))
	items, err := scanEvaluationProductDocuments[evaluation.DiscoveryRun](ctx, s.db, query, args, "discovery runs")
	if err != nil || filter.SourceKind == "" {
		return items, err
	}
	filtered := items[:0]
	for _, item := range items {
		if productSourceKindsContain(item.SourceKinds, filter.SourceKind) {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (s *SQLiteStore) GetDiscoveryRun(ctx context.Context, tenantID, discoveryRunID string) (evaluation.DiscoveryRun, bool, error) {
	return getEvaluationProductDocument[evaluation.DiscoveryRun](ctx, s.db, "evaluation_discovery_runs", "discovery_run_id", tenantID, discoveryRunID, "discovery run")
}

func (s *SQLiteStore) SaveDiscoveredCandidate(ctx context.Context, item evaluation.DiscoveredCandidate, evidence evaluation.CandidateEvidence) error {
	if s == nil {
		return nil
	}
	tenantID, err := s.evaluationProductTenantID(ctx, item.TenantID)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	evidence.TenantID = tenantID
	if evidence.DiscoveredCandidateID == "" {
		evidence.DiscoveredCandidateID = item.DiscoveredCandidateID
	}
	if evidence.EvidenceID != "" && item.EvidenceRef == "" {
		item.EvidenceRef = evidence.EvidenceID
	}
	candidateJSON, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal evaluation discovered candidate: %w", err)
	}
	evidenceJSON, err := json.Marshal(evidence)
	if err != nil {
		return fmt.Errorf("marshal evaluation candidate evidence: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin save evaluation discovered candidate: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO evaluation_discovered_candidates (
			discovered_candidate_id, tenant_id, discovery_run_id, source_kind, source_id,
			score, score_band, redaction_status, evidence_ref, readiness_status,
			suppression_state, retention_state, created_at, updated_at, expires_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(discovered_candidate_id) DO UPDATE SET
			tenant_id = COALESCE(evaluation_discovered_candidates.tenant_id, excluded.tenant_id),
			discovery_run_id = excluded.discovery_run_id,
			source_kind = excluded.source_kind,
			source_id = excluded.source_id,
			score = excluded.score,
			score_band = excluded.score_band,
			redaction_status = excluded.redaction_status,
			evidence_ref = excluded.evidence_ref,
			readiness_status = excluded.readiness_status,
			suppression_state = excluded.suppression_state,
			retention_state = excluded.retention_state,
			updated_at = excluded.updated_at,
			expires_at = excluded.expires_at,
			document_json = excluded.document_json
	`, item.DiscoveredCandidateID, tenantID, item.DiscoveryRunID, string(item.SourceKind), item.SourceID,
		item.Score, string(item.ScoreBand), string(item.RedactionStatus), nullString(item.EvidenceRef),
		string(item.ReadinessStatus), string(item.SuppressionState), string(item.RetentionState),
		formatProductTime(item.CreatedAt), formatProductTime(item.UpdatedAt), nullableTimeString(item.ExpiresAt), string(candidateJSON)); err != nil {
		return fmt.Errorf("save evaluation discovered candidate %s: %w", item.DiscoveredCandidateID, err)
	}
	if evidence.EvidenceID != "" {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO evaluation_candidate_evidence (
				evidence_id, tenant_id, discovered_candidate_id, redaction_status,
				materialization_allowed, retention_state, created_at, expires_at, document_json
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(evidence_id) DO UPDATE SET
				tenant_id = COALESCE(evaluation_candidate_evidence.tenant_id, excluded.tenant_id),
				discovered_candidate_id = excluded.discovered_candidate_id,
				redaction_status = excluded.redaction_status,
				materialization_allowed = excluded.materialization_allowed,
				retention_state = excluded.retention_state,
				expires_at = excluded.expires_at,
				document_json = excluded.document_json
		`, evidence.EvidenceID, tenantID, evidence.DiscoveredCandidateID, string(candidateEvidenceRedactionStatus(evidence)),
			boolToInt(evidence.MaterializationAllowed), string(evidence.RetentionState), formatProductTime(evidence.CreatedAt),
			nullableTimeString(evidence.ExpiresAt), string(evidenceJSON)); err != nil {
			return fmt.Errorf("save evaluation candidate evidence %s: %w", evidence.EvidenceID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit save evaluation discovered candidate %s: %w", item.DiscoveredCandidateID, err)
	}
	return nil
}

func (s *SQLiteStore) ListDiscoveredCandidates(ctx context.Context, filter evaluation.DiscoveredCandidateFilter) ([]evaluation.DiscoveredCandidate, error) {
	tenantID, err := s.evaluationProductFilterTenantID(ctx, filter.TenantID)
	if err != nil {
		return nil, err
	}
	query := `SELECT document_json FROM evaluation_discovered_candidates WHERE tenant_id = ?`
	args := []any{tenantID}
	if filter.DiscoveryRunID != "" {
		query += ` AND discovery_run_id = ?`
		args = append(args, filter.DiscoveryRunID)
	}
	if filter.SourceKind != "" {
		query += ` AND source_kind = ?`
		args = append(args, string(filter.SourceKind))
	}
	if filter.ReadinessStatus != "" {
		query += ` AND readiness_status = ?`
		args = append(args, string(filter.ReadinessStatus))
	}
	if filter.SuppressionState != "" {
		query += ` AND suppression_state = ?`
		args = append(args, string(filter.SuppressionState))
	}
	if filter.ScoreBand != "" {
		query += ` AND score_band = ?`
		args = append(args, string(filter.ScoreBand))
	}
	if filter.Cursor != "" {
		query += ` AND discovered_candidate_id < ?`
		args = append(args, filter.Cursor)
	}
	query += ` ORDER BY updated_at DESC, discovered_candidate_id DESC LIMIT ?`
	args = append(args, evaluation.NormalizeProductLimit(filter.Limit))
	return scanEvaluationProductDocuments[evaluation.DiscoveredCandidate](ctx, s.db, query, args, "discovered candidates")
}

func (s *SQLiteStore) GetDiscoveredCandidate(ctx context.Context, tenantID, discoveredCandidateID string) (evaluation.DiscoveredCandidate, bool, error) {
	return getEvaluationProductDocument[evaluation.DiscoveredCandidate](ctx, s.db, "evaluation_discovered_candidates", "discovered_candidate_id", tenantID, discoveredCandidateID, "discovered candidate")
}

func (s *SQLiteStore) GetLatestCandidateEvidence(ctx context.Context, tenantID, discoveredCandidateID string) (evaluation.CandidateEvidence, bool, error) {
	tenantID, err := s.evaluationProductFilterTenantID(ctx, tenantID)
	if err != nil {
		return evaluation.CandidateEvidence{}, false, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json FROM evaluation_candidate_evidence
		WHERE tenant_id = ? AND discovered_candidate_id = ?
		ORDER BY created_at DESC, evidence_id DESC
		LIMIT 1
	`, tenantID, discoveredCandidateID)
	if err != nil {
		return evaluation.CandidateEvidence{}, false, fmt.Errorf("get latest evaluation candidate evidence %s: %w", discoveredCandidateID, err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return evaluation.CandidateEvidence{}, false, fmt.Errorf("scan latest evaluation candidate evidence %s: %w", discoveredCandidateID, err)
		}
		return evaluation.CandidateEvidence{}, false, nil
	}
	var raw []byte
	if err := rows.Scan(&raw); err != nil {
		return evaluation.CandidateEvidence{}, false, fmt.Errorf("scan latest evaluation candidate evidence %s: %w", discoveredCandidateID, err)
	}
	var item evaluation.CandidateEvidence
	if err := json.Unmarshal(raw, &item); err != nil {
		return evaluation.CandidateEvidence{}, false, fmt.Errorf("decode latest evaluation candidate evidence %s: %w", discoveredCandidateID, err)
	}
	return item, true, nil
}

func (s *SQLiteStore) CreateSuppression(ctx context.Context, item evaluation.SuppressionRecord) error {
	if s == nil {
		return nil
	}
	tenantID, err := s.evaluationProductTenantID(ctx, item.TenantID)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal evaluation suppression: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO evaluation_suppressions (
			suppression_id, tenant_id, target_kind, target_id, target_source_ref,
			reason_code, created_by, active, created_at, expires_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(suppression_id) DO UPDATE SET
			tenant_id = COALESCE(evaluation_suppressions.tenant_id, excluded.tenant_id),
			target_kind = excluded.target_kind,
			target_id = excluded.target_id,
			target_source_ref = excluded.target_source_ref,
			reason_code = excluded.reason_code,
			created_by = excluded.created_by,
			active = excluded.active,
			expires_at = excluded.expires_at,
			document_json = excluded.document_json
	`, item.SuppressionID, tenantID, string(item.TargetKind), nullString(item.TargetID), nullString(item.TargetSourceRef),
		item.ReasonCode, nullString(item.CreatedBy), boolToInt(item.Active), formatProductTime(item.CreatedAt),
		nullableTimeString(item.ExpiresAt), string(document))
	if err != nil {
		return fmt.Errorf("create evaluation suppression %s: %w", item.SuppressionID, err)
	}
	return nil
}

func (s *SQLiteStore) ApplyRetention(ctx context.Context, filter evaluation.RetentionApplicationFilter) error {
	tenantID, err := s.evaluationProductFilterTenantID(ctx, filter.TenantID)
	if err != nil {
		return err
	}
	kinds := filter.ResourceKinds
	if len(kinds) == 0 {
		kinds = []evaluation.ProductResourceKind{
			evaluation.ProductResourceDiscoveredCandidate,
			evaluation.ProductResourceCandidateEvidence,
			evaluation.ProductResourceProductFixture,
			evaluation.ProductResourceCampaign,
			evaluation.ProductResourceDashboardProjection,
			evaluation.ProductResourceToolCallInspection,
		}
	}
	now := time.Now().UTC()
	for _, kind := range kinds {
		applicationID := fmt.Sprintf("retention_%d_%s", now.UnixNano(), strings.ReplaceAll(string(kind), "_", ""))
		outcome := "dry_run"
		if !filter.DryRun {
			outcome = "expired"
			if err := s.applyProductRetentionKind(ctx, tenantID, kind, now); err != nil {
				return err
			}
		}
		document := map[string]any{
			"applicationId": applicationID,
			"tenantId":      tenantID,
			"resourceKind":  kind,
			"dryRun":        filter.DryRun,
			"outcome":       outcome,
			"appliedAt":     now,
		}
		encoded, err := json.Marshal(document)
		if err != nil {
			return fmt.Errorf("marshal evaluation retention application: %w", err)
		}
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO evaluation_retention_applications (
				application_id, tenant_id, resource_kind, resource_id, dry_run, outcome, applied_at, document_json
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, applicationID, tenantID, string(kind), nil, boolToInt(filter.DryRun), outcome, formatProductTime(now), string(encoded)); err != nil {
			return fmt.Errorf("record evaluation retention application %s: %w", kind, err)
		}
	}
	return nil
}

func (s *SQLiteStore) UpsertProductFixture(ctx context.Context, item evaluation.ProductManagedFixture) error {
	tenantID, err := s.evaluationProductTenantID(ctx, item.TenantID)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal evaluation product fixture: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO evaluation_product_fixtures (
			fixture_id, tenant_id, display_name, domain_class, source_kind, source_candidate_id,
			current_revision_id, review_state, suppression_state, retention_state, created_by,
			created_at, updated_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(fixture_id) DO UPDATE SET
			tenant_id = COALESCE(evaluation_product_fixtures.tenant_id, excluded.tenant_id),
			display_name = excluded.display_name,
			domain_class = excluded.domain_class,
			source_kind = excluded.source_kind,
			source_candidate_id = excluded.source_candidate_id,
			current_revision_id = excluded.current_revision_id,
			review_state = excluded.review_state,
			suppression_state = excluded.suppression_state,
			retention_state = excluded.retention_state,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json
	`, item.FixtureID, tenantID, item.DisplayName, string(item.DomainClass), nullString(item.SourceKind),
		nullString(item.SourceCandidateID), nullString(item.CurrentRevisionID), string(item.ReviewState),
		string(item.SuppressionState), string(item.RetentionState), nullString(item.CreatedBy),
		formatProductTime(item.CreatedAt), formatProductTime(item.UpdatedAt), string(document))
	if err != nil {
		return fmt.Errorf("upsert evaluation product fixture %s: %w", item.FixtureID, err)
	}
	return nil
}

func (s *SQLiteStore) ListProductFixtures(ctx context.Context, filter evaluation.ProductListFilter) ([]evaluation.ProductManagedFixture, error) {
	tenantID, err := s.evaluationProductFilterTenantID(ctx, filter.TenantID)
	if err != nil {
		return nil, err
	}
	query := `SELECT document_json FROM evaluation_product_fixtures WHERE tenant_id = ?`
	args := []any{tenantID}
	if filter.Cursor != "" {
		query += ` AND fixture_id < ?`
		args = append(args, filter.Cursor)
	}
	query += ` ORDER BY updated_at DESC, fixture_id DESC LIMIT ?`
	args = append(args, evaluation.NormalizeProductLimit(filter.Limit))
	return scanEvaluationProductDocuments[evaluation.ProductManagedFixture](ctx, s.db, query, args, "product fixtures")
}

func (s *SQLiteStore) GetProductFixture(ctx context.Context, tenantID, fixtureID string) (evaluation.ProductManagedFixture, bool, error) {
	return getEvaluationProductDocument[evaluation.ProductManagedFixture](ctx, s.db, "evaluation_product_fixtures", "fixture_id", tenantID, fixtureID, "product fixture")
}

func (s *SQLiteStore) SaveFixtureRevision(ctx context.Context, item evaluation.FixtureRevision) error {
	tenantID, err := s.evaluationProductTenantID(ctx, item.TenantID)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal evaluation fixture revision: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO evaluation_fixture_revisions (
			revision_id, fixture_id, tenant_id, revision_number, redaction_status,
			created_by, created_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(revision_id) DO UPDATE SET document_json = evaluation_fixture_revisions.document_json
	`, item.RevisionID, item.FixtureID, tenantID, item.RevisionNumber, string(item.RedactionStatus),
		nullString(item.CreatedBy), formatProductTime(item.CreatedAt), string(document))
	if err != nil {
		return fmt.Errorf("save evaluation fixture revision %s: %w", item.RevisionID, err)
	}
	return nil
}

func (s *SQLiteStore) ListFixtureRevisions(ctx context.Context, tenantID, fixtureID string, limit int) ([]evaluation.FixtureRevision, error) {
	tenantID, err := s.evaluationProductFilterTenantID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	query := `
		SELECT document_json FROM evaluation_fixture_revisions
		WHERE tenant_id = ? AND fixture_id = ?
		ORDER BY revision_number DESC, revision_id DESC
		LIMIT ?
	`
	args := []any{tenantID, fixtureID, evaluation.NormalizeProductLimit(limit)}
	return scanEvaluationProductDocuments[evaluation.FixtureRevision](ctx, s.db, query, args, "fixture revisions")
}

func (s *SQLiteStore) SaveReplayCampaign(ctx context.Context, item evaluation.ReplayCampaign) error {
	tenantID, err := s.evaluationProductTenantID(ctx, item.TenantID)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal evaluation replay campaign: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO evaluation_campaigns (
			campaign_id, tenant_id, display_name, status, created_at, started_at,
			completed_at, published_at, retention_state, idempotency_key, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(campaign_id) DO UPDATE SET
			tenant_id = COALESCE(evaluation_campaigns.tenant_id, excluded.tenant_id),
			display_name = excluded.display_name,
			status = excluded.status,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			published_at = excluded.published_at,
			retention_state = excluded.retention_state,
			document_json = excluded.document_json
	`, item.CampaignID, tenantID, item.DisplayName, string(item.Status), formatProductTime(item.CreatedAt),
		nullableTimeString(item.StartedAt), nullableTimeString(item.CompletedAt), nullableTimeString(item.PublishedAt),
		string(item.RetentionState), nullString(item.IdempotencyKey), string(document))
	if err != nil {
		return fmt.Errorf("save evaluation replay campaign %s: %w", item.CampaignID, err)
	}
	return nil
}

func (s *SQLiteStore) ListReplayCampaigns(ctx context.Context, filter evaluation.ProductListFilter) ([]evaluation.ReplayCampaign, error) {
	tenantID, err := s.evaluationProductFilterTenantID(ctx, filter.TenantID)
	if err != nil {
		return nil, err
	}
	query := `SELECT document_json FROM evaluation_campaigns WHERE tenant_id = ?`
	args := []any{tenantID}
	if filter.Cursor != "" {
		query += ` AND campaign_id < ?`
		args = append(args, filter.Cursor)
	}
	query += ` ORDER BY created_at DESC, campaign_id DESC LIMIT ?`
	args = append(args, evaluation.NormalizeProductLimit(filter.Limit))
	return scanEvaluationProductDocuments[evaluation.ReplayCampaign](ctx, s.db, query, args, "replay campaigns")
}

func (s *SQLiteStore) GetReplayCampaign(ctx context.Context, tenantID, campaignID string) (evaluation.ReplayCampaign, bool, error) {
	return getEvaluationProductDocument[evaluation.ReplayCampaign](ctx, s.db, "evaluation_campaigns", "campaign_id", tenantID, campaignID, "replay campaign")
}

func (s *SQLiteStore) SaveCampaignItem(ctx context.Context, item evaluation.CampaignItem) error {
	tenantID, err := s.evaluationProductTenantID(ctx, item.TenantID)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal evaluation campaign item: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO evaluation_campaign_items (
			campaign_item_id, campaign_id, tenant_id, source_type, source_id,
			suppression_checked_at, created_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(campaign_item_id) DO UPDATE SET document_json = excluded.document_json
	`, item.CampaignItemID, item.CampaignID, tenantID, string(item.SourceType), item.SourceID,
		formatProductTime(item.SuppressionCheckedAt), formatProductTime(item.CreatedAt), string(document))
	if err != nil {
		return fmt.Errorf("save evaluation campaign item %s: %w", item.CampaignItemID, err)
	}
	return nil
}

func (s *SQLiteStore) ListCampaignItems(ctx context.Context, filter evaluation.ProductListFilter, campaignID string) ([]evaluation.CampaignItem, error) {
	tenantID, err := s.evaluationProductFilterTenantID(ctx, filter.TenantID)
	if err != nil {
		return nil, err
	}
	query := `SELECT document_json FROM evaluation_campaign_items WHERE tenant_id = ?`
	args := []any{tenantID}
	if campaignID != "" {
		query += ` AND campaign_id = ?`
		args = append(args, campaignID)
	}
	if filter.Cursor != "" {
		query += ` AND campaign_item_id < ?`
		args = append(args, filter.Cursor)
	}
	query += ` ORDER BY created_at DESC, campaign_item_id DESC LIMIT ?`
	args = append(args, evaluation.NormalizeProductLimit(filter.Limit))
	return scanEvaluationProductDocuments[evaluation.CampaignItem](ctx, s.db, query, args, "campaign items")
}

func (s *SQLiteStore) SaveCampaignAttemptGroup(ctx context.Context, item evaluation.CampaignAttemptGroup) error {
	tenantID, err := s.evaluationProductTenantID(ctx, item.TenantID)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal evaluation campaign attempt group: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO evaluation_campaign_attempt_groups (
			attempt_group_id, campaign_id, campaign_item_id, tenant_id, status, drift_count,
			failure_count, unsupported_count, operator_action_needed_count, created_at,
			updated_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(attempt_group_id) DO UPDATE SET
			status = excluded.status,
			drift_count = excluded.drift_count,
			failure_count = excluded.failure_count,
			unsupported_count = excluded.unsupported_count,
			operator_action_needed_count = excluded.operator_action_needed_count,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json
	`, item.AttemptGroupID, item.CampaignID, item.CampaignItemID, tenantID, string(item.Status), item.DriftCount,
		item.FailureCount, item.UnsupportedCount, item.OperatorActionNeededCount, formatProductTime(item.CreatedAt),
		formatProductTime(item.UpdatedAt), string(document))
	if err != nil {
		return fmt.Errorf("save evaluation campaign attempt group %s: %w", item.AttemptGroupID, err)
	}
	return nil
}

func (s *SQLiteStore) ListCampaignAttemptGroups(ctx context.Context, filter evaluation.ProductListFilter, campaignID string) ([]evaluation.CampaignAttemptGroup, error) {
	tenantID, err := s.evaluationProductFilterTenantID(ctx, filter.TenantID)
	if err != nil {
		return nil, err
	}
	query := `SELECT document_json FROM evaluation_campaign_attempt_groups WHERE tenant_id = ?`
	args := []any{tenantID}
	if campaignID != "" {
		query += ` AND campaign_id = ?`
		args = append(args, campaignID)
	}
	if filter.Cursor != "" {
		query += ` AND attempt_group_id < ?`
		args = append(args, filter.Cursor)
	}
	query += ` ORDER BY updated_at DESC, attempt_group_id DESC LIMIT ?`
	args = append(args, evaluation.NormalizeProductLimit(filter.Limit))
	return scanEvaluationProductDocuments[evaluation.CampaignAttemptGroup](ctx, s.db, query, args, "campaign attempt groups")
}

func (s *SQLiteStore) SaveDashboardProjection(ctx context.Context, item evaluation.DashboardProjection) error {
	tenantID, err := s.evaluationProductTenantID(ctx, item.TenantID)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	if item.RetentionState == "" {
		item.RetentionState = evaluation.RetentionStateActive
	}
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal evaluation dashboard projection: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO evaluation_dashboard_projections (
			projection_id, tenant_id, window_start, window_end, generated_at, cursor, retention_state, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(projection_id) DO UPDATE SET
			window_start = excluded.window_start,
			window_end = excluded.window_end,
			generated_at = excluded.generated_at,
			cursor = excluded.cursor,
			retention_state = excluded.retention_state,
			document_json = excluded.document_json
	`, item.ProjectionID, tenantID, formatProductTime(item.WindowStart), formatProductTime(item.WindowEnd),
		formatProductTime(item.GeneratedAt), nullString(item.Cursor), string(item.RetentionState), string(document))
	if err != nil {
		return fmt.Errorf("save evaluation dashboard projection %s: %w", item.ProjectionID, err)
	}
	return nil
}

func (s *SQLiteStore) ListDashboardProjections(ctx context.Context, filter evaluation.ProductListFilter) ([]evaluation.DashboardProjection, error) {
	tenantID, err := s.evaluationProductFilterTenantID(ctx, filter.TenantID)
	if err != nil {
		return nil, err
	}
	query := `SELECT document_json FROM evaluation_dashboard_projections WHERE tenant_id = ? AND retention_state = ?`
	args := []any{tenantID, string(evaluation.RetentionStateActive)}
	if filter.Cursor != "" {
		query += ` AND projection_id < ?`
		args = append(args, filter.Cursor)
	}
	query += ` ORDER BY generated_at DESC, projection_id DESC LIMIT ?`
	args = append(args, evaluation.NormalizeProductLimit(filter.Limit))
	return scanEvaluationProductDocuments[evaluation.DashboardProjection](ctx, s.db, query, args, "dashboard projections")
}

func (s *SQLiteStore) GetDashboardProjection(ctx context.Context, tenantID, projectionID string) (evaluation.DashboardProjection, bool, error) {
	return getEvaluationProductDocument[evaluation.DashboardProjection](ctx, s.db, "evaluation_dashboard_projections", "projection_id", tenantID, projectionID, "dashboard projection")
}

func (s *SQLiteStore) SaveToolCallInspection(ctx context.Context, item evaluation.ToolCallInspection) error {
	tenantID, err := s.evaluationProductTenantID(ctx, item.TenantID)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	if item.RetentionState == "" {
		item.RetentionState = evaluation.RetentionStateActive
	}
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal evaluation tool-call inspection: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO evaluation_tool_call_inspections (
			inspection_id, tenant_id, campaign_id, campaign_item_id, tool_call_ref,
			classification, redaction_status, retention_state, created_at, updated_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(inspection_id) DO UPDATE SET
			classification = excluded.classification,
			redaction_status = excluded.redaction_status,
			retention_state = excluded.retention_state,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json
	`, item.InspectionID, tenantID, item.CampaignID, item.CampaignItemID, item.ToolCallRef, item.Classification,
		string(item.RedactionStatus), string(item.RetentionState), formatProductTime(item.CreatedAt), formatProductTime(item.UpdatedAt), string(document))
	if err != nil {
		return fmt.Errorf("save evaluation tool-call inspection %s: %w", item.InspectionID, err)
	}
	return nil
}

func (s *SQLiteStore) ListToolCallInspections(ctx context.Context, filter evaluation.ProductListFilter, campaignID string) ([]evaluation.ToolCallInspection, error) {
	tenantID, err := s.evaluationProductFilterTenantID(ctx, filter.TenantID)
	if err != nil {
		return nil, err
	}
	query := `SELECT document_json FROM evaluation_tool_call_inspections WHERE tenant_id = ? AND retention_state = ?`
	args := []any{tenantID, string(evaluation.RetentionStateActive)}
	if campaignID != "" {
		query += ` AND campaign_id = ?`
		args = append(args, campaignID)
	}
	if filter.Cursor != "" {
		query += ` AND inspection_id < ?`
		args = append(args, filter.Cursor)
	}
	query += ` ORDER BY updated_at DESC, inspection_id DESC LIMIT ?`
	args = append(args, evaluation.NormalizeProductLimit(filter.Limit))
	return scanEvaluationProductDocuments[evaluation.ToolCallInspection](ctx, s.db, query, args, "tool-call inspections")
}

func (s *SQLiteStore) GetToolCallInspection(ctx context.Context, tenantID, inspectionID string) (evaluation.ToolCallInspection, bool, error) {
	return getEvaluationProductDocument[evaluation.ToolCallInspection](ctx, s.db, "evaluation_tool_call_inspections", "inspection_id", tenantID, inspectionID, "tool-call inspection")
}

func (s *SQLiteStore) applyProductRetentionKind(ctx context.Context, tenantID string, kind evaluation.ProductResourceKind, now time.Time) error {
	switch kind {
	case evaluation.ProductResourceDiscoveredCandidate:
		return updateEvaluationProductRetention[evaluation.DiscoveredCandidate](ctx, s.db, "evaluation_discovered_candidates", "discovered_candidate_id", tenantID, now, productRetentionColumns{expiresAt: true, updatedAt: true}, func(item *evaluation.DiscoveredCandidate) {
			item.RetentionState = evaluation.RetentionStateExpired
			item.UpdatedAt = now
			item.ExpiresAt = &now
		})
	case evaluation.ProductResourceCandidateEvidence:
		return updateEvaluationProductRetention[evaluation.CandidateEvidence](ctx, s.db, "evaluation_candidate_evidence", "evidence_id", tenantID, now, productRetentionColumns{expiresAt: true}, func(item *evaluation.CandidateEvidence) {
			item.RetentionState = evaluation.RetentionStateExpired
			item.ExpiresAt = &now
		})
	case evaluation.ProductResourceProductFixture:
		return updateEvaluationProductRetention[evaluation.ProductManagedFixture](ctx, s.db, "evaluation_product_fixtures", "fixture_id", tenantID, now, productRetentionColumns{updatedAt: true}, func(item *evaluation.ProductManagedFixture) {
			item.RetentionState = evaluation.RetentionStateExpired
			item.UpdatedAt = now
		})
	case evaluation.ProductResourceCampaign:
		return updateEvaluationProductRetention[evaluation.ReplayCampaign](ctx, s.db, "evaluation_campaigns", "campaign_id", tenantID, now, productRetentionColumns{}, func(item *evaluation.ReplayCampaign) {
			item.RetentionState = evaluation.RetentionStateExpired
		})
	case evaluation.ProductResourceDashboardProjection:
		return updateEvaluationProductRetention[evaluation.DashboardProjection](ctx, s.db, "evaluation_dashboard_projections", "projection_id", tenantID, now, productRetentionColumns{}, func(item *evaluation.DashboardProjection) {
			item.RetentionState = evaluation.RetentionStateExpired
		})
	case evaluation.ProductResourceToolCallInspection:
		return updateEvaluationProductRetention[evaluation.ToolCallInspection](ctx, s.db, "evaluation_tool_call_inspections", "inspection_id", tenantID, now, productRetentionColumns{updatedAt: true}, func(item *evaluation.ToolCallInspection) {
			item.RetentionState = evaluation.RetentionStateExpired
			item.UpdatedAt = now
		})
	default:
		return nil
	}
}

func (s *SQLiteStore) evaluationProductTenantID(ctx context.Context, explicit string) (string, error) {
	tenantID := coalesceString(explicit, tenantBindingString(s.ResolveActiveTenantBinding(ctx)), tenantBindingString(s.ResolveDefaultTenantBinding(ctx)))
	if err := evaluation.ValidateTenantScopedProductRequest(tenantID); err != nil {
		return "", err
	}
	return tenantID, nil
}

func (s *SQLiteStore) evaluationProductFilterTenantID(ctx context.Context, explicit string) (string, error) {
	tenantID := coalesceString(explicit, tenantBindingString(s.ResolveActiveTenantBinding(ctx)))
	if err := evaluation.ValidateTenantScopedProductRequest(tenantID); err != nil {
		return "", err
	}
	return tenantID, nil
}

func scanEvaluationProductDocuments[T any](ctx context.Context, db *sql.DB, query string, args []any, label string) ([]T, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list evaluation product %s: %w", label, err)
	}
	defer rows.Close()
	items := []T{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item T
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode evaluation product %s: %w", label, err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func getEvaluationProductDocument[T any](ctx context.Context, db *sql.DB, table, idColumn, tenantID, id, label string) (T, bool, error) {
	var zero T
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(id) == "" {
		return zero, false, nil
	}
	var raw string
	if err := db.QueryRowContext(ctx, fmt.Sprintf(`SELECT document_json FROM %s WHERE tenant_id = ? AND %s = ?`, table, idColumn), tenantID, id).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return zero, false, nil
		}
		return zero, false, fmt.Errorf("get evaluation product %s %s: %w", label, id, err)
	}
	var item T
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		return zero, false, fmt.Errorf("decode evaluation product %s %s: %w", label, id, err)
	}
	return item, true, nil
}

type productRetentionColumns struct {
	expiresAt bool
	updatedAt bool
}

func updateEvaluationProductRetention[T any](ctx context.Context, db *sql.DB, table, idColumn, tenantID string, now time.Time, columns productRetentionColumns, mutate func(*T)) error {
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`SELECT %s, document_json FROM %s WHERE tenant_id = ?`, idColumn, table), tenantID)
	if err != nil {
		return fmt.Errorf("load evaluation product retention rows for %s: %w", table, err)
	}
	defer rows.Close()
	type row struct {
		id       string
		document string
	}
	loaded := []row{}
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.id, &item.document); err != nil {
			return err
		}
		loaded = append(loaded, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, loadedRow := range loaded {
		var item T
		if err := json.Unmarshal([]byte(loadedRow.document), &item); err != nil {
			return fmt.Errorf("decode retention row %s/%s: %w", table, loadedRow.id, err)
		}
		mutate(&item)
		encoded, err := json.Marshal(item)
		if err != nil {
			return fmt.Errorf("marshal retention row %s/%s: %w", table, loadedRow.id, err)
		}
		assignments := []string{`retention_state = ?`}
		args := []any{string(evaluation.RetentionStateExpired)}
		if columns.expiresAt {
			assignments = append(assignments, `expires_at = COALESCE(expires_at, ?)`)
			args = append(args, formatProductTime(now))
		}
		if columns.updatedAt {
			assignments = append(assignments, `updated_at = ?`)
			args = append(args, formatProductTime(now))
		}
		assignments = append(assignments, `document_json = ?`)
		args = append(args, string(encoded), tenantID, loadedRow.id)
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`UPDATE %s SET %s WHERE tenant_id = ? AND %s = ?`, table, strings.Join(assignments, ", "), idColumn), args...); err != nil {
			return fmt.Errorf("update retention row %s/%s: %w", table, loadedRow.id, err)
		}
	}
	return nil
}

func productSourceKindsContain(values []evaluation.SourceKind, target evaluation.SourceKind) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func candidateEvidenceRedactionStatus(item evaluation.CandidateEvidence) evaluation.RedactionStatus {
	if len(item.RedactionRulesApplied) > 0 || len(item.SensitiveFieldsExcluded) > 0 {
		return evaluation.RedactionStatusRedacted
	}
	return evaluation.RedactionStatusClean
}

func formatProductTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}
