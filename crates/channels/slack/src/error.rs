//! Crate-level error type (Go package errors are plain error values; the
//! port groups them into one thiserror enum).

use crate::transport_webapi::WebApiError;

/// Errors surfaced by the slack connector crate. Display strings match the Go
/// sentinel errors exactly.
#[derive(Debug, thiserror::Error)]
pub enum SlackError {
    #[error("slack connector id is required")]
    ConnectorIdRequired,
    #[error("slack display name is required")]
    DisplayNameRequired,
    #[error("connector id is required")]
    DiagnosticConnectorRequired,
    #[error("diagnostic reason code is required")]
    DiagnosticReasonRequired,
    #[error("unsupported slack surface: {0}")]
    UnsupportedSurface(String),
    #[error("{0}")]
    Message(String),
    #[error(transparent)]
    WebApi(#[from] WebApiError),
}
