//! Side-effect execution (port of `executor.go`).

use serde::Deserialize;
use serde::Serialize;

use crate::error::LiveValidationError;
use crate::events::LEDGER_EVENT_OPERATOR_ACTION_NEEDED;
use crate::events::LEDGER_EVENT_SIDE_EFFECT_RECORDED;
use crate::idempotency::correlation_key;
use crate::ledger::LedgerOutcome;
use crate::manager::Manager;
use crate::matrix::SafetyClass;
use crate::matrix::ToolClass;
use crate::types::AmbiguousCommit;
use crate::types::AmbiguousCommitCause;
use crate::types::Attempt;
use crate::types::SideEffectLedgerEntry;

/// Input to [`Manager::execute_side_effect`].
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SideEffectExecutionInput {
    pub attempt: Attempt,
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

impl Manager {
    /// Port of `Manager.ExecuteSideEffect`.
    pub async fn execute_side_effect(
        &self,
        mut input: SideEffectExecutionInput,
    ) -> Result<SideEffectLedgerEntry, LiveValidationError> {
        let now = self.now();
        let (decision, denial) = self
            .evaluate_kill_switch(&input.attempt.tenant_id, now)
            .await?;
        if !decision.allowed {
            let aborted = self
                .append_ledger_entry(SideEffectLedgerEntry {
                    validation_id: input.attempt.validation_id.clone(),
                    tenant_id: input.attempt.tenant_id.clone(),
                    candidate_id: input.attempt.candidate_id.clone(),
                    source_ref: input.source_ref.clone(),
                    tool_class: input.tool_class.clone(),
                    action_ref: input.action_ref.clone(),
                    approval_id: input.approval_id.clone(),
                    downstream_ref: input.downstream_ref.clone(),
                    outcome: LedgerOutcome::from(LedgerOutcome::ABORTED),
                    reason_code: denial.reason_code,
                    updated_at: now,
                    ..SideEffectLedgerEntry::default()
                })
                .await?;
            self.emit_ledger_event(LEDGER_EVENT_SIDE_EFFECT_RECORDED, &aborted);
            return Ok(aborted);
        }

        let matrix = self.support_matrix()?;
        let row = match matrix.lookup(&input.tool_class) {
            Ok(row) => row,
            Err(_) => {
                let denied = self
                    .append_ledger_entry(SideEffectLedgerEntry {
                        validation_id: input.attempt.validation_id.clone(),
                        tenant_id: input.attempt.tenant_id.clone(),
                        candidate_id: input.attempt.candidate_id.clone(),
                        source_ref: input.source_ref.clone(),
                        tool_class: input.tool_class.clone(),
                        safety_class: SafetyClass::from(SafetyClass::UNSUPPORTED),
                        action_ref: input.action_ref.clone(),
                        approval_id: input.approval_id.clone(),
                        downstream_ref: input.downstream_ref.clone(),
                        outcome: LedgerOutcome::from(LedgerOutcome::DENIED),
                        reason_code: "live_validation.unsupported_tool_class".to_string(),
                        ..SideEffectLedgerEntry::default()
                    })
                    .await?;
                self.emit_ledger_event(LEDGER_EVENT_SIDE_EFFECT_RECORDED, &denied);
                return Ok(denied);
            }
        };

        let ledger_entry_id = crate::manager::new_id("lv_ledger");
        let correlation_key = correlation_key(
            &input.attempt.validation_id,
            &ledger_entry_id,
            &input.action_ref,
        );
        let mut attempted = self
            .append_ledger_entry(SideEffectLedgerEntry {
                ledger_entry_id,
                validation_id: input.attempt.validation_id.clone(),
                tenant_id: input.attempt.tenant_id.clone(),
                candidate_id: input.attempt.candidate_id.clone(),
                source_ref: input.source_ref.clone(),
                tool_class: input.tool_class.clone(),
                safety_class: row.safety_class,
                action_ref: input.action_ref.clone(),
                approval_id: input.approval_id.clone(),
                downstream_ref: input.downstream_ref.clone(),
                outcome: LedgerOutcome::from(LedgerOutcome::ATTEMPTED),
                correlation_key,
                ..SideEffectLedgerEntry::default()
            })
            .await?;
        self.emit_ledger_event(LEDGER_EVENT_SIDE_EFFECT_RECORDED, &attempted);

        if input.requested_outcome.is_empty() {
            input.requested_outcome = LedgerOutcome::from(LedgerOutcome::COMPLETED);
        }
        if input.requested_outcome == LedgerOutcome::OPERATOR_ACTION_NEEDED
            || !input.ambiguous_cause.is_empty()
        {
            attempted.ambiguous_commit = true;
            self.record_ambiguous_commit(AmbiguousCommit {
                ledger_entry_id: attempted.ledger_entry_id.clone(),
                validation_id: attempted.validation_id.clone(),
                tenant_id: attempted.tenant_id.clone(),
                cause: input.ambiguous_cause,
                last_known_request_ref: attempted.correlation_key.clone(),
                ..AmbiguousCommit::default()
            })
            .await?;
            attempted.outcome = LedgerOutcome::from(LedgerOutcome::OPERATOR_ACTION_NEEDED);
            self.emit_ledger_event(LEDGER_EVENT_OPERATOR_ACTION_NEEDED, &attempted);
            return Ok(attempted);
        }

        self.update_ledger_outcome(
            &attempted.ledger_entry_id,
            &input.requested_outcome,
            "live_validation.executor_result",
        )
        .await?;
        attempted.outcome = input.requested_outcome;
        self.emit_ledger_event(LEDGER_EVENT_SIDE_EFFECT_RECORDED, &attempted);
        Ok(attempted)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Arc;

    use parking_lot::Mutex;

    use crate::manager::Clock;
    use crate::manager::Dependencies;
    use crate::matrix::ToolClass;
    use crate::testutil::MemStore;
    use crate::testutil::fixed_clock;
    use crate::types::AmbiguousCommitCause;
    use crate::types::Attempt;

    fn manager_with_events() -> (Manager, Arc<MemStore>, Arc<Mutex<Vec<String>>>) {
        let store = Arc::new(MemStore::default());
        let events: Arc<Mutex<Vec<String>>> = Arc::new(Mutex::new(Vec::new()));
        let events_sink = events.clone();
        let clock: Clock = Arc::new(fixed_clock);
        let manager = Manager::new(Dependencies {
            environment_scope: "test".to_string(),
            store: Some(store.clone()),
            enabled: true,
            billing: None,
            hosted_billing: false,
            clock: Some(clock),
            ledger_event_sink: Some(Arc::new(
                move |name: &str, _entry: &SideEffectLedgerEntry| {
                    events_sink.lock().push(name.to_string());
                },
            )),
            candidate_tool_class_resolver: None,
        });
        (manager, store, events)
    }

    #[tokio::test]
    async fn propagates_correlation_key_and_terminal_outcome() {
        let (manager, store, events) = manager_with_events();
        let entry = manager
            .execute_side_effect(SideEffectExecutionInput {
                attempt: Attempt {
                    validation_id: "lv_1".to_string(),
                    tenant_id: "ten_1".to_string(),
                    candidate_id: "candidate_1".to_string(),
                    ..Attempt::default()
                },
                tool_class: ToolClass::from(ToolClass::INTEGRATION_PROBE_MUTATION),
                action_ref: "action_1".to_string(),
                requested_outcome: LedgerOutcome::from(LedgerOutcome::COMPLETED),
                ..SideEffectExecutionInput::default()
            })
            .await
            .expect("execute side effect");

        assert_eq!(
            entry.correlation_key,
            format!("live_validation:lv_1:{}:action_1", entry.ledger_entry_id)
        );
        let state = store.state.lock();
        assert_eq!(state.ledger.len(), 1);
        assert_eq!(
            state.ledger[0].outcome,
            LedgerOutcome::from(LedgerOutcome::COMPLETED)
        );
        assert_eq!(state.ledger[0].correlation_key, entry.correlation_key);
        drop(state);
        let ev = events.lock();
        assert_eq!(ev.len(), 2);
        assert_eq!(ev[0], LEDGER_EVENT_SIDE_EFFECT_RECORDED);
        assert_eq!(ev[1], LEDGER_EVENT_SIDE_EFFECT_RECORDED);
    }

    #[tokio::test]
    async fn records_ambiguous_commit_and_stops_retry() {
        let (manager, _store, events) = manager_with_events();
        let entry = manager
            .execute_side_effect(SideEffectExecutionInput {
                attempt: Attempt {
                    validation_id: "lv_1".to_string(),
                    tenant_id: "ten_1".to_string(),
                    candidate_id: "candidate_1".to_string(),
                    ..Attempt::default()
                },
                tool_class: ToolClass::from(ToolClass::MAIL_SEND),
                action_ref: "send_1".to_string(),
                ambiguous_cause: AmbiguousCommitCause::from(AmbiguousCommitCause::TIMEOUT),
                ..SideEffectExecutionInput::default()
            })
            .await
            .expect("execute side effect");

        assert_eq!(
            entry.outcome,
            LedgerOutcome::from(LedgerOutcome::OPERATOR_ACTION_NEEDED)
        );
        assert!(entry.ambiguous_commit);
        let ev = events.lock();
        assert_eq!(ev.len(), 2);
        assert_eq!(ev[1], LEDGER_EVENT_OPERATOR_ACTION_NEEDED);
    }
}
