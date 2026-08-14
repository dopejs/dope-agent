//! workspace_bindings route family.

use axum::Router;

use crate::state::AppState;

/// Route family router.
#[must_use]
pub fn router() -> Router<AppState> {
    Router::new()
}
