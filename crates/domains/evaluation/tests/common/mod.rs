//! Shared helpers for the dope-evaluation behavioral tests: a memory store
//! (port of the Go `memoryStore`), a SQLite-backed store adapter over the
//! dope-store evaluation DAOs, a stub billing repository that fails closed,
//! a counting runtime recorder, and fixture builders.

#![allow(dead_code)]

use std::collections::HashMap;
use std::sync::atomic::{AtomicUsize, Ordering};

use chrono::{DateTime, Utc};

use dope_billing::{
    BillingError, Category, DenialPayload, ReserveInput, ReserveResult,
};
use dope_evaluation::{
    AttemptFilter, CandidateFilter, ComparisonFilter, ComparisonResult, EvaluationError,
    FixtureFilter, ReplayAttempt, ReplayCandidate, RegressionFixture, ReplayRecordInput,
    ReplayRecordResult, RuntimeRecorder, Store,
};

pub static COUNTER: AtomicUsize = AtomicUsize::new(0);

pub fn temp_dir(name: &str) -> String {
    let n = COUNTER.fetch_add(1, Ordering::Relaxed);
    let dir = std::env::temp_dir().join(format!("dope_evaluation_{name}_{}_{}", std::process::id(), n));
    let _ = std::fs::remove_dir_all(&dir);
    dir.to_string_lossy().to_string()
}

/// Go `fixedClock` (manager_test.go).
#[must_use]
pub fn fixed_now() -> DateTime<Utc> {
    DateTime::parse_from_rfc3339("2026-04-24T10:00:00Z")
        .map(|dt| dt.with_timezone(&Utc))
        .expect("fixed clock parses")
}

/// Go `testReplayCandidate` (billing_test.go).
#[must_use]
pub fn test_replay_candidate(candidate_id: &str) -> ReplayCandidate {
    let now = fixed_now();
    ReplayCandidate {
        candidate_id: candidate_id.to_string(),
        candidate_kind: dope_evaluation::CandidateKind::CuratedWork,
        display_name: format!("Replay Candidate {candidate_id}"),
        source_kind: dope_evaluation::SourceKind::Run,
        source_id: format!("run_{candidate_id}"),
        source_refs: vec![dope_evaluation::SourceRef {
            kind: dope_evaluation::SourceKind::Run,
            id: format!("run_{candidate_id}"),
            route: format!("/v1/runs/run_{candidate_id}"),
        }],
        environment_scope: "test".to_string(),
        readiness_status: dope_evaluation::ReadinessStatus::FullyReplayable,
        default_replay_mode: dope_evaluation::ReplayMode::NonLive,
        expected_comparison: Some(dope_evaluation::PlaneSummaries {
            runtime: "runtime captured".to_string(),
            policy: "policy captured".to_string(),
            evidence: "evidence captured".to_string(),
            ..Default::default()
        }),
        created_at: now,
        updated_at: now,
        ..ReplayCandidate::default()
    }
}

/// Go `fixtureCandidate` / `fixtureEvidence` (product_fixture_test.go).
#[must_use]
pub fn fixture_candidate(now: DateTime<Utc>) -> dope_evaluation::DiscoveredCandidate {
    dope_evaluation::DiscoveredCandidate {
        discovered_candidate_id: "candidate_1".to_string(),
        tenant_id: "ten_eval".to_string(),
        discovery_run_id: "discovery_run_1".to_string(),
        source_kind: dope_evaluation::SourceKind::Run,
        source_id: "run_1".to_string(),
        source_refs: vec![dope_evaluation::SourceRef {
            kind: dope_evaluation::SourceKind::Run,
            id: "run_1".to_string(),
            route: "/v1/runs/run_1".to_string(),
        }],
        score: 0.9,
        score_band: dope_evaluation::ScoreBand::High,
        redaction_status: dope_evaluation::RedactionStatus::Redacted,
        readiness_status: dope_evaluation::ReadinessStatus::FullyReplayable,
        suppression_state: dope_evaluation::SuppressionState::None,
        retention_state: dope_evaluation::RetentionState::Active,
        created_at: now,
        updated_at: now,
        ..Default::default()
    }
}

