//! Support matrix (port of `matrix.go`): per-tool-class safety policy for
//! live replay.

use std::collections::HashMap;

use serde::Deserialize;
use serde::Serialize;

use crate::error::MatrixError;
use crate::ledger::LedgerOutcome;

define_string_enum!(
    /// Tool class covered by the live-validation support matrix.
    ToolClass {
        DAEMON_INSPECTION_READ => "daemon.inspection.read",
        RUNTIME_LOCAL_TOOL_CALL => "runtime.local_tool_call",
        MCP_TOOL_CALL => "mcp.tool_call",
        INTEGRATION_PROBE_READ => "integration.probe.read",
        INTEGRATION_PROBE_MUTATION => "integration.probe.mutation",
        CALENDAR_EVENT_CREATE => "calendar.event.create",
        CALENDAR_EVENT_UPDATE => "calendar.event.update",
        CALENDAR_EVENT_CANCEL => "calendar.event.cancel",
        CALENDAR_ATTENDEE_UPDATE => "calendar.attendee.update",
        MAIL_DRAFT_CREATE => "mail.draft.create",
        MAIL_DRAFT_UPDATE => "mail.draft.update",
        MAIL_SEND => "mail.send",
        MAIL_REPLY => "mail.reply",
        MAIL_FORWARD => "mail.forward",
        REMINDER_LIFECYCLE_MUTATION => "reminder.lifecycle.mutation",
        DELIVERY_DISPATCH => "delivery.dispatch",
        CONNECTOR_MESSAGE_SEND => "connector.message.send",
        PROVIDER_SANDBOX_UNSUPPORTED => "provider.sandbox.unsupported"
    }
);

define_string_enum!(
    /// Side-effect safety classification of a tool class.
    SafetyClass {
        READ_ONLY => "read_only",
        IDEMPOTENT_MUTATION => "idempotent_mutation",
        NON_IDEMPOTENT_MUTATION => "non_idempotent_mutation",
        UNSUPPORTED => "unsupported"
    }
);

define_string_enum!(
    /// Approval requirement declared by a matrix row.
    MatrixApproval {
        NOT_REQUIRED => "not_required",
        SCOPE_LEVEL => "scope_level",
        PER_ACTION => "per_action",
        UNSUPPORTED => "unsupported"
    }
);

define_string_enum!(
    /// Retry policy declared by a matrix row.
    RetryPolicy {
        AUTOMATIC => "automatic_retry",
        MANUAL => "manual_retry",
        NONE => "no_retry"
    }
);

define_string_enum!(
    /// Compensation strategy declared by a matrix row.
    CompensationKind {
        NOT_APPLICABLE => "not_applicable",
        AUTOMATIC => "automatic_compensation",
        MANUAL_CONFIRMATION => "manual_confirmation",
        UNSUPPORTED => "unsupported"
    }
);

/// One support-matrix row: the safety policy for replaying a tool class
/// against live systems.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct MatrixRow {
    pub tool_class: ToolClass,
    pub safety_class: SafetyClass,
    pub permission: String,
    pub approval: MatrixApproval,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub approval_action: String,
    pub idempotency: String,
    pub retry_policy: RetryPolicy,
    pub ambiguous_commit_behavior: String,
    pub compensation: CompensationKind,
    pub ledger_events: Vec<LedgerOutcome>,
    pub test_case: String,
    pub version: String,
}

impl MatrixRow {
    /// Port of `MatrixRow.Validate`.
    pub fn validate(&self) -> Result<(), MatrixError> {
        if self.tool_class.is_empty()
            || self.safety_class.is_empty()
            || self.retry_policy.is_empty()
        {
            return Err(MatrixError::RowInvalid);
        }
        if self.safety_class == SafetyClass::UNSUPPORTED {
            if self.approval != MatrixApproval::UNSUPPORTED
                && self.compensation != CompensationKind::UNSUPPORTED
            {
                return Err(MatrixError::UnsupportedRowRunnable);
            }
        } else if self.permission.is_empty() {
            return Err(MatrixError::MissingPermission);
        }
        if self.safety_class == SafetyClass::NON_IDEMPOTENT_MUTATION
            && self.retry_policy == RetryPolicy::AUTOMATIC
        {
            return Err(MatrixError::UnsafeAutomaticRetry);
        }
        if self.ledger_events.is_empty() {
            return Err(MatrixError::MissingLedgerOutcomes);
        }
        if self.test_case.is_empty() {
            return Err(MatrixError::RowMissingTest);
        }
        Ok(())
    }
}

