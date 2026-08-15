//! Side-effect ledger evidence (port of `ledger.go`): outcomes, terminal
//! detection, transition validation, and the manager's ledger operations.

use chrono::DateTime;
use chrono::Utc;

use crate::error::LiveValidationError;
use crate::manager::Manager;
use crate::store::LedgerFilter;
use crate::types::SideEffectLedgerEntry;

define_string_enum!(
    /// Recorded outcome of one side-effect ledger entry.
    LedgerOutcome {
        ATTEMPTED => "attempted",
        SKIPPED => "skipped",
        COMPLETED => "completed",
        FAILED => "failed",
        ABORTED => "aborted",
        DENIED => "denied",
        OPERATOR_ACTION_NEEDED => "operator_action_needed"
    }
);

/// Port of `IsTerminalLedgerOutcome`.
#[must_use]
pub fn is_terminal_ledger_outcome(outcome: &LedgerOutcome) -> bool {
    matches!(
        outcome.as_str(),
        LedgerOutcome::SKIPPED
            | LedgerOutcome::COMPLETED
            | LedgerOutcome::ABORTED
            | LedgerOutcome::DENIED
            | LedgerOutcome::OPERATOR_ACTION_NEEDED
            | LedgerOutcome::FAILED
    )
}

/// Port of `knownLedgerOutcome`.
#[must_use]
pub fn known_ledger_outcome(outcome: &LedgerOutcome) -> bool {
    matches!(
        outcome.as_str(),
        LedgerOutcome::ATTEMPTED
            | LedgerOutcome::SKIPPED
            | LedgerOutcome::COMPLETED
            | LedgerOutcome::FAILED
            | LedgerOutcome::ABORTED
            | LedgerOutcome::DENIED
            | LedgerOutcome::OPERATOR_ACTION_NEEDED
    )
}

/// Port of `ValidateLedgerTransition`. An empty `from` is the initial
/// append and is always allowed for a known outcome.
pub fn validate_ledger_transition(
    from: &LedgerOutcome,
    to: &LedgerOutcome,
) -> Result<(), LiveValidationError> {
    if !known_ledger_outcome(to) {
        return Err(LiveValidationError::LedgerOutcomeUnknown(to.to_string()));
    }
    if from.is_empty() {
        return Ok(());
    }
    if !known_ledger_outcome(from) {
        return Err(LiveValidationError::LedgerOutcomeUnknown(from.to_string()));
    }
    if from == to {
        return Ok(());
    }
    match from.as_str() {
        LedgerOutcome::ATTEMPTED => match to.as_str() {
            LedgerOutcome::COMPLETED
            | LedgerOutcome::FAILED
            | LedgerOutcome::ABORTED
            | LedgerOutcome::OPERATOR_ACTION_NEEDED => return Ok(()),
            _ => {}
        },
        LedgerOutcome::FAILED if to.as_str() == LedgerOutcome::ATTEMPTED => {
            return Ok(());
        }
        _ => {}
    }
    Err(LiveValidationError::LedgerTransitionInvalid {
        from: from.to_string(),
        to: to.to_string(),
    })
}

impl Manager {
    /// Port of `Manager.AppendLedgerEntry`: fills defaults, stamps times, and
    /// persists the entry.
    pub async fn append_ledger_entry(
        &self,
        mut entry: SideEffectLedgerEntry,
    ) -> Result<SideEffectLedgerEntry, LiveValidationError> {
        if entry.outcome.is_empty() {
            entry.outcome = LedgerOutcome::from(LedgerOutcome::ATTEMPTED);
        }
        if !known_ledger_outcome(&entry.outcome) {
            return Err(LiveValidationError::LedgerOutcomeUnknown(
                entry.outcome.to_string(),
            ));
        }
        let now = self.now();
        if entry.ledger_entry_id.is_empty() {
            entry.ledger_entry_id = crate::manager::new_id("lv_ledger");
        }
        if entry.updated_at == DateTime::<Utc>::default() {
            entry.updated_at = now;
        }
        if entry.attempted_at.is_none() && entry.outcome == LedgerOutcome::ATTEMPTED {
            entry.attempted_at = Some(now);
        }
        if is_terminal_ledger_outcome(&entry.outcome)
            && entry.completed_at.is_none()
            && entry.outcome != LedgerOutcome::SKIPPED
            && entry.outcome != LedgerOutcome::DENIED
        {
            entry.completed_at = Some(now);
        }
        if let Some(store) = self.store() {
            store.append_ledger_entry(entry.clone()).await?;
        }
        Ok(entry)
    }

    /// Port of `Manager.UpdateLedgerOutcome`.
    pub async fn update_ledger_outcome(
        &self,
        ledger_entry_id: &str,
        outcome: &LedgerOutcome,
        reason_code: &str,
    ) -> Result<(), LiveValidationError> {
        if !known_ledger_outcome(outcome) {
            return Err(LiveValidationError::LedgerOutcomeUnknown(
                outcome.to_string(),
            ));
        }
        let Some(store) = self.store() else {
            return Ok(());
        };
        store
            .update_ledger_entry_outcome(ledger_entry_id, outcome, reason_code)
            .await
    }

    /// Port of `Manager.ListLedgerEntries`.
    pub async fn list_ledger_entries(
        &self,
        filter: LedgerFilter,
    ) -> Result<Vec<SideEffectLedgerEntry>, LiveValidationError> {
        let Some(store) = self.store() else {
            return Ok(Vec::new());
        };
        store.list_ledger_entries(filter).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn ledger_transitions() {
        let valid = [
            LedgerOutcome::COMPLETED,
            LedgerOutcome::FAILED,
            LedgerOutcome::ABORTED,
            LedgerOutcome::OPERATOR_ACTION_NEEDED,
        ];
        for outcome in valid {
            validate_ledger_transition(
                &LedgerOutcome::from(LedgerOutcome::ATTEMPTED),
                &LedgerOutcome::from(outcome),
            )
            .unwrap_or_else(|err| panic!("attempted -> {outcome} returned error: {err}"));
        }
        assert!(matches!(
            validate_ledger_transition(
                &LedgerOutcome::from(LedgerOutcome::COMPLETED),
                &LedgerOutcome::from(LedgerOutcome::ATTEMPTED),
            ),
            Err(LiveValidationError::LedgerTransitionInvalid { .. })
        ));
        validate_ledger_transition(
            &LedgerOutcome::default(),
            &LedgerOutcome::from(LedgerOutcome::SKIPPED),
        )
        .expect("initial skipped transition");
    }
}
