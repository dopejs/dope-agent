//! Error types for the activation service (port of the Go `Error` struct and
//! `ReasonCodeFromError` helper in `service.go`).

use crate::types::FailureStage;
use crate::types::ReasonCode;
use crate::types::RemediationOwner;

/// Error type returned by the dependency traits ([`crate::StateStore`],
/// [`crate::IdentityRepository`], [`crate::ChatRunner`], [`crate::AuditSink`]).
/// Implementations report backend failures through it; the service wraps them
/// in [`ActivationError::Dependency`].
pub type StoreError = Box<dyn std::error::Error + Send + Sync>;

/// Stable activation failure with reason metadata (Go `*activation.Error`).
///
/// `Display` matches Go's `Error()`: the message when present, otherwise the
/// reason code.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Error {
    pub reason_code: ReasonCode,
    pub stage: FailureStage,
    pub retryable: bool,
    pub remediation_owner: RemediationOwner,
    pub message: String,
}

impl std::fmt::Display for Error {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        if !self.message.is_empty() {
            return f.write_str(&self.message);
        }
        f.write_str(self.reason_code.as_str())
    }
}

impl std::error::Error for Error {}

/// Top-level error returned by the activation service.
///
/// Dependency (store/repository) failures propagate without a stable reason
/// code, mirroring Go's raw error returns; domain failures carry the stable
/// [`Error`] payload.
#[derive(Debug, thiserror::Error)]
pub enum ActivationError {
    #[error(transparent)]
    Domain(#[from] Error),
    #[error("activation dependency failed: {0}")]
    Dependency(String),
}

impl ActivationError {
    /// Wraps a dependency-trait failure, preserving only its message (Go
    /// propagates the raw error; callers only ever read `Error()` off it).
    pub(crate) fn dependency(err: StoreError) -> Self {
        Self::Dependency(err.to_string())
    }
}

/// Extracts the stable reason code from an activation error, or an empty
/// code for non-domain failures (Go `ReasonCodeFromError`).
#[must_use]
pub fn reason_code_from_error(err: &ActivationError) -> ReasonCode {
    match err {
        ActivationError::Domain(domain) => domain.reason_code.clone(),
        ActivationError::Dependency(_) => ReasonCode::default(),
    }
}

/// Builds a stable domain error (Go `activationError`).
pub(crate) fn activation_error(
    reason: ReasonCode,
    stage: FailureStage,
    retryable: bool,
    owner: RemediationOwner,
    message: impl Into<String>,
) -> ActivationError {
    ActivationError::Domain(Error {
        reason_code: reason,
        stage,
        retryable,
        remediation_owner: owner,
        message: message.into(),
    })
}
