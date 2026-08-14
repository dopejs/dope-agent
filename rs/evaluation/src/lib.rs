//! Port of daemon/internal/evaluation/types.go: the evaluation data model — replay
//! candidates, replay attempts, comparison results, drift findings, and regression
//! fixtures — plus the query filters and create inputs used by the evaluation API.
//! Types only: manager/recorder/campaign logic lives in the Go daemon and is out of
//! scope here. Wire layout mirrors the Go JSON tags exactly: camelCase fields,
//! snake_case enum values, and `omitempty` mapped to
//! `#[serde(default, skip_serializing_if = ...)]`.

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

macro_rules! string_enum {
    ($name:ident { $first:ident => $first_s:literal $(, $v:ident => $s:literal)* $(,)? }) => {
        #[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default, Serialize, Deserialize)]
        #[serde(rename_all = "snake_case")]
        pub enum $name {
            #[default]
            $first,
            $($v),*
        }
        impl $name {
            #[must_use]
            pub fn as_str(self) -> &'static str {
                match self {
                    $name::$first => $first_s,
                    $( $name::$v => $s ),*
                }
            }
        }
        impl std::fmt::Display for $name {
            fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                f.write_str(self.as_str())
            }
        }
    };
}

string_enum!(CandidateKind {
    CuratedWork => "curated_work",
    Fixture => "fixture",
});

string_enum!(SourceKind {
    Run => "run",
    Workflow => "workflow",
    Schedule => "schedule",
    Integration => "integration",
    ComputerUse => "computer_use",
    Fixture => "fixture",
});

string_enum!(ReadinessStatus {
    FullyReplayable => "fully_replayable",
    PartiallyReplayable => "partially_replayable",
    Blocked => "blocked",
    Unreplayable => "unreplayable",
});

string_enum!(ReplayMode {
    NonLive => "non_live",
    LiveValidation => "live_validation",
});

string_enum!(ReplayAttemptStatus {
    Queued => "queued",
    Running => "running",
    Completed => "completed",
    Blocked => "blocked",
    Unreplayable => "unreplayable",
    Failed => "failed",
    Cancelled => "cancelled",
});

string_enum!(ApprovalHandling {
    Blocked => "blocked",
    EvidenceOnly => "evidence_only",
    FreshApprovalRequired => "fresh_approval_required",
});

string_enum!(SideEffectHandling {
    Blocked => "blocked",
    EvidenceOnly => "evidence_only",
    Live => "live",
});

string_enum!(ComparisonTerminalStatus {
    Matched => "matched",
    Drifted => "drifted",
    Blocked => "blocked",
    Unreplayable => "unreplayable",
});

string_enum!(DriftPlane {
    Runtime => "runtime",
    Policy => "policy",
    Integration => "integration",
    Delivery => "delivery",
    Evidence => "evidence",
    Unknown => "unknown",
    Mixed => "mixed",
});

string_enum!(FixtureDomainClass {
    Schedule => "schedule",
    Integration => "integration",
    ComputerUse => "computer_use",
});

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SourceRef {
    pub kind: SourceKind,
    pub id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub route: String,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SafetyScope {
    pub mode: ReplayMode,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub description: String,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PlaneSummaries {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub runtime: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub policy: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub integration: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub delivery: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub evidence: String,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ReplayCandidate {
    pub candidate_id: String,
    pub candidate_kind: CandidateKind,
    pub display_name: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub description: String,
    pub source_kind: SourceKind,
    pub source_id: String,
    pub source_refs: Vec<SourceRef>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub tool_classes: Vec<String>,
    pub environment_scope: String,
    pub readiness_status: ReadinessStatus,
    pub readiness_reasons: Vec<String>,
    pub limitations: Vec<String>,
    pub default_replay_mode: ReplayMode,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub fixture_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub latest_attempt_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub latest_comparison_id: String,
    // Go json tag is "expectedComparisonSummary" (not the camelCase of the field).
    #[serde(default, skip_serializing_if = "Option::is_none", rename = "expectedComparisonSummary")]
    pub expected_comparison: Option<PlaneSummaries>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub captured_evidence_refs: Vec<SourceRef>,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ReplayAttempt {
    pub attempt_id: String,
    pub candidate_id: String,
    pub source_refs: Vec<SourceRef>,
    pub environment_scope: String,
    pub mode: ReplayMode,
    pub status: ReplayAttemptStatus,
    pub safety_scope: SafetyScope,
    pub approval_handling: ApprovalHandling,
    pub side_effect_handling: SideEffectHandling,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub launched_by: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub change_window_label: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub baseline_attempt_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub result_run_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub result_workflow_id: String,
    pub evidence_refs: Vec<SourceRef>,
    pub blocked_reasons: Vec<String>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub runtime_summary: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub policy_summary: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub integration_summary: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub delivery_summary: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub evidence_summary: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub started_at: Option<DateTime<Utc>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub completed_at: Option<DateTime<Utc>>,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ComparisonResult {
    pub comparison_id: String,
    pub candidate_id: String,
    pub baseline_ref: String,
    pub attempt_id: String,
    pub environment_scope: String,
    pub terminal_status: ComparisonTerminalStatus,
    pub runtime_summary: String,
    pub policy_summary: String,
    pub integration_summary: String,
    pub delivery_summary: String,
    pub evidence_summary: String,
    pub confidence: String,
    pub limitations: Vec<String>,
    pub drift_findings: Vec<DriftFinding>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub change_window_label: String,
    pub generated_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DriftFinding {
    pub finding_id: String,
    pub comparison_id: String,
    pub plane: DriftPlane,
    pub severity: String,
    pub summary: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub baseline_value: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub replay_value: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub evidence_refs: Vec<SourceRef>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub recommended_action: String,
    pub created_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RegressionFixture {
    pub fixture_id: String,
    pub display_name: String,
    pub domain_class: FixtureDomainClass,
    pub manifest_path: String,
    pub source_refs: Vec<SourceRef>,
    pub captured_evidence_refs: Vec<SourceRef>,
    pub assumptions: Vec<String>,
    pub limitations: Vec<String>,
    pub expected_replay_mode: ReplayMode,
    pub expected_comparison_summary: PlaneSummaries,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub candidate_id: String,
    pub environment_scope: String,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

// Plain query structs: no serde, used as API query parameters.

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct CandidateFilter {
    pub environment_scope: String,
    pub candidate_kind: CandidateKind,
    pub source_kind: SourceKind,
    pub readiness_status: ReadinessStatus,
    pub limit: i64,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct AttemptFilter {
    pub environment_scope: String,
    pub candidate_id: String,
    pub status: ReplayAttemptStatus,
    pub limit: i64,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct ComparisonFilter {
    pub environment_scope: String,
    pub candidate_id: String,
    pub attempt_id: String,
    pub terminal_status: ComparisonTerminalStatus,
    pub limit: i64,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct FixtureFilter {
    pub environment_scope: String,
    pub domain_class: FixtureDomainClass,
    pub limit: i64,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CreateReplayAttemptInput {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mode: Option<ReplayMode>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub change_window_label: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub baseline_attempt_id: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub safety_scope: Option<SafetyScope>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub launched_by: String,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CreateComparisonInput {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub baseline_attempt_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub baseline_ref: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub change_window_label: String,
}
