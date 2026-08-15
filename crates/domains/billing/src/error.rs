//! Billing error type (port of the Go sentinel errors in `manager.go` and the
//! ad-hoc `fmt.Errorf` failure modes).

/// Errors produced by billing accounting, admin operations, and repositories.
///
/// The first five variants correspond to the Go sentinel errors
/// (`ErrQuotaDenied`, `ErrQuotaStateUnavailable`, `ErrNegativeEffectiveUsage`,
/// `ErrReasonRequired`, `ErrOperatorActionRequired`,
/// `ErrReservationNotFound`).
#[derive(Debug, Clone, thiserror::Error)]
pub enum BillingError {
    #[error("quota denied")]
    QuotaDenied,
    #[error("quota state unavailable")]
    QuotaStateUnavailable,
    #[error("billing adjustment would make effective usage negative")]
    NegativeEffectiveUsage,
    #[error("billing reason is required")]
    ReasonRequired,
    #[error("billing reservation requires operator action")]
    OperatorActionRequired,
    #[error("billing reservation not found for {0}")]
    ReservationNotFound(String),
    #[error("reservation not found for {0}")]
    ReservationIdNotFound(String),
    #[error("unknown quota category {0:?}")]
    UnknownCategory(String),
    #[error("counter not found for reservation {0}")]
    CounterNotFound(String),
    #[error("unsupported reservation resolution outcome {0:?}")]
    UnsupportedResolutionOutcome(String),
    /// The repository does not implement an optional capability (Go:
    /// interface assertion failure in admin/lookup paths).
    #[error("billing repository does not support {0}")]
    NotSupported(&'static str),
    /// Repository implementations report backend failures through this.
    #[error("billing repository error: {0}")]
    Repository(String),
}

/// Result alias for billing operations.
pub type Result<T, E = BillingError> = std::result::Result<T, E>;
