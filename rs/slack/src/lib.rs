//! Port of the Go slack channel connector (daemon/internal/connectors/slack).
//!
//! The crate implements the Slack channel-connector boundary behind the shared
//! connector contracts: route policy + routing decisions, hosted OAuth setup
//! readiness, workspace bindings and destinations, diagnostics, smoke
//! evidence, the Web API REST transport (ureq), a fake transport for tests,
//! and the runtime that wires the supervisor, message loop, store, and event
//! bus together. See rs/MIGRATION.md for the porting conventions.
//!
//! tenantctx is not ported: the Go runtime's context tenant id is passed
//! explicitly through Config::tenant_id (falling back to the store's default
//! personal tenant), faithful to where the Go code sources it.

pub mod destinations;
pub mod diagnostics;
pub mod error;
pub mod readiness;
pub mod route;
pub mod runtime;
pub mod smoke;
pub mod transport;
pub mod transport_webapi;
pub mod util;

pub use destinations::*;
pub use diagnostics::*;
pub use error::*;
pub use readiness::*;
pub use route::*;
pub use runtime::*;
pub use smoke::*;
pub use transport::*;
pub use transport_webapi::*;
