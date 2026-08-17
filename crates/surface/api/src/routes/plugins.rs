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
use serde::Serialize;

use crate::error::ApiError;
use crate::response::Json;
use crate::state::AppState;

/// One hook-bus registration in the report.
#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct HookRegistration {
    pub point: String,
    pub plugin_id: String,
}

/// GET /v1/plugins response: the assembly report plus the hook-bus
/// registrations made during assembly.
#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct PluginsResponse {
    #[serde(flatten)]
    pub report: dope_plugin::AssemblyReport,
    pub hooks: Vec<HookRegistration>,
}

/// GET /v1/plugins — the boot-time plugin assembly report.
#[allow(clippy::unused_async)]
pub async fn list_plugins(State(state): State<AppState>) -> Result<Json<PluginsResponse>, ApiError> {
    let report = state
        .plugins
        .as_ref()
        .ok_or_else(|| ApiError::internal("plugin assembly report is not configured"))?;
    let hooks = state
        .hooks
        .as_ref()
        .map(|bus| {
            bus.registrations()
                .into_iter()
                .map(|(point, plugin_id)| HookRegistration { point, plugin_id })
                .collect()
        })
        .unwrap_or_default();
    Ok(Json(PluginsResponse { report: (**report).clone(), hooks }))
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

        let bus = dope_plugin::HookBus::new();
        struct Noop;
        impl dope_plugin::Hook for Noop {
            fn handle(&self, _p: &mut serde_json::Value) -> dope_plugin::HookOutcome {
                dope_plugin::HookOutcome::Continue
            }
        }
        bus.register(dope_plugin::points::CHAT_TURN_END, "beta", Arc::new(Noop));
        state.hooks = Some(Arc::new(bus));

        let (status, json) = request_json(state, "GET", "/v1/plugins", None).await;
        assert_eq!(status, StatusCode::OK);
        assert_eq!(json["hooks"][0]["point"], "chat/turn-end");
        assert_eq!(json["hooks"][0]["pluginId"], "beta");
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
