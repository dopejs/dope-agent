//! Outcome comparison (port of `comparison.go`).

use std::collections::BTreeMap;

use crate::error::LiveValidationError;
use crate::ledger::LedgerOutcome;
use crate::manager::Manager;
use crate::matrix::SafetyClass;
use crate::matrix::ToolClass;
use crate::store::LedgerFilter;
use crate::types::Comparison;
use crate::types::ComparisonStatus;
use crate::types::LedgerSummary;

impl Manager {
    /// Port of `Manager.CreateComparison`.
    pub async fn create_comparison(
        &self,
        validation_id: &str,
    ) -> Result<Comparison, LiveValidationError> {
        let Some(attempt) = self.get_attempt(validation_id).await? else {
            return Err(LiveValidationError::Disabled);
        };
        let ledger = self
            .list_ledger_entries(LedgerFilter {
                tenant_id: attempt.tenant_id.clone(),
                validation_id: validation_id.to_string(),
                ..LedgerFilter::default()
            })
            .await?;

        let mut summary: LedgerSummary = BTreeMap::new();
        let mut unsupported: Vec<ToolClass> = Vec::new();
        let mut ambiguous: Vec<String> = Vec::new();
        for entry in ledger {
            *summary.entry(entry.outcome.clone()).or_insert(0) += 1;
            let denied_or_skipped =
                entry.outcome == LedgerOutcome::DENIED || entry.outcome == LedgerOutcome::SKIPPED;
            if denied_or_skipped && entry.safety_class == SafetyClass::UNSUPPORTED {
                unsupported.push(entry.tool_class);
            }
            if entry.ambiguous_commit {
                ambiguous.push(entry.ledger_entry_id);
            }
        }

        let operator_action_needed = summary
            .get(&LedgerOutcome::from(LedgerOutcome::OPERATOR_ACTION_NEEDED))
            .copied()
            .unwrap_or(0)
            > 0;
        let denied = summary
            .get(&LedgerOutcome::from(LedgerOutcome::DENIED))
            .copied()
            .unwrap_or(0)
            > 0;
        let failed = summary
            .get(&LedgerOutcome::from(LedgerOutcome::FAILED))
            .copied()
            .unwrap_or(0)
            > 0;

        let mut status = ComparisonStatus::from(ComparisonStatus::MATCHED);
        if operator_action_needed || !ambiguous.is_empty() {
            status = ComparisonStatus::from(ComparisonStatus::OPERATOR_ACTION_NEEDED);
        } else if denied {
            status = ComparisonStatus::from(ComparisonStatus::BLOCKED);
        } else if !unsupported.is_empty() {
            status = ComparisonStatus::from(ComparisonStatus::UNSUPPORTED);
        } else if failed {
            status = ComparisonStatus::from(ComparisonStatus::DRIFTED);
        }

        let comparison = Comparison {
            comparison_id: crate::manager::new_id("lv_comparison"),
            validation_id: validation_id.to_string(),
            candidate_id: attempt.candidate_id,
            baseline_ref: attempt.source_attempt_id,
            terminal_status: status,
            ledger_summary: summary,
            unsupported_classes: unsupported,
            ambiguous_commits: ambiguous,
            generated_at: self.now(),
            ..Comparison::default()
        };
        if let Some(store) = self.store() {
            store.save_comparison(comparison.clone()).await?;
        }
        Ok(comparison)
    }
}

#[cfg(test)]
mod tests {
    use std::sync::Arc;

    use crate::ledger::LedgerOutcome;
    use crate::matrix::SafetyClass;
    use crate::matrix::ToolClass;
    use crate::testutil::MemStore;
    use crate::testutil::manager_with_store;
    use crate::types::Attempt;
    use crate::types::ComparisonStatus;
    use crate::types::SideEffectLedgerEntry;

    #[tokio::test]
    async fn create_comparison_summarizes_outcomes() {
        let store = Arc::new(MemStore::default());
        {
            let mut state = store.state.lock();
            state.attempts.push(Attempt {
                validation_id: "lv_1".to_string(),
                tenant_id: "ten_1".to_string(),
                candidate_id: "candidate_1".to_string(),
                ..Attempt::default()
            });
            state.ledger.push(SideEffectLedgerEntry {
                ledger_entry_id: "ledger_1".to_string(),
                validation_id: "lv_1".to_string(),
                tenant_id: "ten_1".to_string(),
                candidate_id: "candidate_1".to_string(),
                tool_class: ToolClass::from(ToolClass::MCP_TOOL_CALL),
                safety_class: SafetyClass::from(SafetyClass::UNSUPPORTED),
                outcome: LedgerOutcome::from(LedgerOutcome::DENIED),
                ..SideEffectLedgerEntry::default()
            });
            state.ledger.push(SideEffectLedgerEntry {
                ledger_entry_id: "ledger_2".to_string(),
                validation_id: "lv_1".to_string(),
                tenant_id: "ten_1".to_string(),
                candidate_id: "candidate_1".to_string(),
                tool_class: ToolClass::from(ToolClass::MAIL_SEND),
                safety_class: SafetyClass::from(SafetyClass::NON_IDEMPOTENT_MUTATION),
                outcome: LedgerOutcome::from(LedgerOutcome::OPERATOR_ACTION_NEEDED),
                ambiguous_commit: true,
                ..SideEffectLedgerEntry::default()
            });
        }
        let manager = manager_with_store(store.clone());

        let comparison = manager
            .create_comparison("lv_1")
            .await
            .expect("create comparison");

        assert_eq!(
            comparison.terminal_status,
            ComparisonStatus::from(ComparisonStatus::OPERATOR_ACTION_NEEDED)
        );
        assert_eq!(
            comparison
                .ledger_summary
                .get(&LedgerOutcome::from(LedgerOutcome::DENIED)),
            Some(&1)
        );
        assert_eq!(
            comparison
                .ledger_summary
                .get(&LedgerOutcome::from(LedgerOutcome::OPERATOR_ACTION_NEEDED)),
            Some(&1)
        );
    }
}
