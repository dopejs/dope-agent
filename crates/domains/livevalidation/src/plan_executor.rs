//! Side-effect plan execution (port of `plan_executor.go`).

use std::collections::BTreeMap;

use serde::Deserialize;
use serde::Serialize;

use crate::error::LiveValidationError;
use crate::events::LEDGER_EVENT_SIDE_EFFECT_RECORDED;
use crate::executor::SideEffectExecutionInput;
use crate::ledger::LedgerOutcome;
use crate::manager::Manager;
use crate::matrix::Matrix;
use crate::matrix::SafetyClass;
use crate::matrix::ToolClass;
use crate::readiness::tool_class_set;
use crate::types::AmbiguousCommitCause;
use crate::types::Attempt;
use crate::types::AttemptStatus;
use crate::types::LedgerSummary;
use crate::types::SideEffectLedgerEntry;

/// One step of a live side-effect plan.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SideEffectPlanStep {
    pub tool_class: ToolClass,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub action_ref: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub source_ref: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub approval_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub downstream_ref: String,
    pub requested_outcome: LedgerOutcome,
    #[serde(default, skip_serializing_if = "AmbiguousCommitCause::is_empty")]
    pub ambiguous_cause: AmbiguousCommitCause,
}

/// Result of running a side-effect plan: the final attempt plus its ledger.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SideEffectPlanResult {
    pub attempt: Attempt,
    pub ledger: Vec<SideEffectLedgerEntry>,
}

impl Manager {
    /// Port of `Manager.RunSideEffectPlan`.
    pub async fn run_side_effect_plan(
        &self,
        validation_id: &str,
        steps: Vec<SideEffectPlanStep>,
    ) -> Result<SideEffectPlanResult, LiveValidationError> {
        let Some(mut attempt) = self.get_attempt(validation_id).await? else {
            return Err(LiveValidationError::NotFound(validation_id.to_string()));
        };
        if attempt.status != AttemptStatus::RUNNING {
            return Err(LiveValidationError::NotRunning(validation_id.to_string()));
        }
        let matrix = self.support_matrix()?;
        let included = tool_class_set(&attempt.requested_scope.included_tool_classes);
        let excluded = tool_class_set(&attempt.requested_scope.excluded_tool_classes);
        let has_explicit_includes = !included.is_empty();

        let mut ledger: Vec<SideEffectLedgerEntry> = Vec::with_capacity(steps.len());
        let mut summary: LedgerSummary = BTreeMap::new();
        for step in steps {
            let out_of_scope = excluded.contains(&step.tool_class)
                || (has_explicit_includes && !included.contains(&step.tool_class));
            if out_of_scope {
                let entry = self
                    .append_skipped_plan_step(&attempt, &matrix, &step)
                    .await?;
                *summary.entry(entry.outcome.clone()).or_insert(0) += 1;
                ledger.push(entry);
                continue;
            }
            let entry = self
                .execute_side_effect(SideEffectExecutionInput {
                    attempt: attempt.clone(),
                    tool_class: step.tool_class,
                    action_ref: step.action_ref,
                    source_ref: step.source_ref,
                    approval_id: step.approval_id,
                    downstream_ref: step.downstream_ref,
                    requested_outcome: step.requested_outcome,
                    ambiguous_cause: step.ambiguous_cause,
                })
                .await?;
            *summary.entry(entry.outcome.clone()).or_insert(0) += 1;
            ledger.push(entry);
        }

        let now = self.now();
        let operator_action_needed = summary
            .get(&LedgerOutcome::from(LedgerOutcome::OPERATOR_ACTION_NEEDED))
            .copied()
            .unwrap_or(0)
            > 0;
        let failed = summary
            .get(&LedgerOutcome::from(LedgerOutcome::FAILED))
            .copied()
            .unwrap_or(0)
            > 0;
        attempt.status = if operator_action_needed {
            AttemptStatus::from(AttemptStatus::OPERATOR_ACTION_NEEDED)
        } else if failed {
            AttemptStatus::from(AttemptStatus::FAILED)
        } else {
            AttemptStatus::from(AttemptStatus::COMPLETED)
        };
        attempt.ledger_summary = summary;
        attempt.updated_at = now;
        attempt.completed_at = Some(now);
        self.persist_attempt(&attempt).await?;
        Ok(SideEffectPlanResult { attempt, ledger })
    }

