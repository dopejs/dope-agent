//! Fake downstream outcomes for executor tests (port of `fake_outcome.go`).

use serde::Deserialize;
use serde::Serialize;

use crate::ledger::LedgerOutcome;
use crate::matrix::SafetyClass;
use crate::types::FakeOutcome;

/// The deterministic downstream verdict produced by a fake outcome.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct FakeOutcomeResult {
    pub outcome: LedgerOutcome,
    pub ambiguous_commit: bool,
    pub automatic_retry_allowed: bool,
    pub correlation_key_required: bool,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub reason_code: String,
}

/// Maps a fake downstream outcome to its ledger/retry semantics.
#[must_use]
pub fn fake_outcome_result_for(
    outcome: &FakeOutcome,
    safety_class: &SafetyClass,
) -> FakeOutcomeResult {
    let non_idempotent = safety_class.as_str() == SafetyClass::NON_IDEMPOTENT_MUTATION;
    match outcome.as_str() {
        FakeOutcome::COMPLETED => FakeOutcomeResult {
            outcome: LedgerOutcome::from(LedgerOutcome::COMPLETED),
            correlation_key_required: true,
            ..FakeOutcomeResult::default()
        },
        FakeOutcome::FAILED => FakeOutcomeResult {
            outcome: LedgerOutcome::from(LedgerOutcome::FAILED),
            automatic_retry_allowed: !non_idempotent,
            correlation_key_required: true,
            reason_code: "live_validation.fake_failed".to_string(),
            ..FakeOutcomeResult::default()
        },
        FakeOutcome::DUPLICATE_RETRY => FakeOutcomeResult {
            outcome: LedgerOutcome::from(LedgerOutcome::COMPLETED),
            automatic_retry_allowed: !non_idempotent,
            correlation_key_required: true,
            reason_code: "live_validation.fake_duplicate_retry".to_string(),
            ..FakeOutcomeResult::default()
        },
        FakeOutcome::TIMEOUT_AFTER_SUBMIT | FakeOutcome::SUBMIT_UNKNOWN => FakeOutcomeResult {
            outcome: LedgerOutcome::from(LedgerOutcome::OPERATOR_ACTION_NEEDED),
            ambiguous_commit: true,
            automatic_retry_allowed: false,
            correlation_key_required: true,
            reason_code: "live_validation.fake_ambiguous_commit".to_string(),
        },
        _ => FakeOutcomeResult {
            outcome: LedgerOutcome::from(LedgerOutcome::DENIED),
            reason_code: "live_validation.fake_outcome_unknown".to_string(),
            ..FakeOutcomeResult::default()
        },
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::ledger::LedgerOutcome;
    use crate::matrix::SafetyClass;
    use crate::types::FakeOutcome;

    #[test]
    fn completed_requires_correlation_key() {
        let result = fake_outcome_result_for(
            &FakeOutcome::from(FakeOutcome::COMPLETED),
            &SafetyClass::from(SafetyClass::READ_ONLY),
        );
        assert_eq!(
            result.outcome,
            LedgerOutcome::from(LedgerOutcome::COMPLETED)
        );
        assert!(result.correlation_key_required);
        assert!(!result.ambiguous_commit);
    }

    #[test]
    fn failed_disallows_automatic_retry_for_non_idempotent() {
        let idempotent = fake_outcome_result_for(
            &FakeOutcome::from(FakeOutcome::FAILED),
            &SafetyClass::from(SafetyClass::IDEMPOTENT_MUTATION),
        );
        assert!(idempotent.automatic_retry_allowed);

        let non_idempotent = fake_outcome_result_for(
            &FakeOutcome::from(FakeOutcome::FAILED),
            &SafetyClass::from(SafetyClass::NON_IDEMPOTENT_MUTATION),
        );
        assert!(!non_idempotent.automatic_retry_allowed);
    }

    #[test]
    fn timeout_is_ambiguous_and_stops_retry() {
        let result = fake_outcome_result_for(
            &FakeOutcome::from(FakeOutcome::TIMEOUT_AFTER_SUBMIT),
            &SafetyClass::from(SafetyClass::NON_IDEMPOTENT_MUTATION),
        );
        assert_eq!(
            result.outcome,
            LedgerOutcome::from(LedgerOutcome::OPERATOR_ACTION_NEEDED)
        );
        assert!(result.ambiguous_commit);
        assert!(!result.automatic_retry_allowed);
    }
}
