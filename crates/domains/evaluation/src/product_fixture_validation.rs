//! Port of `daemon/internal/evaluation/product_fixture_validation.go`:
//! materializing only the redacted payload into a product fixture.

use crate::error::EvaluationError;
use crate::product_redaction::{CandidateEvidenceInput, candidate_evidence_from_payload};
use crate::types::{CandidateEvidence, RedactionStatus};

/// Go `ProductFixturePayloadValidation`.
#[derive(Debug, Clone, Default, PartialEq)]
pub struct ProductFixturePayloadValidation {
    pub payload: serde_json::Map<String, serde_json::Value>,
    pub status: RedactionStatus,
    pub evidence: CandidateEvidence,
}

/// Go `ValidateProductFixturePayload`.
pub fn validate_product_fixture_payload(
    input: CandidateEvidenceInput,
) -> Result<ProductFixturePayloadValidation, EvaluationError> {
    let evidence = candidate_evidence_from_payload(input)?;
    let mut status = RedactionStatus::Clean;
    if !evidence.sensitive_fields_excluded.is_empty() {
        status = RedactionStatus::Redacted;
    }
    if !evidence.materialization_allowed {
        status = RedactionStatus::Failed;
    }
    Ok(ProductFixturePayloadValidation {
        payload: evidence.redacted_payload.clone(),
        status,
        evidence,
    })
}

/// Go `RejectRepoManagedFixtureEdit`.
pub fn reject_repo_managed_fixture_edit(source_kind: &str) -> Result<(), EvaluationError> {
    if source_kind.trim() == "repo_fixture"
        || source_kind.trim() == crate::types::SourceKind::Fixture.as_str()
    {
        return Err(EvaluationError::RepoFixtureImmutable);
    }
    Ok(())
}
