//! Error types (port of the package's sentinel errors).

use crate::manager::StartResult;

/// Support-matrix validation and lookup failures (Go `ErrMatrixRow*` /
/// `ErrUnsafeAutomaticRetry` / `ErrMatrixMissingLedgerOutcomes`).
#[derive(Debug, thiserror::Error)]
pub enum MatrixError {
    #[error("live validation support matrix row missing")]
    RowMissing,
    #[error("live validation support matrix row unsupported")]
    RowUnsupported,
    #[error("live validation support matrix row invalid")]
    RowInvalid,
    #[error(
        "live validation support matrix row invalid: unsupported rows must not declare runnable approval or compensation"
    )]
    UnsupportedRowRunnable,
    #[error("live validation support matrix row invalid: missing permission")]
    MissingPermission,
    #[error("non-idempotent matrix row cannot allow automatic retry")]
    UnsafeAutomaticRetry,
    #[error("live validation support matrix row missing proving test")]
    RowMissingTest,
    #[error("live validation support matrix row missing ledger outcomes")]
    MissingLedgerOutcomes,
}

/// Manager failures (Go `ErrLiveValidation*`, ledger transition errors, and
/// wrapped store/billing errors).
#[derive(Debug, thiserror::Error)]
pub enum LiveValidationError {
    /// Go `ErrLiveValidationDisabled`. Also returned by
    /// [`crate::Manager::create_comparison`] when the attempt is unknown,
    /// matching the Go quirk.
    #[error("live validation is disabled")]
    Disabled,
    #[error("live validation kill switch permission denied")]
    KillSwitchPermissionDenied,
    #[error("live validation reconciliation permission denied")]
    ReconciliationPermissionDenied,
    #[error("invalid live validation ledger transition: {from} -> {to}")]
    LedgerTransitionInvalid { from: String, to: String },
    #[error("unknown live validation ledger outcome: {0}")]
    LedgerOutcomeUnknown(String),
    #[error("live validation {0} not found")]
    NotFound(String),
    #[error("live validation {0} is not running")]
    NotRunning(String),
    #[error(transparent)]
    Matrix(#[from] MatrixError),
    #[error(transparent)]
    Billing(#[from] kura_billing::BillingError),
    #[error("live validation store error: {0}")]
    Store(String),
}

/// Failure of [`crate::Manager::start`]. Go returns the partial
/// [`StartResult`] alongside `ErrLiveValidationBlocked`; the blocked variant
/// carries it here so callers can still inspect the attempt and denials.
#[derive(Debug, thiserror::Error)]
pub enum StartFailure {
    #[error("live validation is disabled")]
    Disabled,
    #[error("live validation blocked")]
    Blocked(StartResult),
    #[error(transparent)]
    Internal(#[from] LiveValidationError),
}
