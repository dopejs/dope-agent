//! Port of daemon/internal/evaluation/comparison_test.go.

mod common;

use std::sync::Arc;

use dope_evaluation::{
    CandidateKind, ComparisonTerminalStatus, CreateComparisonInput, CreateReplayAttemptInput,
    Dependencies, DriftPlane, Manager, PlaneSummaries, ReadinessStatus, ReplayMode, SourceKind,
    SourceRef,
};

use common::{MemoryStore, fixed_now};

#[tokio::test]
async fn create_comparison_reports_drift_and_limitations() {
    let store = Arc::new(MemoryStore::new());
    let manager = Manager::new(Dependencies {
        environment_scope: "test".to_string(),
        store: Some(store.clone()),
        clock: Some(Arc::new(fixed_now)),
        ..Default::default()
    });
    let candidate = dope_evaluation::ReplayCandidate {
        candidate_id: "candidate_drift".to_string(),
        candidate_kind: CandidateKind::CuratedWork,
        display_name: "Runtime drift".to_string(),
        source_kind: SourceKind::Run,
        source_id: "run_base".to_string(),
        environment_scope: "test".to_string(),
        readiness_status: ReadinessStatus::FullyReplayable,
        default_replay_mode: ReplayMode::NonLive,
        source_refs: vec![SourceRef {
            kind: SourceKind::Run,
            id: "run_base".to_string(),
            route: "/v1/runs/run_base".to_string(),
        }],
        expected_comparison: Some(PlaneSummaries {
            runtime: "baseline runtime completed".to_string(),
            policy: "baseline policy allowed".to_string(),
            integration: "baseline integration stable".to_string(),
            delivery: "baseline delivery completed".to_string(),
            evidence: "baseline evidence complete".to_string(),
        }),
        ..Default::default()
    };
    manager
        .upsert_replay_candidate(candidate)
        .expect("UpsertReplayCandidate returned error");
    let attempt = manager
        .create_replay_attempt("candidate_drift", CreateReplayAttemptInput::default())
        .await
        .expect("CreateReplayAttempt returned error");
    let mut attempt = attempt;
    attempt.runtime_summary = "replay runtime changed".to_string();
    store.insert_attempt(attempt.clone());

    let comparison = manager
        .create_comparison(&attempt.attempt_id, CreateComparisonInput::default())
        .expect("CreateComparison returned error");
    assert_eq!(
        comparison.terminal_status,
        ComparisonTerminalStatus::Drifted,
        "expected drifted comparison"
    );
    assert_eq!(comparison.drift_findings.len(), 1);
    assert_eq!(comparison.drift_findings[0].plane, DriftPlane::Runtime);
}