    async fn append_skipped_plan_step(
        &self,
        attempt: &Attempt,
        matrix: &Matrix,
        step: &SideEffectPlanStep,
    ) -> Result<SideEffectLedgerEntry, LiveValidationError> {
        let mut safety_class = SafetyClass::from(SafetyClass::UNSUPPORTED);
        if let Some(row) = matrix.row(&step.tool_class) {
            safety_class = row.safety_class;
        }
        let entry = self
            .append_ledger_entry(SideEffectLedgerEntry {
                validation_id: attempt.validation_id.clone(),
                tenant_id: attempt.tenant_id.clone(),
                candidate_id: attempt.candidate_id.clone(),
                source_ref: step.source_ref.clone(),
                tool_class: step.tool_class.clone(),
                safety_class,
                action_ref: step.action_ref.clone(),
                approval_id: step.approval_id.clone(),
                downstream_ref: step.downstream_ref.clone(),
                outcome: LedgerOutcome::from(LedgerOutcome::SKIPPED),
                reason_code: "live_validation.scope_excluded".to_string(),
                ..SideEffectLedgerEntry::default()
            })
            .await?;
        self.emit_ledger_event(LEDGER_EVENT_SIDE_EFFECT_RECORDED, &entry);
        Ok(entry)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Arc;

    use crate::ledger::LedgerOutcome;
    use crate::matrix::ToolClass;
    use crate::testutil::MemStore;
    use crate::testutil::fixed_clock;
    use crate::testutil::manager_with_store;
    use crate::types::AmbiguousCommitCause;
    use crate::types::Attempt;
    use crate::types::AttemptStatus;
    use crate::types::SideEffectScope;

    fn running_attempt(
        validation_id: &str,
        included: Vec<ToolClass>,
        excluded: Vec<ToolClass>,
    ) -> Attempt {
        Attempt {
            validation_id: validation_id.to_string(),
            tenant_id: "ten_1".to_string(),
            candidate_id: "candidate_1".to_string(),
            status: AttemptStatus::from(AttemptStatus::RUNNING),
            requested_scope: SideEffectScope {
                scope_id: "scope_1".to_string(),
                included_tool_classes: included,
                excluded_tool_classes: excluded,
                ..SideEffectScope::default()
            },
            created_at: fixed_clock(),
            updated_at: fixed_clock(),
            ..Attempt::default()
        }
    }

    #[tokio::test]
    async fn run_plan_records_skipped_and_completes_attempt() {
        let store = Arc::new(MemStore::default());
        store.state.lock().attempts.push(running_attempt(
            "lv_plan",
            vec![ToolClass::from(ToolClass::DAEMON_INSPECTION_READ)],
            vec![ToolClass::from(ToolClass::MCP_TOOL_CALL)],
        ));
        let manager = manager_with_store(store.clone());

        let result = manager
            .run_side_effect_plan(
                "lv_plan",
                vec![
                    SideEffectPlanStep {
                        tool_class: ToolClass::from(ToolClass::DAEMON_INSPECTION_READ),
                        action_ref: "inspect_1".to_string(),
                        source_ref: "tool_call_1".to_string(),
                        requested_outcome: LedgerOutcome::from(LedgerOutcome::COMPLETED),
                        ..SideEffectPlanStep::default()
                    },
                    SideEffectPlanStep {
                        tool_class: ToolClass::from(ToolClass::MCP_TOOL_CALL),
                        action_ref: "mcp_1".to_string(),
                        source_ref: "tool_call_2".to_string(),
                        ..SideEffectPlanStep::default()
                    },
                ],
            )
            .await
            .expect("run plan");

        assert_eq!(
            result.attempt.status,
            AttemptStatus::from(AttemptStatus::COMPLETED)
        );
        assert_eq!(
            result
                .attempt
                .ledger_summary
                .get(&LedgerOutcome::from(LedgerOutcome::COMPLETED)),
            Some(&1)
        );
        assert_eq!(
            result
                .attempt
                .ledger_summary
                .get(&LedgerOutcome::from(LedgerOutcome::SKIPPED)),
            Some(&1)
        );
        assert_eq!(result.ledger.len(), 2);
        assert_eq!(
            result.ledger[1].outcome,
            LedgerOutcome::from(LedgerOutcome::SKIPPED)
        );
    }

    #[tokio::test]
    async fn run_plan_marks_attempt_operator_action_needed() {
        let store = Arc::new(MemStore::default());
        store.state.lock().attempts.push(running_attempt(
            "lv_ambiguous_plan",
            vec![ToolClass::from(ToolClass::MAIL_SEND)],
            vec![],
        ));
        let manager = manager_with_store(store.clone());

        let result = manager
            .run_side_effect_plan(
                "lv_ambiguous_plan",
                vec![SideEffectPlanStep {
                    tool_class: ToolClass::from(ToolClass::MAIL_SEND),
                    action_ref: "send_1".to_string(),
                    source_ref: "tool_call_1".to_string(),
                    ambiguous_cause: AmbiguousCommitCause::from(AmbiguousCommitCause::TIMEOUT),
                    ..SideEffectPlanStep::default()
                }],
            )
            .await
            .expect("run plan");

        assert_eq!(
            result.attempt.status,
            AttemptStatus::from(AttemptStatus::OPERATOR_ACTION_NEEDED)
        );
        assert_eq!(
            result
                .attempt
                .ledger_summary
                .get(&LedgerOutcome::from(LedgerOutcome::OPERATOR_ACTION_NEEDED)),
            Some(&1)
        );
    }
}