#[must_use]
pub fn fixture_evidence(now: DateTime<Utc>) -> dope_evaluation::CandidateEvidence {
    dope_evaluation::CandidateEvidence {
        evidence_id: "evidence_1".to_string(),
        tenant_id: "ten_eval".to_string(),
        discovered_candidate_id: "candidate_1".to_string(),
        redacted_payload: serde_json::json!({ "goal": "safe" })
            .as_object()
            .cloned()
            .unwrap_or_default(),
        materialization_allowed: true,
        retention_state: dope_evaluation::RetentionState::Active,
        created_at: now,
        ..Default::default()
    }
}

/// Port of the Go `memoryStore` used by the manager tests.
#[derive(Debug, Default)]
pub struct MemoryStore {
    inner: parking_lot::Mutex<MemoryStoreInner>,
}

#[derive(Debug, Default)]
struct MemoryStoreInner {
    candidates: HashMap<String, ReplayCandidate>,
    attempts: HashMap<String, ReplayAttempt>,
    comparisons: HashMap<String, ComparisonResult>,
    fixtures: HashMap<String, RegressionFixture>,
}

impl MemoryStore {
    #[must_use]
    pub fn new() -> Self {
        MemoryStore::default()
    }

    pub fn insert_candidate(&self, item: ReplayCandidate) {
        self.inner.lock().candidates.insert(item.candidate_id.clone(), item);
    }

    pub fn insert_attempt(&self, item: ReplayAttempt) {
        self.inner.lock().attempts.insert(item.attempt_id.clone(), item);
    }

    pub fn insert_comparison(&self, item: ComparisonResult) {
        self.inner.lock().comparisons.insert(item.comparison_id.clone(), item);
    }

    pub fn insert_fixture(&self, item: RegressionFixture) {
        self.inner.lock().fixtures.insert(item.fixture_id.clone(), item);
    }

    #[must_use]
    pub fn candidate(&self, candidate_id: &str) -> Option<ReplayCandidate> {
        self.inner.lock().candidates.get(candidate_id).cloned()
    }

    #[must_use]
    pub fn attempt(&self, attempt_id: &str) -> Option<ReplayAttempt> {
        self.inner.lock().attempts.get(attempt_id).cloned()
    }
}

impl Store for MemoryStore {
    fn upsert_replay_candidate(&self, item: ReplayCandidate) -> Result<(), EvaluationError> {
        self.insert_candidate(item);
        Ok(())
    }

    fn list_replay_candidates(
        &self,
        filter: &CandidateFilter,
    ) -> Result<Vec<ReplayCandidate>, EvaluationError> {
        let inner = self.inner.lock();
        let mut items: Vec<ReplayCandidate> = inner
            .candidates
            .values()
            .filter(|item| {
                (filter.environment_scope.is_empty() || item.environment_scope == filter.environment_scope)
                    && (filter.candidate_kind == dope_evaluation::CandidateKind::default()
                        || item.candidate_kind == filter.candidate_kind)
                    && (filter.source_kind == dope_evaluation::SourceKind::default()
                        || item.source_kind == filter.source_kind)
                    && (filter.readiness_status == dope_evaluation::ReadinessStatus::default()
                        || item.readiness_status == filter.readiness_status)
            })
            .cloned()
            .collect();
        items.sort_by(|a, b| {
            a.created_at
                .cmp(&b.created_at)
                .then_with(|| a.candidate_id.cmp(&b.candidate_id))
        });
        if filter.limit > 0 && items.len() > filter.limit as usize {
            items.truncate(filter.limit as usize);
        }
        Ok(items)
    }

    fn get_replay_candidate(
        &self,
        _environment_scope: &str,
        candidate_id: &str,
    ) -> Result<Option<ReplayCandidate>, EvaluationError> {
        Ok(self.inner.lock().candidates.get(candidate_id).cloned())
    }

