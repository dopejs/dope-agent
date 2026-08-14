//! Error type for configuration loading.

use std::path::PathBuf;

/// Errors returned while resolving, reading, or decoding daemon configuration.
#[derive(Debug, thiserror::Error)]
pub enum ConfigError {
    /// A directory path was required but empty.
    #[error("path is required")]
    PathRequired,

    /// The user home directory could not be determined.
    #[error("resolve user home: {0}")]
    HomeResolution(String),

    /// The bootstrap data dir (from `DOPE_DATA_DIR` or the environment
    /// default) failed `~` resolution.
    #[error("resolve bootstrap data dir: {0}")]
    ResolveBootstrapDataDir(#[source] Box<ConfigError>),

    /// Creating the bootstrap data dir failed.
    #[error("initialize bootstrap data dir: {0}")]
    InitBootstrapDataDir(#[source] std::io::Error),

    /// Reading the config file failed for reasons other than nonexistence.
    #[error("read config file {}: {source}", path.display())]
    ReadFile {
        /// Path of the config file.
        path: PathBuf,
        /// Underlying I/O error.
        source: std::io::Error,
    },

    /// The config file existed but was not valid JSON for the file schema.
    #[error("decode config file {}: {source}", path.display())]
    DecodeFile {
        /// Path of the config file.
        path: PathBuf,
        /// Underlying JSON decode error.
        source: serde_json::Error,
    },

    /// The effective data dir (after file config and env overrides) failed
    /// `~` resolution.
    #[error("resolve effective data dir: {0}")]
    ResolveEffectiveDataDir(#[source] Box<ConfigError>),

    /// Creating the effective data dir failed.
    #[error("initialize effective data dir: {0}")]
    InitEffectiveDataDir(#[source] std::io::Error),
}
