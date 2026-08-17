//! `/v1/plugins` — plugin assembly introspection.
//!
//! Returns the [`dope_plugin::AssemblyReport`] recorded at boot: every known
//! plugin in build order with its resolved enablement (and the reason when
//! disabled), plus warnings for profile entries that matched nothing. This is
//! the daemon's `dump-config` equivalent for the plugin plane: what actually
//! assembled, not what the profile intended.

use axum::extract::State;
use axum::routing::get;
use axum::Router;

use crate::error::ApiError;
use crate::response::Json;
use crate::state::AppState;

/// GET /v1/plugins — the boot-time plugin assembly report.
#[allow(clippy::unused_async)]
pub async fn list_plugins(
    State(state): State<AppState>,
) -> Result<Json<dope_plugin::AssemblyReport>, ApiError> {
    let report = state
        .plugins
        .as_ref()
        .ok_or_else(|| ApiError::internal("plugin assembly report is not configured"))?;
    Ok(Json((**report).clone()))
}

/// The `/v1/plugins` route family.
pub fn router() -> Router<AppState> {
    Router::new().route("/v1/plugins", get(list_plugins))
}

#[cfg(test)]
mod tests {
    use std::sync::Arc;

    use axum::http::StatusCode;

    use super::super::tests_support::{request_json, test_state};

    #[tokio::test]
    async fn list_plugins_returns_report() {
        let mut state = test_state();
        let report = dope_plugin::resolve(
            &[
                dope_plugin::PluginDescriptor {
                    id: "alpha",
                    summary: "test plugin",
                    provides: &["alpha.svc"],
                    requires: &[],
                },
                dope_plugin::PluginDescriptor {
                    id: "beta",
                    summary: "dependent",
                    provides: &[],
                    requires: &["alpha"],
                },
            ],
            &dope_plugin::PluginProfile {
                disabled: vec!["alpha".to_string()],
                ..Default::default()
            },
        );
        state.plugins = Some(Arc::new(report));

        let (status, json) = request_json(state, "GET", "/v1/plugins", None).await;
        assert_eq!(status, StatusCode::OK);
        assert_eq!(json["plugins"][0]["id"], "alpha");
        assert_eq!(json["plugins"][0]["enabled"], false);
        assert_eq!(json["plugins"][0]["reason"], "disabled by profile");
        assert_eq!(json["plugins"][0]["source"], "builtin");
        assert_eq!(json["plugins"][1]["id"], "beta");
        assert_eq!(json["plugins"][1]["reason"], "requires disabled plugin `alpha`");
        assert_eq!(json["warnings"], serde_json::json!([]));
    }

    #[tokio::test]
    async fn list_plugins_without_report_is_internal() {
        let (status, json) = request_json(test_state(), "GET", "/v1/plugins", None).await;
        assert_eq!(status, StatusCode::INTERNAL_SERVER_ERROR);
        assert_eq!(json["error"], "plugin assembly report is not configured");
    }
}