/// The live-validation support matrix (Go `Matrix`).
#[derive(Debug, Clone, Default)]
pub struct Matrix {
    rows: HashMap<ToolClass, MatrixRow>,
}

impl Matrix {
    /// Port of `NewMatrix`: validates every row before indexing it.
    pub fn new(rows: Vec<MatrixRow>) -> Result<Self, MatrixError> {
        let mut matrix = Matrix {
            rows: HashMap::with_capacity(rows.len()),
        };
        for row in rows {
            row.validate()?;
            matrix.rows.insert(row.tool_class.clone(), row);
        }
        Ok(matrix)
    }

    /// Port of `Matrix.Lookup`: missing rows and unsupported rows are errors.
    pub fn lookup(&self, tool_class: &ToolClass) -> Result<MatrixRow, MatrixError> {
        let row = self.rows.get(tool_class).ok_or(MatrixError::RowMissing)?;
        if row.safety_class == SafetyClass::UNSUPPORTED {
            return Err(MatrixError::RowUnsupported);
        }
        Ok(row.clone())
    }

    /// Port of `Matrix.Row`: presence check without the unsupported verdict.
    pub fn row(&self, tool_class: &ToolClass) -> Option<MatrixRow> {
        self.rows.get(tool_class).cloned()
    }

    /// Port of `Matrix.Rows`: all rows sorted by tool class.
    pub fn rows(&self) -> Vec<MatrixRow> {
        let mut rows: Vec<MatrixRow> = self.rows.values().cloned().collect();
        rows.sort_by(|a, b| a.tool_class.cmp(&b.tool_class));
        rows
    }
}

/// Port of `DefaultMatrixRow`.
#[must_use]
pub fn default_matrix_row(tool_class: &ToolClass) -> Option<MatrixRow> {
    default_matrix_rows()
        .into_iter()
        .find(|row| &row.tool_class == tool_class)
}