    fn upsert_replay_attempt(&self, item: ReplayAttempt) -> Result<(), EvaluationError> {
        self.insert_attempt(item);
        Ok(())
    }

    fn list_replay_attempts(
        &self,
        filter: &AttemptFilter,
    ) -> Result<Vec<ReplayAttempt>, EvaluationError> {
        let inner = self.inner.lock();
        let mut items: Vec<ReplayAttempt> = inner
            .attempts
            .values()
            .filter(|item| {
                (filter.environment_scope.is_empty() || item.environment_scope == filter.environment_scope)
                    && (filter.candidate_id.is_empty() || item.candidate_id == filter.candidate_id)
                    && (filter.status == dope_evaluation::ReplayAttemptStatus::default()
                        || item.status == filter.status)
            })
            .cloned()
            .collect();
        items.sort_by(|a, b| {
            a.created_at
                .cmp(&b.created_at)
                .then_with(|| a.attempt_id.cmp(&b.attempt_id))
        });
        if filter.limit > 0 && items.len() > filter.limit as usize {
            items.truncate(filter.limit as usize);
        }
        Ok(items)
    }

    fn get_replay_attempt(
        &self,
        _environment_scope: &str,
        attempt_id: &str,
    ) -> Result<Option<ReplayAttempt>, EvaluationError> {
        Ok(self.inner.lock().attempts.get(attempt_id).cloned())
    }

    fn upsert_comparison_result(&self, item: ComparisonResult) -> Result<(), EvaluationError> {
        self.insert_comparison(item);
        Ok(())
    }

    fn list_comparison_results(
        &self,
        filter: &ComparisonFilter,
    ) -> Result<Vec<ComparisonResult>, EvaluationError> {
        let inner = self.inner.lock();
        let mut items: Vec<ComparisonResult> = inner
            .comparisons
            .values()
            .filter(|item| {
                (filter.environment_scope.is_empty() || item.environment_scope == filter.environment_scope)
                    && (filter.candidate_id.is_empty() || item.candidate_id == filter.candidate_id)
                    && (filter.attempt_id.is_empty() || item.attempt_id == filter.attempt_id)
                    && (filter.terminal_status == dope_evaluation::ComparisonTerminalStatus::default()
                        || item.terminal_status == filter.terminal_status)
            })
            .cloned()
            .collect();
        items.sort_by(|a, b| {
            a.generated_at
                .cmp(&b.generated_at)
                .then_with(|| a.comparison_id.cmp(&b.comparison_id))
        });
        if filter.limit > 0 && items.len() > filter.limit as usize {
            items.truncate(filter.limit as usize);
        }
        Ok(items)
    }

    fn get_comparison_result(
        &self,
        _environment_scope: &str,
        comparison_id: &str,
    ) -> Result<Option<ComparisonResult>, EvaluationError> {
        Ok(self.inner.lock().comparisons.get(comparison_id).cloned())
    }

    fn upsert_regression_fixture(&self, item: RegressionFixture) -> Result<(), EvaluationError> {
        self.insert_fixture(item);
        Ok(())
    }

    fn list_regression_fixtures(
        &self,
        filter: &FixtureFilter,
    ) -> Result<Vec<RegressionFixture>, EvaluationError> {
        let inner = self.inner.lock();
        let mut items: Vec<RegressionFixture> = inner
            .fixtures
            .values()
            .filter(|item| {
                (filter.environment_scope.is_empty() || item.environment_scope == filter.environment_scope)
                    && (filter.domain_class == dope_evaluation::FixtureDomainClass::default()
                        || item.domain_class == filter.domain_class)
            })
            .cloned()
            .collect();
        items.sort_by(|a, b| {
            a.domain_class
                .cmp(&b.domain_class)
                .then_with(|| a.fixture_id.cmp(&b.fixture_id))
        });
        if filter.limit > 0 && items.len() > filter.limit as usize {
            items.truncate(filter.limit as usize);
        }
        Ok(items)
    }
}

