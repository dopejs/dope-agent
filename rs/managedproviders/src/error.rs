//! Error taxonomy for the managed-provider bridge layer.
//!
//! The Go package surfaces plain `error` values whose text is folded into
//! auth-state fields (`LastError`, check messages) or into structured
//! runner errors. The Rust port keeps the same information but classifies it so
//! callers can branch on the failure kind without string matching.

use std::fmt;

use crate::bridge::RunError;
use crate::evaluate::ManagedProviderOperationEvaluation;

/// A sandbox policy denial that carries the (finalized) evaluation that was
/// denied, mirroring Go's `(evaluation, true, errors.New(...))` convention:
/// the caller that reports the denial still needs the evaluation's metadata and
/// consumer view.
#[derive(Debug, Clone)]
pub struct DeniedEvaluation {
    /// The denial message (Go `errors.New("sandbox denied managed provider local state access")`).
    pub message: String,
    /// The evaluation with `failureClass` finalized into its metadata.
    pub evaluation: ManagedProviderOperationEvaluation,
}

/// Errors produced by the managed-provider registry, bridges, and manager.
#[derive(Debug, Clone)]
pub enum Error {
    /// A feature deliberately deferred in this port (see the crate docs and
    /// README): the caller gets a stable, documented message instead of a
    /// partial implementation.
    Deferred(String),
    /// A sandbox policy denial, with the evaluation that was denied.
    Denied(DeniedEvaluation),
    /// A structured runner failure (exec or sandbox execution).
    Run(RunError),
    /// A store persistence/read failure.
    Store(String),
    /// An I/O failure (file read/write, temp file creation).
    Io(String),
    /// A JSON decode failure.
    Decode(String),
    /// Any other failure, surfaced by its message.
    Other(String),
}

impl Error {
    /// Maps a crate error into the `dope-providers` error surface so the
    /// registry can be consumed through `dope_providers::ManagedRegistry`.
    #[must_use]
    pub fn map_providers_error(err: Error) -> dope_providers::ProvidersError {
        dope_providers::ProvidersError::ProviderAuthUnavailable(err.to_string())
    }
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Error::Deferred(message)
            | Error::Store(message)
            | Error::Io(message)
            | Error::Decode(message)
            | Error::Other(message) => f.write_str(message),
            Error::Run(run_err) => write!(f, "{run_err}"),
            Error::Denied(denied) => f.write_str(&denied.message),
        }
    }
}

impl std::error::Error for Error {}

impl From<RunError> for Error {
    fn from(value: RunError) -> Self {
        Error::Run(value)
    }
}

impl From<String> for Error {
    fn from(value: String) -> Self {
        Error::Other(value)
    }
}

impl From<&str> for Error {
    fn from(value: &str) -> Self {
        Error::Other(value.to_string())
    }
}