/// Port of `DefaultMatrixRows`: the built-in v1 support matrix.
#[must_use]
pub fn default_matrix_rows() -> Vec<MatrixRow> {
    vec![
        supported_row(
            ToolClass::DAEMON_INSPECTION_READ,
            SafetyClass::READ_ONLY,
            MatrixApproval::SCOPE_LEVEL,
            RetryPolicy::AUTOMATIC,
            CompensationKind::NOT_APPLICABLE,
            "matrix completeness and read-only replay test",
        ),
        supported_row(
            ToolClass::RUNTIME_LOCAL_TOOL_CALL,
            SafetyClass::IDEMPOTENT_MUTATION,
            MatrixApproval::SCOPE_LEVEL,
            RetryPolicy::AUTOMATIC,
            CompensationKind::MANUAL_CONFIRMATION,
            "fake runtime local tool replay/restart test",
        ),
        unsupported_row(
            ToolClass::MCP_TOOL_CALL,
            "MCP unsupported completeness test",
        ),
        supported_row(
            ToolClass::INTEGRATION_PROBE_READ,
            SafetyClass::READ_ONLY,
            MatrixApproval::SCOPE_LEVEL,
            RetryPolicy::AUTOMATIC,
            CompensationKind::NOT_APPLICABLE,
            "fake integration read probe test",
        ),
        supported_row(
            ToolClass::INTEGRATION_PROBE_MUTATION,
            SafetyClass::IDEMPOTENT_MUTATION,
            MatrixApproval::SCOPE_LEVEL,
            RetryPolicy::MANUAL,
            CompensationKind::MANUAL_CONFIRMATION,
            "fake integration mutation timeout/retry test",
        ),
        supported_row(
            ToolClass::CALENDAR_EVENT_CREATE,
            SafetyClass::NON_IDEMPOTENT_MUTATION,
            MatrixApproval::PER_ACTION,
            RetryPolicy::NONE,
            CompensationKind::MANUAL_CONFIRMATION,
            "fake calendar create ambiguous commit test",
        ),
        supported_row(
            ToolClass::CALENDAR_EVENT_UPDATE,
            SafetyClass::IDEMPOTENT_MUTATION,
            MatrixApproval::SCOPE_LEVEL,
            RetryPolicy::MANUAL,
            CompensationKind::MANUAL_CONFIRMATION,
            "fake calendar update retry/reconciliation test",
        ),
        supported_row(
            ToolClass::CALENDAR_EVENT_CANCEL,
            SafetyClass::NON_IDEMPOTENT_MUTATION,
            MatrixApproval::PER_ACTION,
            RetryPolicy::NONE,
            CompensationKind::MANUAL_CONFIRMATION,
            "fake calendar cancel submit-unknown test",
        ),
        supported_row(
            ToolClass::CALENDAR_ATTENDEE_UPDATE,
            SafetyClass::NON_IDEMPOTENT_MUTATION,
            MatrixApproval::PER_ACTION,
            RetryPolicy::NONE,
            CompensationKind::MANUAL_CONFIRMATION,
            "fake calendar attendee invitation per-action approval test",
        ),
        supported_row(
            ToolClass::MAIL_DRAFT_CREATE,
            SafetyClass::IDEMPOTENT_MUTATION,
            MatrixApproval::SCOPE_LEVEL,
            RetryPolicy::MANUAL,
            CompensationKind::MANUAL_CONFIRMATION,
            "fake mail draft create test",
        ),
        supported_row(
            ToolClass::MAIL_DRAFT_UPDATE,
            SafetyClass::IDEMPOTENT_MUTATION,
            MatrixApproval::SCOPE_LEVEL,
            RetryPolicy::MANUAL,
            CompensationKind::MANUAL_CONFIRMATION,
            "fake mail draft update test",
        ),
        supported_row(
            ToolClass::MAIL_SEND,
            SafetyClass::NON_IDEMPOTENT_MUTATION,
            MatrixApproval::PER_ACTION,
            RetryPolicy::NONE,
            CompensationKind::MANUAL_CONFIRMATION,
            "fake mail send ambiguous commit test",
        ),
        supported_row(
            ToolClass::MAIL_REPLY,
            SafetyClass::NON_IDEMPOTENT_MUTATION,
            MatrixApproval::PER_ACTION,
            RetryPolicy::NONE,
            CompensationKind::MANUAL_CONFIRMATION,
            "fake mail reply ambiguous commit test",
        ),
        supported_row(
            ToolClass::MAIL_FORWARD,
            SafetyClass::NON_IDEMPOTENT_MUTATION,
            MatrixApproval::PER_ACTION,
            RetryPolicy::NONE,
            CompensationKind::MANUAL_CONFIRMATION,
            "fake mail forward ambiguous commit test",
        ),
        supported_row(
            ToolClass::REMINDER_LIFECYCLE_MUTATION,
            SafetyClass::IDEMPOTENT_MUTATION,
            MatrixApproval::SCOPE_LEVEL,
            RetryPolicy::AUTOMATIC,
            CompensationKind::MANUAL_CONFIRMATION,
            "fake reminder lifecycle test",
        ),
        supported_row(
            ToolClass::DELIVERY_DISPATCH,
            SafetyClass::NON_IDEMPOTENT_MUTATION,
            MatrixApproval::PER_ACTION,
            RetryPolicy::NONE,
            CompensationKind::MANUAL_CONFIRMATION,
            "fake delivery dispatch duplicate retry test",
        ),
        supported_row(
            ToolClass::CONNECTOR_MESSAGE_SEND,
            SafetyClass::NON_IDEMPOTENT_MUTATION,
            MatrixApproval::PER_ACTION,
            RetryPolicy::NONE,
            CompensationKind::MANUAL_CONFIRMATION,
            "fake connector message send test",
        ),
        unsupported_row(
            ToolClass::PROVIDER_SANDBOX_UNSUPPORTED,
            "unsupported provider/sandbox classification test",
        ),
    ]
}