/// Store adapter over dope-store's SQLite evaluation DAOs (the production
/// wiring for the manager's `Store` trait; dope-store itself cannot hold the
/// impl because store already depends on dope-evaluation).
pub struct SqliteStoreAdapter(pub parking_lot::Mutex<dope_store::SQLiteStore>);

impl SqliteStoreAdapter {
    #[must_use]
    pub fn new(store: dope_store::SQLiteStore) -> Self {
        SqliteStoreAdapter(parking_lot::Mutex::new(store))
    }
}

impl Store for SqliteStoreAdapter {
    fn upsert_replay_candidate(&self, item: ReplayCandidate) -> Result<(), EvaluationError> {
        self.0.lock().upsert_replay_candidate(&item).map_err(EvaluationError::Store)
    }
    fn list_replay_candidates(&self, filter: &CandidateFilter) -> Result<Vec<ReplayCandidate>, EvaluationError> {
        self.0.lock().list_replay_candidates(filter).map_err(EvaluationError::Store)
    }
    fn get_replay_candidate(&self, environment_scope: &str, candidate_id: &str) -> Result<Option<ReplayCandidate>, EvaluationError> {
        self.0.lock().get_replay_candidate(environment_scope, candidate_id).map_err(EvaluationError::Store)
    }
    fn upsert_replay_attempt(&self, item: ReplayAttempt) -> Result<(), EvaluationError> {
        self.0.lock().upsert_replay_attempt(&item).map_err(EvaluationError::Store)
    }
    fn list_replay_attempts(&self, filter: &AttemptFilter) -> Result<Vec<ReplayAttempt>, EvaluationError> {
        self.0.lock().list_replay_attempts(filter).map_err(EvaluationError::Store)
    }
    fn get_replay_attempt(&self, environment_scope: &str, attempt_id: &str) -> Result<Option<ReplayAttempt>, EvaluationError> {
        self.0.lock().get_replay_attempt(environment_scope, attempt_id).map_err(EvaluationError::Store)
    }
    fn upsert_comparison_result(&self, item: ComparisonResult) -> Result<(), EvaluationError> {
        self.0.lock().upsert_comparison_result(&item).map_err(EvaluationError::Store)
    }
    fn list_comparison_results(&self, filter: &ComparisonFilter) -> Result<Vec<ComparisonResult>, EvaluationError> {
        self.0.lock().list_comparison_results(filter).map_err(EvaluationError::Store)
    }
    fn get_comparison_result(&self, environment_scope: &str, comparison_id: &str) -> Result<Option<ComparisonResult>, EvaluationError> {
        self.0.lock().get_comparison_result(environment_scope, comparison_id).map_err(EvaluationError::Store)
    }
    fn upsert_regression_fixture(&self, item: RegressionFixture) -> Result<(), EvaluationError> {
        self.0.lock().upsert_regression_fixture(&item).map_err(EvaluationError::Store)
    }
    fn list_regression_fixtures(&self, filter: &FixtureFilter) -> Result<Vec<RegressionFixture>, EvaluationError> {
        self.0.lock().list_regression_fixtures(filter).map_err(EvaluationError::Store)
    }
}

/// Go `countingReplayRecorder` (billing_test.go).
#[derive(Debug, Default)]
pub struct CountingRecorder {
    pub calls: std::sync::atomic::AtomicUsize,
}

impl RuntimeRecorder for CountingRecorder {
    fn record_replay(
        &self,
        _input: ReplayRecordInput,
    ) -> dope_evaluation::BoxFuture<'_, Result<ReplayRecordResult, EvaluationError>> {
        self.calls.fetch_add(1, Ordering::Relaxed);
        Box::pin(async {
            Ok(ReplayRecordResult {
                run_id: "run_replay_recorded".to_string(),
                workflow_id: "workflow_replay_recorded".to_string(),
                ..Default::default()
            })
        })
    }
}

/// Stub billing repository that fails every reservation closed with a quota
/// denial (used to prove attempt creation denies before any runtime work).
#[derive(Debug, Clone, Default)]
pub struct QuotaDenyRepo;

