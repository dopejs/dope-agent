//! Port of the sentinel errors and ad-hoc `fmt.Errorf` failure modes across
//! the Go evaluation package (`manager.go`, `campaign.go`, `campaign_aggregation.go`,
//! `comparison.go`, `discovery.go`, `discovery_scoring.go`, `discovery_sources.go`,
//! `fixtures.go`, `product_fixture.go`, `product_validation.go`,
//! `suppression.go`, `tool_call_inspection.go`).

use dope_billing::{BillingError, ReserveResult};

/// Reservation failure surfaced by `create_replay_attempt` / the runtime
/// recorder when quota cannot be reserved (Go `BillingReservationError`).
#[derive(Debug, Clone)]
pub struct BillingReservationError {
    pub result: ReserveResult,
    pub error: BillingError,
}

impl std::fmt::Display for BillingReservationError {
    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
        write!(f, "{}", self.error)
    }
}

#[derive(Debug, thiserror::Error, Clone)]
pub enum EvaluationError {
    // -- Manager / ledger ---------------------------------------------------
    #[error("evaluation store is not configured")]
    StoreNotConfigured,
    #[error("replay candidate {0} not found")]
    CandidateNotFound(String),
    #[error("replay attempt {0} not found")]
    AttemptNotFound(String),
    #[error("baseline replay attempt {0} not found")]
    BaselineAttemptNotFound(String),
    #[error("fixture {0} not found for replay candidate {1}")]
    FixtureNotFound(String, String),
    #[error("candidateId is required")]
    CandidateIdRequired,
    #[error("unsupported candidateKind {0}")]
    UnsupportedCandidateKind(String),
    #[error("displayName is required")]
    DisplayNameRequired,
    #[error("sourceKind is required")]
    SourceKindRequired,
    #[error("sourceId is required")]
    SourceIdRequired,
    #[error("sourceRefs is required")]
    SourceRefsRequired,
    #[error("sourceRefs[{0}] requires kind and id")]
    SourceRefInvalid(usize),
    #[error("blocked or unreplayable candidates require readinessReasons or limitations")]
    BlockedCandidateRequiresReasons,
    #[error("defaultReplayMode must be \"non_live\"")]
    DefaultReplayModeInvalid,
    #[error("store error: {0}")]
    Store(String),
    #[error("billing reservation failed: {0}")]
    BillingReservation(BillingReservationError),
    #[error("billing lifecycle failed: {0}")]
    BillingLifecycle(BillingError),
    #[error("record replay runtime run: {0}")]
    RecordReplay(String),

    // -- Fixtures -----------------------------------------------------------
    #[error("read fixtures dir: {0}")]
    ReadFixturesDir(String),
    #[error("read fixture manifest {0}: {1}")]
    ReadFixtureManifest(String, String),
    #[error("decode fixture manifest {0}: {1}")]
    DecodeFixtureManifest(String, String),
    #[error("validate fixture {0}: {1}")]
    ValidateFixture(String, String),
    #[error("fixture {0} has no captured evidence refs")]
    CapturedEvidenceMissing(String),
    #[error("read captured evidence {0}: {1}")]
    ReadCapturedEvidence(String, String),
    #[error("decode captured evidence {0}: {1}")]
    DecodeCapturedEvidence(String, String),

    // -- Product validation --------------------------------------------------
    #[error("evaluation product tenant required")]
    ProductTenantRequired,
    #[error("evaluation product bounds invalid")]
    ProductBoundsInvalid,
    #[error("evaluation product redaction failed")]
    ProductRedactionFailed,
    #[error("evaluation product source required")]
    ProductSourceRequired,
    #[error("evaluation product suppression target required")]
    ProductSuppressionTargetRequired,
    #[error("evaluation product source belongs to another tenant")]
    ProductCrossTenantSource,
    #[error("evaluation product fixture source required")]
    ProductFixtureSourceRequired,
    #[error("evaluation product fixture is not editable")]
    ProductFixtureNotEditable,
    #[error("evaluation product fixture is not selectable")]
    ProductFixtureNotSelectable,
    #[error("repo-managed fixture is immutable from product editing")]
    RepoFixtureImmutable,

    // -- Campaign / discovery / inspection -----------------------------------
    #[error("evaluation campaign transition invalid")]
    CampaignTransitionInvalid,
    #[error("evaluation campaign selection invalid")]
    CampaignSelectionInvalid,
    #[error("evaluation discovery source reader required")]
    DiscoverySourceReaderRequired,
    #[error("evaluation tool-call inspection evidence required")]
    ToolCallInspectionEvidenceRequired,
}