fn supported_row(
    tool_class: &str,
    safety_class: &str,
    approval: &str,
    retry: &str,
    compensation: &str,
    test_case: &str,
) -> MatrixRow {
    MatrixRow {
        tool_class: ToolClass::from(tool_class),
        safety_class: SafetyClass::from(safety_class),
        permission: "live_validation.execute".to_string(),
        approval: MatrixApproval::from(approval),
        approval_action: "live_validation.approve".to_string(),
        idempotency: "validation correlation key".to_string(),
        retry_policy: RetryPolicy::from(retry),
        ambiguous_commit_behavior: LedgerOutcome::OPERATOR_ACTION_NEEDED.to_string(),
        compensation: CompensationKind::from(compensation),
        ledger_events: vec![
            LedgerOutcome::from(LedgerOutcome::ATTEMPTED),
            LedgerOutcome::from(LedgerOutcome::COMPLETED),
            LedgerOutcome::from(LedgerOutcome::FAILED),
            LedgerOutcome::from(LedgerOutcome::ABORTED),
            LedgerOutcome::from(LedgerOutcome::DENIED),
            LedgerOutcome::from(LedgerOutcome::OPERATOR_ACTION_NEEDED),
        ],
        test_case: test_case.to_string(),
        version: "v1".to_string(),
    }
}

fn unsupported_row(tool_class: &str, test_case: &str) -> MatrixRow {
    MatrixRow {
        tool_class: ToolClass::from(tool_class),
        safety_class: SafetyClass::from(SafetyClass::UNSUPPORTED),
        permission: String::new(),
        approval: MatrixApproval::from(MatrixApproval::UNSUPPORTED),
        approval_action: String::new(),
        idempotency: "not available".to_string(),
        retry_policy: RetryPolicy::from(RetryPolicy::NONE),
        ambiguous_commit_behavior: "unsupported validation state".to_string(),
        compensation: CompensationKind::from(CompensationKind::UNSUPPORTED),
        ledger_events: vec![
            LedgerOutcome::from(LedgerOutcome::SKIPPED),
            LedgerOutcome::from(LedgerOutcome::DENIED),
        ],
        test_case: test_case.to_string(),
        version: "v1".to_string(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn matrix_rows_validate_required_columns() {
        let matrix = Matrix::new(default_matrix_rows()).expect("default matrix must validate");
        for row in matrix.rows() {
            assert!(
                !row.tool_class.is_empty()
                    && !row.safety_class.is_empty()
                    && !row.approval.is_empty()
                    && !row.idempotency.is_empty()
                    && !row.retry_policy.is_empty()
                    && !row.ambiguous_commit_behavior.is_empty()
                    && !row.compensation.is_empty()
                    && !row.test_case.is_empty()
                    && !row.version.is_empty(),
                "matrix row has missing required columns: {row:?}"
            );
            if row.safety_class != SafetyClass::UNSUPPORTED {
                assert_eq!(
                    row.permission, "live_validation.execute",
                    "supported row {} missing execute permission",
                    row.tool_class
                );
            }
            assert!(
                !row.ledger_events.is_empty(),
                "matrix row {} missing ledger events",
                row.tool_class
            );
        }
    }

    #[test]
    fn matrix_missing_rows_are_unsupported() {
        let matrix = Matrix::new(default_matrix_rows()).expect("default matrix must validate");
        assert!(matches!(
            matrix.lookup(&ToolClass::from("unknown.future_tool")),
            Err(MatrixError::RowMissing)
        ));
        assert!(matches!(
            matrix.lookup(&ToolClass::from(ToolClass::MCP_TOOL_CALL)),
            Err(MatrixError::RowUnsupported)
        ));
    }

    #[test]
    fn matrix_rejects_unsafe_non_idempotent_automatic_retry() {
        let row = supported_row(
            ToolClass::MAIL_SEND,
            SafetyClass::NON_IDEMPOTENT_MUTATION,
            MatrixApproval::PER_ACTION,
            RetryPolicy::AUTOMATIC,
            CompensationKind::MANUAL_CONFIRMATION,
            "bad retry test",
        );
        assert!(matches!(
            row.validate(),
            Err(MatrixError::UnsafeAutomaticRetry)
        ));
    }
}