impl dope_billing::Repository for QuotaDenyRepo {
    fn active_plan(
        &self,
        _tenant_id: &str,
    ) -> dope_billing::BoxFuture<'_, Result<Option<dope_billing::TenantPlan>, BillingError>> {
        Box::pin(async { Ok(None) })
    }
    fn quota_override(
        &self,
        _tenant_id: &str,
        _category: &Category,
        _at: DateTime<Utc>,
    ) -> dope_billing::BoxFuture<'_, Result<Option<dope_billing::QuotaOverride>, BillingError>> {
        Box::pin(async { Ok(None) })
    }
    fn open_period(
        &self,
        _tenant_id: &str,
        _definition: &dope_billing::QuotaDefinition,
        _at: DateTime<Utc>,
    ) -> dope_billing::BoxFuture<'_, Result<dope_billing::QuotaPeriod, BillingError>> {
        Box::pin(async { Ok(dope_billing::QuotaPeriod::default()) })
    }
    fn usage_counter(
        &self,
        _tenant_id: &str,
        _category: &Category,
        _quota_period_id: &str,
    ) -> dope_billing::BoxFuture<'_, Result<Option<dope_billing::UsageCounter>, BillingError>> {
        Box::pin(async { Ok(None) })
    }
    fn save_usage_counter(
        &self,
        _counter: dope_billing::UsageCounter,
    ) -> dope_billing::BoxFuture<'_, Result<(), BillingError>> {
        Box::pin(async { Ok(()) })
    }
    fn reservation_by_operation(
        &self,
        _tenant_id: &str,
        _category: &Category,
        _operation_key: &str,
    ) -> dope_billing::BoxFuture<'_, Result<Option<dope_billing::UsageReservation>, BillingError>> {
        Box::pin(async { Ok(None) })
    }
    fn save_reservation(
        &self,
        _reservation: dope_billing::UsageReservation,
    ) -> dope_billing::BoxFuture<'_, Result<(), BillingError>> {
        Box::pin(async { Ok(()) })
    }
    fn append_usage_event(
        &self,
        _event: dope_billing::UsageEvent,
    ) -> dope_billing::BoxFuture<'_, Result<(), BillingError>> {
        Box::pin(async { Ok(()) })
    }
    fn append_quota_denial(
        &self,
        _denial: dope_billing::QuotaDenial,
    ) -> dope_billing::BoxFuture<'_, Result<(), BillingError>> {
        Box::pin(async { Ok(()) })
    }
    fn list_pending_reservations(
        &self,
    ) -> dope_billing::BoxFuture<'_, Result<Vec<dope_billing::UsageReservation>, BillingError>> {
        Box::pin(async { Ok(Vec::new()) })
    }
    fn reserve_usage(
        &self,
        input: ReserveInput,
        _now: DateTime<Utc>,
    ) -> dope_billing::BoxFuture<'_, Result<Option<ReserveResult>, BillingError>> {
        Box::pin(async move {
            let result = ReserveResult {
                allowed: false,
                denial: Some(DenialPayload {
                    code: "quota_denied".to_string(),
                    reason_code: "quota_denied:replay_evaluation_attempts_exhausted".to_string(),
                    tenant_id: input.tenant_id.clone(),
                    category: input.category.clone(),
                    operation_key: input.operation_key.clone(),
                    message: "Quota exhausted for replay_evaluation_attempts.".to_string(),
                    ..Default::default()
                }),
                failure: Some(BillingError::QuotaDenied),
                ..Default::default()
            };
            Ok(Some(result))
        })
    }
}

/// Tenant context with a fixed tenant id for quota-gated tests.
#[must_use]
pub fn tenant_context(tenant_id: &str) -> dope_identity::TenantContext {
    dope_identity::TenantContext {
        tenant_id: tenant_id.to_string(),
        principal_id: format!("prn_{tenant_id}"),
        ..Default::default()
    }
}
