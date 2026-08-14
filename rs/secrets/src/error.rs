//! Error type mirroring the Go sentinel errors (`ErrTenantRequired`, ...).

/// Errors mirroring the Go package's sentinel errors. Go callers use
/// `errors.Is`; Rust callers match on variants.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum SecretsError {
    #[error("tenant id is required")]
    TenantRequired,
    #[error("secret ref is required")]
    SecretRefRequired,
    #[error("secret value is required")]
    SecretValueRequired,
    #[error("tenant secret not found")]
    SecretNotFound,
    #[error("tenant secret is disabled")]
    SecretDisabled,
    #[error("tenant secret version not found")]
    SecretVersionNotFound,
    #[error("cross-tenant credential access denied")]
    CrossTenantSecret,
    /// Go: `fmt.Errorf("tenant secret already exists: %s", ref)`. The Go
    /// `Manager.Create`/`CreateDisabledMetadata` also return the existing
    /// secret alongside the error; Rust's `Result` carries only the error.
    #[error("tenant secret already exists: {0}")]
    SecretAlreadyExists(String),
    #[error("secret id and version id are required")]
    SecretIdsRequired,
    #[error("secret backend root is required")]
    BackendRootRequired,
    #[error("secret backend is not configured")]
    BackendNotConfigured,
    #[error("secret backend ref escapes root")]
    BackendRefEscapesRoot,
    /// Wrapped store/persistence failure (Go: `fmt.Errorf(... %w)` chains and
    /// arbitrary store errors propagated through the manager).
    #[error("secret store error: {0}")]
    Store(String),
    /// Wrapped value-backend failure (Go: `create/write/read/delete secret
    /// value: %w` and backend-root setup errors).
    #[error("secret backend error: {0}")]
    Backend(String),
    /// Local credential bridge file failure (Go: `read/decode local credential
    /// file <base>: %w`); carries the full formatted message.
    #[error("{0}")]
    CredentialFile(String),
}

pub type Result<T, E = SecretsError> = std::result::Result<T, E>;
