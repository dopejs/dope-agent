//! `/v1/plugins` — plugin assembly introspection and profile management.
//!
//! `GET /v1/plugins` returns the [`dope_plugin::AssemblyReport`] recorded at
//! boot: every known plugin in build order with its resolved enablement (and
//! the reason when disabled), plus warnings for profile entries that matched
//! nothing. This is the daemon's `dump-config` equivalent for the plugin
//! plane: what actually assembled, not what the profile intended.
//!
//! `GET/PUT /v1/plugins/profile` read and replace the on-disk profile
//! (`<data_dir>/plugins.json`). The profile is boot-time input: a write is
//! validated and persisted atomically but takes effect at the next daemon
//! start (`restartRequired: true` in the response makes that explicit).

use std::path::Path;

use axum::body::Bytes;
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

/// GET /v1/plugins/profile — the on-disk plugin profile (what the next boot
/// assembles under). A missing file is the default (empty) profile.
#[allow(clippy::unused_async)]
pub async fn get_profile(
    State(state): State<AppState>,
) -> Result<Json<dope_plugin::PluginProfile>, ApiError> {
    dope_plugin::PluginProfile::load(&state.config.data_dir)
        .map(Json)
        .map_err(|err| ApiError::internal(&format!("load plugin profile: {err}")))
}

/// PUT /v1/plugins/profile response.
#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ProfileUpdateResponse {
    pub profile: dope_plugin::PluginProfile,
    /// Always true: the profile is boot-time input.
    pub restart_required: bool,
}

/// PUT /v1/plugins/profile — validate and atomically replace plugins.json.
#[allow(clippy::unused_async)]
pub async fn put_profile(
    State(state): State<AppState>,
    body: Bytes,
) -> Result<Json<ProfileUpdateResponse>, ApiError> {
    let profile: dope_plugin::PluginProfile = super::decode_json_required(&body)?;
    let dir = Path::new(&state.config.data_dir);
    let path = dir.join(dope_plugin::PROFILE_FILE_NAME);
    let encoded = serde_json::to_vec_pretty(&profile)
        .map_err(|err| ApiError::internal(&format!("encode plugin profile: {err}")))?;
    // Atomic replace: a crash mid-write must never leave a truncated
    // profile that would then fail the next boot.
    let tmp = dir.join(format!("{}.tmp", dope_plugin::PROFILE_FILE_NAME));
    std::fs::create_dir_all(dir)
        .and_then(|()| std::fs::write(&tmp, &encoded))
        .and_then(|()| std::fs::rename(&tmp, &path))
        .map_err(|err| ApiError::internal(&format!("write plugin profile: {err}")))?;
    Ok(Json(ProfileUpdateResponse { profile, restart_required: true }))
}

/// The `/v1/plugins` route family.
pub fn router() -> Router<AppState> {
    Router::new()
        .route("/v1/plugins", get(list_plugins))
        .route("/v1/plugins/profile", get(get_profile).put(put_profile))
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

    /// A test state whose config.data_dir is unique (the shared tests_support
    /// config points every test at one path; profile writes need isolation).
    fn profile_test_state() -> crate::state::AppState {
        let mut state = test_state();
        let dir = std::env::temp_dir()
            .join(format!("dope-plugins-profile-{}", uuid::Uuid::now_v7()));
        std::fs::create_dir_all(&dir).expect("mkdir");
        state.config.data_dir = dir.to_string_lossy().into_owned();
        state
    }

    #[tokio::test]
    async fn profile_get_defaults_put_roundtrips_and_rejects_malformed() {
        let state = profile_test_state();

        // Missing file: the default (empty) profile.
        let (status, json) =
            request_json(state.clone(), "GET", "/v1/plugins/profile", None).await;
        assert_eq!(status, StatusCode::OK);
        assert_eq!(json["disabled"], serde_json::json!([]));

        // PUT persists atomically and flags the restart requirement.
        let update = serde_json::json!({
            "disabled": ["channel-discord"],
            "entries": { "session-strategy": { "config": { "personalBudgetChars": 1000 } } }
        });
        let (status, json) =
            request_json(state.clone(), "PUT", "/v1/plugins/profile", Some(update.clone())).await;
        assert_eq!(status, StatusCode::OK);
        assert_eq!(json["restartRequired"], true);
        assert_eq!(json["profile"]["disabled"][0], "channel-discord");

        // The write landed on disk and reads back.
        let (status, json) = request_json(state.clone(), "GET", "/v1/plugins/profile", None).await;
        assert_eq!(status, StatusCode::OK);
        assert_eq!(json["disabled"][0], "channel-discord");
        assert_eq!(
            json["entries"]["session-strategy"]["config"]["personalBudgetChars"],
            1000
        );

        // Malformed body: 400, disk untouched.
        let (status, _) = request_json(
            state.clone(),
            "PUT",
            "/v1/plugins/profile",
            Some(serde_json::json!({ "disabled": "not-an-array" })),
        )
        .await;
        assert_eq!(status, StatusCode::BAD_REQUEST);
        let (_, json) = request_json(state, "GET", "/v1/plugins/profile", None).await;
        assert_eq!(json["disabled"][0], "channel-discord", "malformed PUT changed nothing");
    }
}
